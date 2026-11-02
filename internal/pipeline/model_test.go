package pipeline_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
)

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// A YAML model means and hashes what its JSON spelling does; a YAML model
// that does not parse is malformed.
func TestDecodeModelFile(t *testing.T) {
	t.Parallel()
	js := `{"version": 1, "layers": [{"name": "app", "packages": ["m/app"]}]}`
	yml := "version: 1\nlayers:\n  - name: app\n    packages: [m/app]\n"
	a, err := pipeline.DecodeModelFile("anarch.json", []byte(js))
	if err != nil {
		t.Fatal(err)
	}
	b, err := pipeline.DecodeModelFile("anarch.yaml", []byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	equals(t, a.Hash(), b.Hash())
	_, err = pipeline.DecodeModelFile("anarch.yml", []byte("version: [1"))
	equals(t, "model: malformed: not YAML: yaml: line 1: did not find expected ',' or ']'",
		arch.ErrorMessage(err))
	_, err = pipeline.DecodeModelFile("anarch.yaml", []byte("version: 1\nlayer: []\n"))
	equals(t, `model: malformed: json: unknown field "layer"`, arch.ErrorMessage(err))
}

// With no --model, anarch.json is the model, else anarch.yaml, else
// anarch.yml.
func TestModelFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	o := pipeline.Options{Dir: dir}
	equals(t, filepath.Join(dir, "anarch.json"), o.ModelFile())
	for _, name := range []string{"anarch.yml", "anarch.yaml", "anarch.json"} {
		if err := os.WriteFile(
			filepath.Join(dir, name),
			[]byte("version: 1\n"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		equals(t, filepath.Join(dir, name), o.ModelFile())
	}
}
