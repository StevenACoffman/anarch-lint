package gopackages

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// sourceFile is one source file the measures read: its syntax, the absolute
// path of the file it stands for, the file's bytes, and its imports.
type sourceFile struct {
	syntax  *ast.File
	name    string
	src     []byte
	imports []importAt
}

// importAt is one import of a source file: the imported path and the line
// of its import spec.
type importAt struct {
	to   string
	line int
}

// sourceFiles returns the files of p that the measures read, and the files
// they do not: the generated ones, and those its build constraints leave out,
// with paths relative to dir. Test files are not loaded at all.
//
// go/packages type-checks the file cgo writes in place of a file that imports
// C, in the build cache. Its line directives name the source file, which it
// stands for: the source decides whether it is generated, and the source's
// imports are its imports, so it imports C and not the unsafe cgo puts in
// C's place. The files cgo writes for no source file, its declarations of C's
// types and its stubs, are neither read nor listed.
func sourceFiles(
	dir string,
	p *packages.Package,
	readFile func(string) ([]byte, error),
) (files []sourceFile, excluded []arch.ExcludedFile, err error) {
	for _, f := range p.Syntax {
		sf, generated, err := readSource(p, f, readFile)
		switch {
		case err != nil:
			return nil, nil, err
		case sf.name == "":
		case generated:
			excluded = append(excluded, arch.ExcludedFile{
				Path: relPath(dir, sf.name), Package: p.PkgPath, Reason: arch.ExcludedGenerated,
			})
		default:
			files = append(files, sf)
		}
	}
	for _, name := range p.IgnoredFiles {
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			excluded = append(excluded, arch.ExcludedFile{
				Path: relPath(dir, name), Package: p.PkgPath, Reason: arch.ExcludedByBuild,
			})
		}
	}
	return files, excluded, nil
}

// readSource returns the source file a syntax tree of p stands for, with its
// bytes and imports, and whether it is generated, which decides it unread. A
// tree that stands for no file, one cgo writes for itself, has no name.
func readSource(
	p *packages.Package,
	f *ast.File,
	readFile func(string) ([]byte, error),
) (sf sourceFile, generated bool, err error) {
	name, cgo := sourceOf(p, f)
	sf = sourceFile{syntax: f, name: name}
	if name == "" || !cgo && ast.IsGenerated(f) {
		return sf, name != "", nil
	}
	if sf.src, err = readFile(name); err != nil {
		return sourceFile{}, false, &arch.Error{
			Code: arch.EINVALID, Message: fmt.Sprintf("read %s: %v", name, err),
		}
	}
	if cgo {
		generated, sf.imports = sourceImports(name, sf.src)
		return sf, generated, nil
	}
	sf.imports = importsOf(p.Fset, f)
	return sf, false, nil
}

// sourceOf returns the absolute path of the source file a syntax tree of p
// stands for, and whether cgo rewrote it: the tree's own file when p lists it
// among its Go files, or else the Go file its package clause's line
// directive names. It returns "" for a file cgo writes for no Go file.
func sourceOf(p *packages.Package, f *ast.File) (name string, cgo bool) {
	if own := p.Fset.File(f.Pos()).Name(); slices.Contains(p.GoFiles, own) {
		return own, false
	}
	if named := p.Fset.Position(f.Package).Filename; slices.Contains(p.GoFiles, named) {
		return named, true
	}
	return "", false
}

// importsOf returns the imports of a parsed file, at the lines its position
// information gives.
func importsOf(fset *token.FileSet, f *ast.File) []importAt {
	out := make([]importAt, 0, len(f.Imports))
	for _, spec := range f.Imports {
		to, _ := strconv.Unquote(spec.Path.Value)
		out = append(out, importAt{to: to, line: fset.Position(spec.Pos()).Line})
	}
	return out
}

// sourceImports reads what a file's source says about it: whether it is
// generated, and its imports. It serves a cgo file, whose type-checked
// stand-in says neither, and a file the build constraints exclude, which has
// no syntax in the load. A source whose imports do not parse contributes the
// imports before the error, as go/parser returns them.
func sourceImports(name string, src []byte) (generated bool, imports []importAt) {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, name, src, parser.ImportsOnly|parser.ParseComments)
	if f == nil {
		return false, nil
	}
	return ast.IsGenerated(f), importsOf(fset, f)
}
