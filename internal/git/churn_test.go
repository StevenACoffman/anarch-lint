package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/git"
)

func TestParseLog(t *testing.T) {
	t.Parallel()
	log := "\x00bbb\n\na.go\nb.go\n\x00aaa\n\na.go\n\x00ccc\n\n"
	commits, err := git.ParseLog(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	equals(t, map[string][]string{"a.go": {"aaa", "bbb"}, "b.go": {"bbb"}}, commits)
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

	_, err = git.Churn(context.Background(), dir, "no-such-rev")
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}
