package openapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/openapi"
)

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// parse reads testdata/name, refusing any file outside testdata as the
// shell refuses one outside the repository.
func parse(t *testing.T, name string) (arch.APIDocument, error) {
	t.Helper()
	root, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	read := func(file string) ([]byte, error) {
		if rel, err := filepath.Rel(root, file); err != nil || strings.HasPrefix(rel, "..") {
			return nil, errors.New(file + " is outside the repository")
		}
		return os.ReadFile(file)
	}
	return openapi.Parse(t.Context(), openapi.Source{
		File: filepath.Join(root, name), Name: "api/" + name, Read: read,
	})
}

// TestParse: every operation, in path and method order, at the line that
// declares it, in the file that does when a $ref leads elsewhere; and the
// servers' base paths, their variables filled with their defaults.
func TestParse(t *testing.T) {
	t.Parallel()
	doc, err := parse(t, "tasks.yaml")
	equals(t, nil, err)
	equals(t, arch.APIDocument{
		Path: "api/tasks.yaml", BasePaths: []string{"/v1"},
		Operations: []arch.APIOperation{
			{Method: "GET", Path: "/tasks", Pos: arch.Position{File: "api/tasks.yaml", Line: 12}},
			{Method: "POST", Path: "/tasks", Pos: arch.Position{File: "api/tasks.yaml", Line: 16}},
			{
				Method: "DELETE", Path: "/tasks/{taskId}",
				Pos: arch.Position{File: "api/paths/task.yaml", Line: 7},
			},
		},
	}, doc)
}

// TestParseSwagger: a Swagger 2.0 document is read as the OpenAPI 3 one it
// converts to, its basePath a server's; its lines are unknown.
func TestParseSwagger(t *testing.T) {
	t.Parallel()
	doc, err := parse(t, "swagger.yaml")
	equals(t, nil, err)
	equals(t, arch.APIDocument{
		Path: "api/swagger.yaml", BasePaths: []string{"/api"},
		Operations: []arch.APIOperation{
			{Method: "GET", Path: "/items", Pos: arch.Position{File: "api/swagger.yaml", Line: 1}},
		},
	}, doc)
}

// TestParseRefuses: a document whose reference leaves the repository, or
// names a URL, is not read, and neither is a file that is no OpenAPI
// document.
func TestParseRefuses(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"escape.yaml": "is outside the repository",
		"remote.yaml": "only files in the repository are read",
		"notapi.yaml": "not an OpenAPI document",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := parse(t, name)
			equals(t, arch.EINVALID, arch.ErrorCode(err))
			if msg := arch.ErrorMessage(err); !strings.Contains(msg, want) {
				t.Errorf("message %q does not say %q", msg, want)
			}
		})
	}
}
