package gopackages_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/gopackages"
)

// TestFoldSprintf: a call to fmt.Sprintf of constants folds to the string
// it prints; one with any value known only at run time, or a spread
// argument, does not.
func TestFoldSprintf(t *testing.T) {
	t.Parallel()
	const src = `package p

import "fmt"

const uuid = "[0-9a-f]{8}"

func routes(n int, rest []any) []string {
	return []string{
		fmt.Sprintf("/tasks/{id:%s}", uuid),
		fmt.Sprintf("/v%d/%s/%t", 2, "x", true),
		fmt.Sprintf("/tasks/%d", n),
		fmt.Sprintf("/tasks", rest...),
		fmt.Sprint("/tasks"),
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Uses:  map[*ast.Ident]types.Object{},
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("p", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	var got []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, e := range lit.Elts {
			s, ok := gopackages.FoldSprintf(info, e)
			if !ok {
				s = "-"
			}
			got = append(got, s)
		}
		return false
	})
	want := []string{"/tasks/{id:[0-9a-f]{8}}", "/v2/x/true", "-", "-", "-"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("element %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
