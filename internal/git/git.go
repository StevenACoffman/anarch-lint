// Package git reads the lines a change touched and the commits that changed
// each file, by running the git command line tool. It is the only package
// that runs git.
//
// The parser it uses is written here rather than borrowed: revgrep answers
// only point queries and has known parsing faults, and golangci-lint's diff
// processor is GPL-licensed (mmi-ref-plan.md §3).
package git

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

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
	untracked, err := run(ctx, dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	for _, file := range strings.Fields(string(untracked)) {
		changed[file] = []arch.LineRange{{Start: 1, End: math.MaxInt}}
	}
	return changed, nil
}

// ParseDiff parses a unified diff with zero lines of context, as git diff -U0
// writes it, into the ranges of added or modified lines of each new file. A
// deleted file contributes nothing, and "\ No newline at end of file" is not
// a line. A malformed hunk header is EINVALID.
func ParseDiff(r io.Reader) (arch.ChangedLines, error) {
	changed := arch.ChangedLines{}
	file := ""
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "+++ "):
			file = newFileName(line)
		case strings.HasPrefix(line, "@@ ") && file != "":
			rng, err := parseHunk(line)
			if err != nil {
				return nil, err
			}
			if rng.End >= rng.Start {
				changed[file] = append(changed[file], rng)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, &arch.Error{Code: arch.EINVALID, Message: "read diff: " + err.Error()}
	}
	for f, ranges := range changed {
		changed[f] = merge(ranges)
	}
	return changed, nil
}

// newFileName returns the path of a "+++ b/path" line, or "" for a deleted
// file ("+++ /dev/null").
func newFileName(line string) string {
	name := strings.TrimPrefix(line, "+++ ")
	if name == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(name, "b/")
}

// parseHunk returns the new-file line range of a hunk header
// "@@ -a[,b] +c[,d] @@". A count of 0 marks a pure deletion, whose range is
// empty.
func parseHunk(line string) (arch.LineRange, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return arch.LineRange{}, malformed(line)
	}
	startText, countText, hasCount := strings.Cut(strings.TrimPrefix(fields[2], "+"), ",")
	start, err := strconv.Atoi(startText)
	if err != nil {
		return arch.LineRange{}, malformed(line)
	}
	count := 1
	if hasCount {
		if count, err = strconv.Atoi(countText); err != nil {
			return arch.LineRange{}, malformed(line)
		}
	}
	return arch.LineRange{Start: start, End: start + count - 1}, nil
}

func malformed(line string) error {
	return &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("malformed hunk header %q", line)}
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
