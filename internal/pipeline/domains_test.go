package pipeline_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
)

const domainModel = "kind: DomainModel\nversion: v1\nentities:\n  Order:\n    definition: A purchase.\n"

// repository writes a one-package module with a .git directory and a model
// whose fitness.domains rule names path, and returns the module's directory.
func repository(t *testing.T, path string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	write := func(name, content string) {
		t.Helper()
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/r\n\ngo 1.23\n")
	write("r.go", "package r\n\n// Order is a purchase.\ntype Order struct{}\n")
	write(".git/HEAD", "ref: refs/heads/main\n")
	write("docs/r.modelith.yaml", domainModel)
	write("anarch.json", `{"version": 1, "fitness": {"domains": [{"path": "`+path+`"}]}}`)
	return dir
}

func load(t *testing.T, dir string) (arch.Input, error) {
	t.Helper()
	if testing.Short() {
		t.Skip("loads a module; skipped with -short")
	}
	var out bytes.Buffer
	cfg := root.New(nil, &out, &out)
	cfg.Dir = dir
	return cfg.Load(context.Background(), nil, pipeline.NoFacts)
}

func TestLoadReadsDomainModels(t *testing.T) {
	t.Parallel()
	in, err := load(t, repository(t, "docs/r.modelith.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Domains) != 1 || in.Domains[0].Path != "docs/r.modelith.yaml" ||
		in.Domains[0].Entities[0].Name != "Order" {
		t.Fatalf("domains: %+v", in.Domains)
	}
}

func TestLoadConfinesDomainModels(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		path string
		code string
		prep func(t *testing.T, dir string)
	}{
		"a path out of the repository": {
			path: "../outside.modelith.yaml", code: arch.EINVALID,
			prep: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, "..", "outside.modelith.yaml"),
					[]byte(domainModel), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		"a symlink pointing out": {
			path: "docs/link.modelith.yaml", code: arch.EINVALID,
			prep: func(t *testing.T, dir string) {
				t.Helper()
				outside := filepath.Join(dir, "..", "secret.modelith.yaml")
				if err := os.WriteFile(outside, []byte(domainModel), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(
					outside,
					filepath.Join(dir, "docs", "link.modelith.yaml"),
				); err != nil {
					t.Skip("symlinks unavailable:", err)
				}
			},
		},
		"a missing file": {
			path: "docs/none.modelith.yaml",
			code: arch.ENOTFOUND,
			prep: func(*testing.T, string) {},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := repository(t, tc.path)
			tc.prep(t, dir)
			_, err := load(t, dir)
			if got := arch.ErrorCode(err); got != tc.code {
				t.Fatalf("code %q, want %q: %v", got, tc.code, err)
			}
		})
	}
}
