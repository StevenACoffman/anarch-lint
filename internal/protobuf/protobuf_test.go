package protobuf_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/protobuf"
)

// importPath is testdata/proto, as an absolute import path.
func importPath(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "proto"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestParse: the services come from the files named, with their Go
// packages, and not from the files they import; standard imports and
// buf/validate/validate.proto resolve without a copy; a constraint for
// another field type, and a CEL expression that does not type-check, are
// problems at their messages.
func TestParse(t *testing.T) {
	t.Parallel()
	got, err := protobuf.Parse(context.Background(), protobuf.Source{
		Files: []protobuf.File{{
			Name: "tasks/v1/tasks.proto", Display: "proto/tasks/v1/tasks.proto",
		}},
		ImportPaths: []string{importPath(t)},
		Read:        os.ReadFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	at := func(line int) arch.Position {
		return arch.Position{File: "proto/tasks/v1/tasks.proto", Line: line}
	}
	if !reflect.DeepEqual([]arch.ProtoService{
		{Name: "tasks.v1.Tasks", GoPackage: "example.com/gen/tasks/v1", Pos: at(11)},
		{Name: "tasks.v1.task_admin", GoPackage: "example.com/gen/tasks/v1", Pos: at(15)},
	}, got.Services) {
		t.Fatalf("services: %+v", got.Services)
	}
	if got.Messages != 5 {
		t.Fatalf("expected 5 messages, got %d", got.Messages)
	}
	var problems []string
	for _, p := range got.Problems {
		problems = append(problems, p.Message+" "+p.Pos.File)
		if p.Error == "" {
			t.Fatalf("problem %s has no error", p.Message)
		}
	}
	want := []string{
		"tasks.v1.BadRule proto/tasks/v1/tasks.proto",
		"tasks.v1.BadExpression proto/tasks/v1/tasks.proto",
	}
	if !reflect.DeepEqual(want, problems) {
		t.Fatalf("expected %v, got %v (%+v)", want, problems, got.Problems)
	}
	if got.Problems[0].Pos.Line != 33 || got.Problems[1].Pos.Line != 37 {
		t.Fatalf("problem lines: %+v", got.Problems)
	}
}

// TestParseRefuses: a file that does not compile is EINVALID, naming it.
func TestParseRefuses(t *testing.T) {
	t.Parallel()
	_, err := protobuf.Parse(context.Background(), protobuf.Source{
		Files:       []protobuf.File{{Name: "broken.proto", Display: "proto/broken.proto"}},
		ImportPaths: []string{importPath(t)},
		Read:        os.ReadFile,
	})
	if arch.ErrorCode(err) != arch.EINVALID ||
		!strings.Contains(arch.ErrorMessage(err), "broken.proto:5") {
		t.Fatalf("expected EINVALID naming broken.proto:5, got %v", err)
	}
}
