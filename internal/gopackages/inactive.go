package gopackages

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// testSource is what one test file's source gives: its imports, the
// enforcers its directives mark, and its calls of package-level functions.
type testSource struct {
	imports   []arch.Import
	enforcers []arch.Enforcer
	calls     []arch.Call
}

// addFromSource records what the type checker does not see, read from
// source: the imports of the files the build excludes, and the imports and
// enforcers of the test files.
func (x *extractor) addFromSource(
	p *packages.Package,
	readFile func(string) ([]byte, error),
) error {
	if err := x.addInactiveImports(p, readFile); err != nil {
		return err
	}
	return x.addTestSources(p, readFile)
}

// addInactiveImports records the imports of the files the build constraints
// exclude from p, which the load lists by name only: each is read and its
// imports parsed from source, keeping the first position of each
// package-to-package import, so the import rules check a file of another
// platform too. Test files are not read, as they are not loaded.
func (x *extractor) addInactiveImports(
	p *packages.Package,
	readFile func(string) ([]byte, error),
) error {
	for _, name := range p.IgnoredFiles {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := readFile(name)
		if err != nil {
			return &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("read %s: %v", name, err)}
		}
		_, imports := sourceImports(name, src)
		file := relPath(x.dir, name)
		for _, spec := range imports {
			keepFirst(x.inactive, arch.Import{
				From: p.PkgPath, To: spec.to,
				Pos:      arch.Position{File: file, Line: spec.line},
				Inactive: true,
			})
		}
	}
	return nil
}

// addTestSources records the imports of p's test files, internal and
// external, read from their source as the inactive imports are, keeping
// the first position of each import: the import rules that opt in check
// them (IMPROVEMENTS.md §9.7). It records their enforcers too, since a test
// is where an invariant is most often checked (docs/adr/0016). An external
// test package, p's name with "_test", counts as p.
func (x *extractor) addTestSources(
	p *packages.Package,
	readFile func(string) ([]byte, error),
) error {
	if len(p.GoFiles) == 0 {
		return nil
	}
	names, err := filepath.Glob(filepath.Join(filepath.Dir(p.GoFiles[0]), "*_test.go"))
	if err != nil {
		return &arch.Error{Code: arch.EINVALID, Message: err.Error()}
	}
	for _, name := range names {
		src, err := readFile(name)
		if err != nil {
			return &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("read %s: %v", name, err)}
		}
		t := testFile(relPath(x.dir, name), p.PkgPath, src)
		for i := range t.imports {
			keepFirst(x.tests, t.imports[i])
		}
		x.prog.Enforcers = append(x.prog.Enforcers, t.enforcers...)
		x.prog.TestCalls = append(x.prog.TestCalls, t.calls...)
	}
	return nil
}

// testFile reads one test file of the package pkg, given its path as
// reported and its source. A file that does not parse gives nothing, as for
// the inactive imports: go test reports it.
func testFile(file, pkg string, src []byte) testSource {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, file, src, parser.ParseComments|parser.SkipObjectResolution)
	if f == nil {
		return testSource{}
	}
	var imports []arch.Import
	for _, spec := range importsOf(fset, f) {
		if spec.to == pkg {
			continue // an external test importing the package it tests
		}
		imports = append(imports, arch.Import{
			From: pkg, To: spec.to, Pos: arch.Position{File: file, Line: spec.line}, Test: true,
		})
	}
	var enforcers []arch.Enforcer
	for _, decl := range f.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok {
			enforcers = append(enforcers, enforcersOf(d, &arch.Enforcer{
				Func: pkg + "." + sourceFuncName(d), Package: pkg,
				Pos: arch.Position{File: file, Line: fset.Position(d.Pos()).Line}, Test: true,
			})...)
		}
	}
	return testSource{imports: imports, enforcers: enforcers, calls: testCalls(fset, f, file, pkg)}
}

// sourceFuncName names a function declared in source as a Func's ID does
// after the package: "Name", or "Recv.Name" for a method.
func sourceFuncName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	t := d.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	switch r := t.(type) {
	case *ast.IndexExpr: // a generic receiver, T[P]
		t = r.X
	case *ast.IndexListExpr:
		t = r.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + d.Name.Name
	}
	return d.Name.Name
}
