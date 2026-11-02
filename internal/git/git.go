// Package git reads the lines a change touched and the commits that changed
// each file, by running the git command line tool. It is the only package
// that runs git.
//
// Diffs are parsed by github.com/bluekeyes/go-gitdiff (MIT), not revgrep,
// which answers only point queries and has known parsing faults, nor
// golangci-lint's diff processor, which is GPL-licensed (mmi-ref-plan.md §3).
package git

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// FileAt names a file at a revision: Path is relative to the directory git
// runs in.
type FileAt struct {
	Rev  string
	Path string
}

// ChangedLines returns the lines added or modified since rev in the work tree
// of dir, relative to dir, plus every line of each untracked file, which a
// diff does not show. Deleted lines touch nothing: no code remains there.
func ChangedLines(ctx context.Context, dir, rev string) (arch.ChangedLines, error) {
	const op = "git.ChangedLines"
	if err := gitOnPath(); err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	// --end-of-options keeps a revision such as "--output=x" from being read
	// as an option: rev comes from the command line.
	diff, err := run(ctx, dir, "diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames",
		"--relative", "--src-prefix=a/", "--dst-prefix=b/", "--end-of-options", rev, "--")
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	changed, err := ParseDiff(bytes.NewReader(diff))
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	// -z ends each name with NUL and leaves it unquoted, so a name with a
	// space or a non-ASCII letter is one path, as written.
	untracked, err := run(ctx, dir, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	for file := range strings.SplitSeq(string(untracked), "\x00") {
		if file != "" {
			changed[file] = []arch.LineRange{{Start: 1, End: math.MaxInt}}
		}
	}
	return changed, nil
}

// Show returns the contents of a file at a revision, in the repository that
// holds dir. An unknown revision, such as one a shallow clone lacks, is
// EINVALID; a revision without the file is ENOTFOUND.
func Show(ctx context.Context, dir string, file FileAt) ([]byte, error) {
	const op = "git.Show"
	rev, path := file.Rev, file.Path
	if err := gitOnPath(); err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	if _, err := run(ctx, dir, "rev-parse", "--verify", "--quiet", "--end-of-options",
		rev+"^{commit}"); err != nil {
		return nil, &arch.Error{Op: op, Err: &arch.Error{
			Code: arch.EINVALID, Message: "unknown revision " + rev,
		}}
	}
	// rev is a verified commit, so the object name cannot be read as an
	// option; "./" makes the path relative to dir rather than to the root.
	object := rev + ":./" + filepath.ToSlash(path)
	if _, err := run(ctx, dir, "cat-file", "-e", object); err != nil {
		return nil, &arch.Error{Op: op, Err: &arch.Error{
			Code: arch.ENOTFOUND, Message: path + " does not exist at " + rev,
		}}
	}
	out, err := run(ctx, dir, "cat-file", "blob", object)
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	return out, nil
}

// Worktree checks rev out, detached, into a new temporary directory, and
// returns the directory that corresponds there to dir, with a function that
// removes the checkout. The work tree and index of dir are untouched.
func Worktree(ctx context.Context, dir, rev string) (string, func() error, error) {
	const op = "git.Worktree"
	if err := gitOnPath(); err != nil {
		return "", nil, &arch.Error{Op: op, Err: err}
	}
	prefix, err := run(ctx, dir, "rev-parse", "--show-prefix")
	if err != nil {
		return "", nil, &arch.Error{Op: op, Err: err}
	}
	if _, err := run(ctx, dir, "rev-parse", "--verify", "--quiet", "--end-of-options",
		rev+"^{commit}"); err != nil {
		return "", nil, &arch.Error{Op: op, Err: &arch.Error{
			Code: arch.EINVALID, Message: "unknown revision " + rev,
		}}
	}
	tmp, err := os.MkdirTemp("", "anarch-lint-base-")
	if err != nil {
		return "", nil, &arch.Error{
			Op:  op,
			Err: &arch.Error{Code: arch.EINTERNAL, Message: err.Error()},
		}
	}
	root := filepath.Join(tmp, "base")
	if _, err := run(ctx, dir, "worktree", "add", "--detach", "--quiet", "--end-of-options",
		root, rev); err != nil {
		_ = os.RemoveAll(tmp)
		return "", nil, &arch.Error{Op: op, Err: err}
	}
	remove := func() error {
		_, err := run(context.WithoutCancel(ctx), dir, "worktree", "remove", "--force", root)
		if rmErr := os.RemoveAll(tmp); err == nil && rmErr != nil {
			err = &arch.Error{Code: arch.EINTERNAL, Message: rmErr.Error()}
		}
		return err
	}
	return filepath.Join(root, strings.TrimSpace(string(prefix))), remove, nil
}

// ParseDiff parses a unified diff, as git diff writes it, into the ranges of
// added or modified lines of each new file, using
// github.com/bluekeyes/go-gitdiff. A deleted file contributes nothing, and
// "\ No newline at end of file" is not a line. A malformed diff is EINVALID.
func ParseDiff(r io.Reader) (arch.ChangedLines, error) {
	files, _, err := gitdiff.Parse(r)
	if err != nil {
		return nil, &arch.Error{Code: arch.EINVALID, Message: "parse diff: " + err.Error()}
	}
	changed := arch.ChangedLines{}
	for _, f := range files {
		if f.IsDelete {
			continue
		}
		var ranges []arch.LineRange
		for _, frag := range f.TextFragments {
			ranges = append(ranges, addedLines(frag)...)
		}
		if len(ranges) > 0 {
			changed[f.NewName] = merge(append(changed[f.NewName], ranges...))
		}
	}
	return changed, nil
}

// addedLines returns the new-file line of each line a fragment adds, one
// range each; merge joins them.
func addedLines(frag *gitdiff.TextFragment) []arch.LineRange {
	var out []arch.LineRange
	line := int(frag.NewPosition)
	for _, l := range frag.Lines {
		switch l.Op {
		case gitdiff.OpAdd:
			out = append(out, arch.LineRange{Start: line, End: line})
			line++
		case gitdiff.OpContext:
			line++
		case gitdiff.OpDelete:
		}
	}
	return out
}

// merge sorts ranges and joins those that overlap or touch.
func merge(ranges []arch.LineRange) []arch.LineRange {
	slices.SortFunc(ranges, func(a, b arch.LineRange) int {
		return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End))
	})
	out := ranges[:0]
	for _, r := range ranges {
		if n := len(out); n > 0 && r.Start <= out[n-1].End+1 {
			out[n-1].End = max(out[n-1].End, r.End)
			continue
		}
		out = append(out, r)
	}
	return out
}

// gitOnPath reports ENOTFOUND when no git binary is on PATH.
func gitOnPath() error {
	if _, err := exec.LookPath("git"); err != nil {
		return &arch.Error{Code: arch.ENOTFOUND, Message: "git is not on PATH"}
	}
	return nil
}

// run runs git with args in dir and returns its standard output. A failure is
// EINVALID with git's own message, which names the bad revision or directory.
func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	//nolint:gosec // G204: the program is the constant "git"; dir and rev are the user's own inputs, and --end-of-options stops rev from becoming an option
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		return nil, &arch.Error{Code: arch.EINVALID, Message: "git " + args[0] + ": " +
			strings.TrimSpace(stderr.String())}
	case err != nil:
		return nil, &arch.Error{
			Code:    arch.EINTERNAL,
			Message: "git " + args[0] + ": " + err.Error(),
		}
	}
	return out, nil
}
