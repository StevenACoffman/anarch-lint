package git_test

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/git"
)

func TestParseLog(t *testing.T) {
	t.Parallel()
	log := "\x00bbb\x00b@x\n\na.go\nb.go\n\x00aaa\x00a@x\n\na.go\n\x00ccc\x00a@x\n\n"
	commits, authors, err := git.ParseLog(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	equals(t, map[string][]string{"a.go": {"aaa", "bbb"}, "b.go": {"bbb"}}, commits)
	equals(t, map[string]string{"aaa": "a@x", "bbb": "b@x", "ccc": "a@x"}, authors)
}

func TestChurnInRepository(t *testing.T) {
	t.Parallel()
	if testing.Short() || !gitAvailable() {
		t.Skip("needs git; skipped with -short")
	}
	dir := t.TempDir()
	gitIn := func(args ...string) string {
		t.Helper()
		//nolint:gosec // G204: the program is the constant "git" and every argument is a literal of this test
		cmd := exec.CommandContext(t.Context(), "git", append([]string{
			"-C", dir, "-c", "user.email=t@example.com",
			"-c", "user.name=t", "-c", "commit.gpgsign=false",
		}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(name, content string) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		gitIn("add", name)
		gitIn("commit", "-q", "-m", name)
		return gitIn("rev-parse", "HEAD")
	}
	gitIn("init", "-q")
	base := commit("a.go", "package a\n")
	second := commit("sub/b.go", "package b\n")
	third := commit("a.go", "package a // changed\n")

	churn, err := git.Churn(context.Background(), dir, base)
	if err != nil {
		t.Fatalf("churn: %v", err)
	}
	equals(t, base, churn.Since)
	equals(t, third, churn.Head)
	equals(t, map[string][]string{"a.go": {third}, "sub/b.go": {second}}, churn.Commits)
	equals(t, map[string]string{second: "t@example.com", third: "t@example.com"}, churn.Authors)

	_, err = git.Churn(context.Background(), dir, "no-such-rev")
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}

// A window is the last n commits of HEAD's first-parent line: a longer
// history is cut at HEAD~n, a shorter one is read whole, and a shallow clone
// that cannot reach HEAD~n says so rather than reading fewer commits.
func TestWindowInRepository(t *testing.T) {
	t.Parallel()
	dir, gitIn, write := repository(t)
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		write(name, "package a\n")
		gitIn("add", name)
		gitIn("commit", "-q", "-m", name)
	}
	ctx := context.Background()

	rev, err := git.Window(ctx, dir, 2)
	if err != nil {
		t.Fatalf("window: %v", err)
	}
	churn, err := git.Churn(ctx, dir, rev)
	if err != nil {
		t.Fatalf("churn: %v", err)
	}
	equals(t, []string{"b.go", "c.go"}, slices.Sorted(maps.Keys(churn.Commits)))

	rev, err = git.Window(ctx, dir, 3)
	if err != nil {
		t.Fatalf("window: %v", err)
	}
	equals(t, "", rev)
	churn, err = git.Churn(ctx, dir, rev)
	if err != nil {
		t.Fatalf("churn: %v", err)
	}
	equals(t, "", churn.Since)
	equals(t, []string{"a.go", "b.go", "c.go"}, slices.Sorted(maps.Keys(churn.Commits)))

	clone := filepath.Join(t.TempDir(), "clone")
	gitIn("clone", "-q", "--depth", "1", "file://"+dir, clone)
	_, err = git.Window(ctx, clone, 2)
	equals(t, arch.ENOTFOUND, arch.ErrorCode(err))
}
