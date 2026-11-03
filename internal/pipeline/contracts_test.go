package pipeline_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// graphRepository writes a one-package module with a .git directory, a
// GraphQL schema in graph/ and an operation in ops/, and a model whose
// fitness.graphql rule names the globs given, and returns its directory.
func graphRepository(t *testing.T, schema, operations string) string {
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
	write("r.go", "package r\n")
	write(".git/HEAD", "ref: refs/heads/main\n")
	write("graph/schema.graphqls", "type Query {\n  tasks: [String!]!\n}\n")
	write("graph/more.graphqls", "extend type Query {\n  count: Int!\n}\n")
	write("ops/list.graphql", "query { tasks }\n")
	write("anarch.json", `{"version": 1, "fitness": {"graphql": [{"name": "g",
		"schema": ["`+schema+`"], "operations": ["`+operations+`"]}]}}`)
	return dir
}

func TestLoadReadsGraphQLSchemas(t *testing.T) {
	t.Parallel()
	in, err := load(t, graphRepository(t, "graph/*.graphqls", "ops/**/*.graphql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(in.GraphQL) != 1 || len(in.GraphQL[0].Fields) != 2 ||
		in.GraphQL[0].Fields[0].Pos.File != "graph/more.graphqls" ||
		in.GraphQL[0].Operations != 1 || len(in.GraphQL[0].Problems) != 0 {
		t.Fatalf("graphql: %+v", in.GraphQL)
	}
}

func TestLoadRefusesGraphQLGlobs(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		schema string
		prep   func(t *testing.T, dir string)
	}{
		"a glob that matches nothing": {
			schema: "schema/*.graphqls",
			prep:   func(*testing.T, string) {},
		},
		"a glob out of the repository": {
			schema: "../*.graphqls",
			prep:   func(*testing.T, string) {},
		},
		"a symlink pointing out": {
			schema: "graph/*.graphqls",
			prep: func(t *testing.T, dir string) {
				t.Helper()
				outside := filepath.Join(dir, "..", "secret.graphqls")
				if err := os.WriteFile(outside, []byte("type Secret { s: String }\n"),
					0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(
					outside,
					filepath.Join(dir, "graph", "link.graphqls"),
				); err != nil {
					t.Skip("symlinks unavailable:", err)
				}
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := graphRepository(t, tc.schema, "ops/*.graphql")
			tc.prep(t, dir)
			_, err := load(t, dir)
			if got := arch.ErrorCode(err); got != arch.EINVALID {
				t.Fatalf("code %q, want %q: %v", got, arch.EINVALID, err)
			}
		})
	}
}

// protoRepository writes a one-package module with a .git directory and a
// .proto file under api/proto that imports another, and a model whose
// fitness.protobuf rule names the import paths given, and returns its
// directory.
func protoRepository(t *testing.T, importPaths string) string {
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
	write("r.go", "package r\n")
	write(".git/HEAD", "ref: refs/heads/main\n")
	write("api/proto/tasks/v1/tasks.proto", `syntax = "proto3";
package tasks.v1;
import "tasks/v1/task.proto";
option go_package = "example.com/r/gen/tasks/v1";
service Tasks {
  rpc Get(Task) returns (Task);
}
`)
	write(
		"api/proto/tasks/v1/task.proto",
		"syntax = \"proto3\";\npackage tasks.v1;\nmessage Task {}\n",
	)
	write("anarch.json", `{"version": 1, "fitness": {"protobuf": [{"name": "p",
		"files": ["api/proto/tasks/v1/tasks.proto"], "importPaths": [`+importPaths+`]}]}}`)
	return dir
}

func TestLoadReadsProtoFiles(t *testing.T) {
	t.Parallel()
	in, err := load(t, protoRepository(t, `"api/proto"`))
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Protos) != 1 || len(in.Protos[0].Services) != 1 ||
		in.Protos[0].Services[0].Pos != (arch.Position{File: "api/proto/tasks/v1/tasks.proto", Line: 5}) {
		t.Fatalf("protos: %+v", in.Protos)
	}
}

func TestLoadRefusesProtoImportPaths(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"a file under no import path":          `"proto"`,
		"an import path out of the repository": `"../outside"`,
		"imports that resolve under no path":   `"."`,
	}
	for name, importPaths := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := load(t, protoRepository(t, importPaths))
			if got := arch.ErrorCode(err); got != arch.EINVALID {
				t.Fatalf("code %q, want %q: %v", got, arch.EINVALID, err)
			}
		})
	}
}
