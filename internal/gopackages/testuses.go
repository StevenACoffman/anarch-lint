package gopackages

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// testUsesOf returns the package-level objects of the packages under
// analysis that their external test packages name, as "pkg.Name", sorted:
// a use that keeps an identifier exported, which a program loaded without
// its tests cannot see (IMPROVEMENTS.md §9.2). The test files are parsed,
// not type-checked, so only qualified identifiers count.
func testUsesOf(pkgs []*packages.Package, inScope map[string]bool) ([]string, error) {
	seen := map[string]bool{}
	for _, p := range pkgs {
		if len(p.GoFiles) == 0 {
			continue
		}
		files, err := filepath.Glob(filepath.Join(filepath.Dir(p.GoFiles[0]), "*_test.go"))
		if err != nil {
			return nil, &arch.Error{Code: arch.EINVALID, Message: err.Error()}
		}
		for _, file := range files {
			if err := qualifiedUses(file, inScope, seen); err != nil {
				return nil, err
			}
		}
	}
	return slices.Sorted(maps.Keys(seen)), nil
}

// qualifiedUses adds to seen each in-scope pkg.Name an external test file
// names. An internal test file is skipped: being in the package, it does
// not need the name exported.
func qualifiedUses(file string, inScope, seen map[string]bool) error {
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		return &arch.Error{Code: arch.EINVALID, Message: "parse " + file + ": " + err.Error()}
	}
	if !strings.HasSuffix(f.Name.Name, "_test") {
		return nil
	}
	names := map[string]string{} // the name a file gives an import -> its path
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !inScope[path] {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = path
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && names[x.Name] != "" {
				seen[names[x.Name]+"."+sel.Sel.Name] = true
			}
		}
		return true
	})
	return nil
}
