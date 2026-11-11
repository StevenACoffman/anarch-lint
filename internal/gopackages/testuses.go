package gopackages

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// majorVersion matches a module path's major version element, "v2" and on.
var majorVersion = regexp.MustCompile(`^v\d+$`)

// callTally counts calls by caller and callee, keeping the first site of
// each, in the order first seen.
type callTally struct {
	calls []arch.Call
	index map[[2]string]int // caller and callee -> its call in calls
}

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

// testCalls returns the calls in a test file of the package pkg of
// functions of the packages it imports, one per calling function and callee
// with the first site, in source order. The file is parsed, not
// type-checked, so a call counts only as pkg.Name(…) through an import's
// name, and an import whose name the file declares anywhere is left out
// rather than guessed at: a method call, a call through a variable, and a
// dot import are not seen, and a conversion such as time.Duration(n) looks
// like a call.
func testCalls(fset *token.FileSet, f *ast.File, file, pkg string) []arch.Call {
	names := importNames(f)
	for name := range declaredNames(f) {
		delete(names, name)
	}
	tally := callTally{index: map[[2]string]int{}}
	for _, decl := range f.Decls {
		caller := ""
		if d, ok := decl.(*ast.FuncDecl); ok {
			caller = pkg + "." + sourceFuncName(d)
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			if callee, line := qualifiedCall(fset, n, names); callee != "" {
				tally.add(callee, arch.Site{File: file, Line: line, Package: pkg, Func: caller})
			}
			return true
		})
	}
	return tally.calls
}

// qualifiedCall returns the callee of a call pkg.Name(…) through one of the
// names, and its line; "" when n is no such call.
func qualifiedCall(fset *token.FileSet, n ast.Node, names map[string]string) (string, int) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return "", 0
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", 0
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || names[x.Name] == "" {
		return "", 0
	}
	return names[x.Name] + "." + sel.Sel.Name, fset.Position(call.Pos()).Line
}

// add counts one call of callee at site.
func (t *callTally) add(callee string, site arch.Site) {
	key := [2]string{site.Func, callee}
	if i, seen := t.index[key]; seen {
		t.calls[i].Count++
		return
	}
	t.index[key] = len(t.calls)
	t.calls = append(t.calls, arch.Call{Callee: callee, Count: 1, Site: site})
}

// importNames maps the name a file gives each import to its path: the name
// written, or else the path's last element, before a major version suffix
// such as "/v2". A dot or blank import, and a last element that is not an
// identifier, such as "go-yaml", are left out.
func importNames(f *ast.File) map[string]string {
	names := map[string]string{}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		elems := strings.Split(path, "/")
		name := elems[len(elems)-1]
		if len(elems) > 1 && majorVersion.MatchString(name) {
			name = elems[len(elems)-2]
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name != "." && name != "_" && token.IsIdentifier(name) {
			names[name] = path
		}
	}
	return names
}

// declaredNames returns every name the file declares, at any scope: its
// functions, types, variables, constants, parameters, results, struct
// fields, labels and the names := and range define. Any of them may hide an
// import of that name somewhere, which only scope resolution could tell.
func declaredNames(f *ast.File) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		for _, id := range declares(n) {
			if id != nil {
				out[id.Name] = true
			}
		}
		return true
	})
	return out
}

// declares returns the names node n declares itself, nil among them where
// a := or range assigns to something other than a name (defines).
func declares(n ast.Node) []*ast.Ident {
	switch n := n.(type) {
	case *ast.FuncDecl:
		return []*ast.Ident{n.Name}
	case *ast.TypeSpec:
		return []*ast.Ident{n.Name}
	case *ast.ValueSpec:
		return n.Names
	case *ast.Field:
		return n.Names
	case *ast.LabeledStmt:
		return []*ast.Ident{n.Label}
	}
	return defines(n)
}

// defines returns the names a := or a range with := defines, nil among
// them where it assigns to something other than a name.
func defines(n ast.Node) []*ast.Ident {
	var lhs []ast.Expr
	switch n := n.(type) {
	case *ast.AssignStmt:
		if n.Tok == token.DEFINE {
			lhs = n.Lhs
		}
	case *ast.RangeStmt:
		if n.Tok == token.DEFINE {
			lhs = []ast.Expr{n.Key, n.Value}
		}
	}
	out := make([]*ast.Ident, 0, len(lhs))
	for _, e := range lhs {
		id, _ := e.(*ast.Ident)
		out = append(out, id)
	}
	return out
}
