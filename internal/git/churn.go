package git

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// After is the first Commits commits of HEAD's first-parent line after Rev.
type After struct {
	Rev     string
	Commits int
}

// Churn returns the commits since rev, up to HEAD, that changed each file of
// dir's work tree, with paths relative to dir; an empty rev means every
// commit HEAD reaches. Rename detection is off, so a renamed file's history
// starts at the rename and the result does not depend on git's similarity
// guesses. Since and Head are the full hashes the range resolved to, Since ""
// for the whole history.
func Churn(ctx context.Context, dir, rev string) (arch.Churn, error) {
	const op = "git.Churn"
	if err := gitOnPath(); err != nil {
		return arch.Churn{}, &arch.Error{Op: op, Err: err}
	}
	head, err := resolve(ctx, dir, "HEAD")
	if err != nil {
		return arch.Churn{}, &arch.Error{Op: op, Err: err}
	}
	since, span := "", head
	if rev != "" {
		if since, err = resolve(ctx, dir, rev); err != nil {
			return arch.Churn{}, &arch.Error{Op: op, Err: err}
		}
		span = since + ".." + head
	}
	log, err := run(ctx, dir, "log", "--format=%x00%H%x00%ae", "--name-only", "--no-renames",
		"--relative", "--end-of-options", span, "--")
	if err != nil {
		return arch.Churn{}, &arch.Error{Op: op, Err: err}
	}
	commits, authors, err := ParseLog(bytes.NewReader(log))
	if err != nil {
		return arch.Churn{}, &arch.Error{Op: op, Err: err}
	}
	return arch.Churn{Since: since, Head: head, Commits: commits, Authors: authors}, nil
}

// Window returns the revision that bounds the last n commits of HEAD's
// first-parent line, HEAD~n, as Churn's rev: the window moves with HEAD, so
// two runs at one commit agree, and a commit never worsens the measures only
// by being one more. A history of n commits or fewer gives "", the whole of
// it. A shallow clone hides how long the history is, so there an unreachable
// HEAD~n is ENOTFOUND rather than a silently shorter window.
//
// Requires: n > 0.
func Window(ctx context.Context, dir string, n int) (string, error) {
	const op = "git.Window"
	if err := gitOnPath(); err != nil {
		return "", &arch.Error{Op: op, Err: err}
	}
	rev, err := resolve(ctx, dir, "HEAD~"+strconv.Itoa(n))
	if err == nil {
		return rev, nil
	}
	out, shallowErr := run(ctx, dir, "rev-parse", "--is-shallow-repository")
	switch {
	case shallowErr != nil:
		return "", &arch.Error{Op: op, Err: shallowErr}
	case strings.TrimSpace(string(out)) == "true":
		return "", &arch.Error{Op: op, Err: &arch.Error{
			Code: arch.ENOTFOUND,
			Message: fmt.Sprintf("the clone is shallow and does not reach %d commits back; "+
				"fetch the history (git fetch --unshallow, or fetch-depth: 0 in CI)", n),
		}}
	}
	return "", nil
}

// resolve returns the full hash of the commit rev names.
func resolve(ctx context.Context, dir, rev string) (string, error) {
	out, err := run(ctx, dir, "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ParseLog parses git log --format=%x00%H%x00%ae --name-only: each commit is
// a NUL, its hash, a NUL and its author's email, and the names of the files
// it changed. It returns each file's commits, sorted, and each commit's
// author.
func ParseLog(r io.Reader) (map[string][]string, map[string]string, error) {
	commits, authors := map[string][]string{}, map[string]string{}
	hash := ""
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "\x00"):
			var author string
			hash, author, _ = strings.Cut(strings.TrimPrefix(line, "\x00"), "\x00")
			authors[hash] = author
		case line != "" && hash != "":
			commits[line] = append(commits[line], hash)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, nil, &arch.Error{Code: arch.EINVALID, Message: "read log: " + err.Error()}
	}
	for file, hs := range commits {
		slices.Sort(hs)
		commits[file] = slices.Compact(hs)
	}
	return commits, authors, nil
}

// Fixes returns what a validation reads of the commits a names: how many
// there were, and the files each one whose subject reads as a fix changed,
// with paths relative to dir. Rename detection is off, as for Churn. Since
// is the revision's full hash, and Until the last commit read, the
// revision's own when none follows it.
//
// Requires: a.Commits > 0.
func Fixes(ctx context.Context, dir string, a After) (arch.FixHistory, error) {
	const op = "git.Fixes"
	if err := gitOnPath(); err != nil {
		return arch.FixHistory{}, &arch.Error{Op: op, Err: err}
	}
	since, err := resolve(ctx, dir, a.Rev)
	if err != nil {
		return arch.FixHistory{}, &arch.Error{Op: op, Err: err}
	}
	log, err := run(ctx, dir, "log", "--first-parent", "--reverse", "--format=%x00%H%x00%s",
		"--name-only", "--no-renames", "--relative", "--end-of-options", since+"..HEAD", "--")
	if err != nil {
		return arch.FixHistory{}, &arch.Error{Op: op, Err: err}
	}
	h, err := ParseFixes(bytes.NewReader(log), a.Commits)
	if err != nil {
		return arch.FixHistory{}, &arch.Error{Op: op, Err: err}
	}
	h.Since = since
	h.Until = cmp.Or(h.Until, since)
	return h, nil
}

// ParseFixes parses git log --reverse --format=%x00%H%x00%s --name-only, the
// oldest commit first: each commit is a NUL, its hash, a NUL and its
// subject, and the names of the files it changed. It reads the first n
// commits, and returns how many it read, the last of them, and the files of
// each fix among them, each file's fixes sorted.
func ParseFixes(r io.Reader, n int) (arch.FixHistory, error) {
	h := arch.FixHistory{Fixes: map[string][]string{}}
	fix := false
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "\x00"):
			if h.Commits == n {
				return sortedFixes(h), nil
			}
			hash, subject, _ := strings.Cut(strings.TrimPrefix(line, "\x00"), "\x00")
			h.Commits++
			h.Until, fix = hash, arch.IsFix(subject)
			if fix {
				h.FixCommits++
			}
		case line != "" && fix:
			h.Fixes[line] = append(h.Fixes[line], h.Until)
		}
	}
	if err := sc.Err(); err != nil {
		return arch.FixHistory{}, &arch.Error{
			Code:    arch.EINVALID,
			Message: "read log: " + err.Error(),
		}
	}
	return sortedFixes(h), nil
}

// sortedFixes returns h with each file's fixes sorted, without duplicates.
func sortedFixes(h arch.FixHistory) arch.FixHistory {
	for file, hs := range h.Fixes {
		slices.Sort(hs)
		h.Fixes[file] = slices.Compact(hs)
	}
	return h
}
