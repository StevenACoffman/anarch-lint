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
			diff:    "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -4,2 +3,0 @@\n-gone\n-gone\n",
			changed: arch.ChangedLines{},
		},
		"deleted file contributes nothing": {
			diff:    "diff --git a/x.go b/x.go\ndeleted file mode 100644\n--- a/x.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n",
			changed: arch.ChangedLines{},
		},
		"adjacent hunks merge": {
			diff:    "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-a\n+b\n@@ -2 +2,2 @@\n-c\n+d\n+e\n",
			changed: arch.ChangedLines{"x.go": {{Start: 1, End: 3}}},
		},
		"no newline marker is not a line": {
			diff:    "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -5 +5 @@\n-a\n\\ No newline at end of file\n+b\n",
			changed: arch.ChangedLines{"x.go": {{Start: 5, End: 5}}},
		},
		"new file": {
			diff: "diff --git a/n.go b/n.go\nnew file mode 100644\nindex 0000000..1111111\n" +
				"--- /dev/null\n+++ b/n.go\n@@ -0,0 +1,2 @@\n+a\n+b\n",
			changed: arch.ChangedLines{"n.go": {{Start: 1, End: 2}}},
		},
		"quoted path": {
			diff: "diff --git \"a/sp\\303\\251c.go\" \"b/sp\\303\\251c.go\"\n" +
				"--- \"a/sp\\303\\251c.go\"\n+++ \"b/sp\\303\\251c.go\"\n@@ -2 +2 @@\n-x\n+y\n",
			changed: arch.ChangedLines{"spéc.go": {{Start: 2, End: 2}}},
		},
		"two files": {
			diff: "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-x\n+y\n" +
				"diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -7 +7 @@\n-x\n+y\n",
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
	_, err := git.ParseDiff(strings.NewReader(
		"diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1 +zz @@\n"))
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}

// FuzzParseDiff checks that no input crashes the parser, and that what it
// returns is sorted and non-overlapping, as arch.ChangedLines requires.
func FuzzParseDiff(f *testing.F) {
	f.Add("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -3 +3 @@\n-old\n+new\n")
	f.Add("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1,0 +1,5 @@\n")
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

// repository makes an empty repository in a temporary directory, and
// returns it with a function that runs git there and one that writes a file.
func repository(
	t *testing.T,
) (dir string, gitIn func(...string), write func(name, content string)) {
	t.Helper()
	if testing.Short() || !gitAvailable() {
		t.Skip("needs git; skipped with -short")
	}
	dir = t.TempDir()
	gitIn = func(args ...string) {
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
	write = func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitIn("init", "-q")
	return dir, gitIn, write
}

func TestChangedLinesInRepository(t *testing.T) {
	t.Parallel()
	dir, gitIn, write := repository(t)
	write("a.go", "package a\n\nfunc A() {}\n")
	gitIn("add", "a.go")
	gitIn("commit", "-q", "-m", "init")
	write("a.go", "package a\n\nfunc A() { _ = 1 }\n")
	write("new.go", "package a\n")
	write("new file.go", "package a\n") // one path, not "new" and "file.go"
	write("ünïcode.go", "package a\n")  // as written, not quoted

	changed, err := git.ChangedLines(context.Background(), dir, "HEAD")
	if err != nil {
		t.Fatalf("changed lines: %v", err)
	}
	equals(t, arch.ChangedLines{
		"a.go":        {{Start: 3, End: 3}},
		"new.go":      {{Start: 1, End: math.MaxInt}},
		"new file.go": {{Start: 1, End: math.MaxInt}},
		"ünïcode.go":  {{Start: 1, End: math.MaxInt}},
	}, changed)

	_, err = git.ChangedLines(context.Background(), dir, "no-such-rev")
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}

// TestShow: a file at a revision, relative to a directory below the root;
// a file the revision lacks; and an unknown revision.
func TestShow(t *testing.T) {
	t.Parallel()
	dir, gitIn, write := repository(t)
	write("sub/anarch.json", `{"version": 1}`)
	gitIn("add", ".")
	gitIn("commit", "-q", "-m", "init")
	write("sub/anarch.json", `{"version": 1, "layers": []}`)
	sub := filepath.Join(dir, "sub")

	src, err := git.Show(context.Background(), sub, git.FileAt{Rev: "HEAD", Path: "anarch.json"})
	if err != nil {
		t.Fatal(err)
	}
	equals(t, `{"version": 1}`, string(src))
	_, err = git.Show(context.Background(), sub, git.FileAt{Rev: "HEAD", Path: "missing.json"})
	equals(t, arch.ENOTFOUND, arch.ErrorCode(err))
	_, err = git.Show(
		context.Background(),
		sub,
		git.FileAt{Rev: "no-such-rev", Path: "anarch.json"},
	)
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}

// TestWorktree: a revision checked out apart, at the directory that
// corresponds to dir, and removed again; the work tree is untouched.
func TestWorktree(t *testing.T) {
	t.Parallel()
	dir, gitIn, write := repository(t)
	write("sub/a.go", "package a // v1\n")
	gitIn("add", ".")
	gitIn("commit", "-q", "-m", "init")
	write("sub/a.go", "package a // v2\n")

	base, remove, err := git.Worktree(context.Background(), filepath.Join(dir, "sub"), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	equals(t, "sub", filepath.Base(base))
	src, err := os.ReadFile(filepath.Join(base, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	equals(t, "package a // v1\n", string(src))
	if err := remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Fatalf("the checkout remains: %v", err)
	}
	src, _ = os.ReadFile(filepath.Join(dir, "sub", "a.go"))
	equals(t, "package a // v2\n", string(src))

	_, _, err = git.Worktree(context.Background(), dir, "no-such-rev")
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}
