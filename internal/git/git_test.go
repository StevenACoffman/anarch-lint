package git_test

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/git"
)

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

func TestParseDiff(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		diff    string
		changed arch.ChangedLines
	}{
		"added and modified lines": {
			diff: "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n" +
				"@@ -3 +3 @@\n-old\n+new\n@@ -10,0 +11,2 @@\n+a\n+b\n",
			changed: arch.ChangedLines{"x.go": {{Start: 3, End: 3}, {Start: 11, End: 12}}},
		},
		"pure deletion touches nothing": {
			diff:    "--- a/x.go\n+++ b/x.go\n@@ -4,2 +3,0 @@\n-gone\n-gone\n",
			changed: arch.ChangedLines{},
		},
		"deleted file contributes nothing": {
			diff:    "--- a/x.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n",
			changed: arch.ChangedLines{},
		},
		"adjacent hunks merge": {
			diff:    "--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-a\n+b\n@@ -2 +2,2 @@\n-c\n+d\n+e\n",
			changed: arch.ChangedLines{"x.go": {{Start: 1, End: 3}}},
		},
		"no newline marker is not a line": {
			diff:    "--- a/x.go\n+++ b/x.go\n@@ -5 +5 @@\n-a\n\\ No newline at end of file\n+b\n",
			changed: arch.ChangedLines{"x.go": {{Start: 5, End: 5}}},
		},
		"two files": {
			diff: "--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-x\n+y\n" +
				"--- a/b.go\n+++ b/b.go\n@@ -7 +7 @@\n-x\n+y\n",
			changed: arch.ChangedLines{"a.go": {{Start: 1, End: 1}}, "b.go": {{Start: 7, End: 7}}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			changed, err := git.ParseDiff(strings.NewReader(tc.diff))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			equals(t, tc.changed, changed)
		})
	}
}

func TestParseDiffMalformed(t *testing.T) {
	t.Parallel()
	_, err := git.ParseDiff(strings.NewReader("+++ b/x.go\n@@ -1 +zz @@\n"))
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}

// FuzzParseDiff checks that no input crashes the parser, and that what it
// returns is sorted and non-overlapping, as arch.ChangedLines requires.
func FuzzParseDiff(f *testing.F) {
	f.Add("--- a/x.go\n+++ b/x.go\n@@ -3 +3 @@\n-old\n+new\n")
	f.Add("+++ b/x.go\n@@ -1,0 +1,5 @@\n")
	f.Fuzz(func(t *testing.T, diff string) {
		changed, err := git.ParseDiff(strings.NewReader(diff))
		if err != nil {
			return
		}
		for file, ranges := range changed {
			for i := 1; i < len(ranges); i++ {
				if ranges[i].Start <= ranges[i-1].End {
					t.Fatalf("%s: ranges %v overlap or are unsorted", file, ranges)
				}
			}
		}
	})
}

// gitAvailable reports whether a git binary is on PATH; tests that run git
// skip without one.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func TestChangedLinesInRepository(t *testing.T) {
	t.Parallel()
	if testing.Short() || !gitAvailable() {
		t.Skip("needs git; skipped with -short")
	}
	dir := t.TempDir()
	gitIn := func(args ...string) {
		t.Helper()
		//nolint:gosec // G204: the program is the constant "git" and every argument is a literal of this test
		cmd := exec.CommandContext(t.Context(), "git", append([]string{
			"-C", dir, "-c", "user.email=t@example.com",
			"-c", "user.name=t", "-c", "commit.gpgsign=false",
		}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitIn("init", "-q")
	write("a.go", "package a\n\nfunc A() {}\n")
	gitIn("add", "a.go")
	gitIn("commit", "-q", "-m", "init")
	write("a.go", "package a\n\nfunc A() { _ = 1 }\n")
	write("new.go", "package a\n")

	changed, err := git.ChangedLines(context.Background(), dir, "HEAD")
	if err != nil {
		t.Fatalf("changed lines: %v", err)
	}
	equals(t, arch.ChangedLines{
		"a.go":   {{Start: 3, End: 3}},
		"new.go": {{Start: 1, End: math.MaxInt}},
	}, changed)

	_, err = git.ChangedLines(context.Background(), dir, "no-such-rev")
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}
