package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestEveryRuleCanFail: every rule in the catalog is shown failing somewhere
// (IMPROVEMENTS.md §8.2): in a golden file, or in a test that names it by
// its constant or its name. A connascence rule is shown by an instance of
// its kind in the connascence golden file, since every kind becomes a
// finding by the one mechanism TestFactsOverTheLimitAreFindings pins. A new
// rule with no failing case fails this test by name.
func TestEveryRuleCanFail(t *testing.T) {
	t.Parallel()
	constants := ruleConstants(t)
	root, err := os.OpenRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	repo := root.FS()
	var golden, tests strings.Builder
	err = fs.WalkDir(repo, ".", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && path != "." && strings.HasPrefix(d.Name(), "."):
			return fs.SkipDir
		case d.IsDir():
			return nil
		}
		var out *strings.Builder
		switch {
		case strings.HasPrefix(path, "testdata/golden/"):
			out = &golden
		case strings.HasSuffix(path, "_test.go") && path != "rules_test.go" &&
			!strings.HasPrefix(path, "testdata/"):
			out = &tests
		default:
			return nil
		}
		src, err := fs.ReadFile(repo, path)
		out.Write(src)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range arch.Rules() {
		kind, isConnascence := strings.CutPrefix(r.Name, "connascence-")
		shown := strings.Contains(golden.String(), `"rule": "`+r.Name+`"`) ||
			namedInTests(tests.String(), r.Name) ||
			(constants[r.Name] != "" && strings.Contains(tests.String(), "arch."+constants[r.Name])) ||
			(isConnascence && strings.Contains(golden.String(), `"kind": "`+kind+`"`))
		if !shown {
			t.Errorf("no golden file or test shows rule %s failing", r.Name)
		}
	}
}

// namedInTests reports whether a string literal in the tests starts with the
// rule's name, alone or followed by a space or a colon, as a finding's rule
// is written in the tests' expectations.
func namedInTests(tests, name string) bool {
	for _, end := range []string{`"`, " ", ":"} {
		if strings.Contains(tests, `"`+name+end) {
			return true
		}
	}
	return false
}

// ruleConstants maps each rule name to the name of the constant that holds
// it in internal/arch/catalog.go.
func ruleConstants(t *testing.T) map[string]string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(),
		filepath.Join("internal", "arch", "catalog.go"), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Values) != 1 {
			return true
		}
		if lit, ok := spec.Values[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if value, err := strconv.Unquote(lit.Value); err == nil {
				out[value] = spec.Names[0].Name
			}
		}
		return true
	})
	return out
}
