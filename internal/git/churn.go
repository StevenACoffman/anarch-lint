package git

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Churn returns the commits since rev, up to HEAD, that changed each file of
// dir's work tree, with paths relative to dir. Rename detection is off, so a
// renamed file's history starts at the rename and the result does not depend
// on git's similarity guesses. Since and Head are the full hashes the range
// resolved to.
func Churn(ctx context.Context, dir, rev string) (arch.Churn, error) {
	const op = "git.Churn"
	if err := gitOnPath(); err != nil {
		return arch.Churn{}, &arch.Error{Op: op, Err: err}
	}
	since, err := resolve(ctx, dir, rev)
	if err != nil {
		return arch.Churn{}, &arch.Error{Op: op, Err: err}
	}
	head, err := resolve(ctx, dir, "HEAD")
	if err != nil {
		return arch.Churn{}, &arch.Error{Op: op, Err: err}
	}
	log, err := run(ctx, dir, "log", "--format=%x00%H%x00%ae", "--name-only", "--no-renames",
		"--relative", "--end-of-options", since+".."+head, "--")
	if err != nil {
		return arch.Churn{}, &arch.Error{Op: op, Err: err}
	}
	commits, authors, err := ParseLog(bytes.NewReader(log))
	if err != nil {
		return arch.Churn{}, &arch.Error{Op: op, Err: err}
	}
	return arch.Churn{Since: since, Head: head, Commits: commits, Authors: authors}, nil
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
