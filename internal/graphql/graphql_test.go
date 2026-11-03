package graphql_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/graphql"
)

// files names testdata files as a rule's globs would match them.
func files(t *testing.T, names ...string) []graphql.File {
	t.Helper()
	out := make([]graphql.File, 0, len(names))
	for _, n := range names {
		abs, err := filepath.Abs(filepath.Join("testdata", n))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, graphql.File{Path: abs, Name: n})
	}
	return out
}

// TestParse: the root fields come from every schema file, extend type
// included, without the introspection fields; each operation document's
// syntax and validation errors are problems at their lines.
func TestParse(t *testing.T) {
	t.Parallel()
	got, err := graphql.Parse(graphql.Source{
		Schema:     files(t, "schema.graphqls", "search.graphqls"),
		Operations: files(t, "ops/broken.graphql", "ops/invalid.graphql", "ops/valid.graphql"),
		Read:       os.ReadFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := arch.GraphQLSchema{
		Fields: []arch.GraphQLField{
			{
				Root: "Mutation",
				Name: "createTask",
				Pos:  arch.Position{File: "schema.graphqls", Line: 12},
			},
			{Root: "Query", Name: "task", Pos: arch.Position{File: "schema.graphqls", Line: 8}},
			{
				Root: "Query", Name: "task_search",
				Pos: arch.Position{File: "search.graphqls", Line: 2},
			},
			{Root: "Query", Name: "tasks", Pos: arch.Position{File: "schema.graphqls", Line: 7}},
		},
		Operations: 3,
		Problems: []arch.GraphQLProblem{
			{
				Message: "Expected Name, found <EOF>",
				Pos:     arch.Position{File: "ops/broken.graphql", Line: 3},
			},
			{
				Message: `Cannot query field "owner" on type "Task".`,
				Pos:     arch.Position{File: "ops/invalid.graphql", Line: 4},
			},
		},
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

// TestParseRefuses: a schema that does not load is EINVALID, naming the
// file and line.
func TestParseRefuses(t *testing.T) {
	t.Parallel()
	_, err := graphql.Parse(graphql.Source{Schema: files(t, "bad.graphqls"), Read: os.ReadFile})
	if arch.ErrorCode(err) != arch.EINVALID ||
		!strings.Contains(arch.ErrorMessage(err), "bad.graphqls:2") {
		t.Fatalf("expected EINVALID naming bad.graphqls:2, got %v", err)
	}
}
