package gopackages

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// extractor accumulates the facts of every package under analysis.
type extractor struct {
	dir     string
	inScope map[string]bool
	prog    arch.Program
	refs    map[[2]string]*arch.Ref    // keyed by (from, to)
	imports map[[2]string]*arch.Import // keyed by (from, to)
	types   map[string]int             // type ID -> index into prog.Types
	named   map[string]*types.TypeName // type ID -> its object, for interface checks
	// facts collects the connascence facts; nil when they are not wanted.
	facts *factSink
}

// fileContext is what extraction needs to know about the file it is in.
type fileContext struct {
	pkg   *packages.Package
	fset  *token.FileSet
	path  string // relative to the extractor's dir
	lines codeLines
}

// methodDecl remembers a method's receiver and size until its type is known.
type methodDecl struct {
	receiver string
	lines    int
}

func newExtractor(dir string, inScope map[string]bool, facts bool) *extractor {
	x := &extractor{
		dir:     dir,
		inScope: inScope,
		refs:    map[[2]string]*arch.Ref{},
		imports: map[[2]string]*arch.Import{},
		types:   map[string]int{},
		named:   map[string]*types.TypeName{},
	}
	if facts {
		x.facts = newFactSink(inScope)
	}
	return x
}

// addPackage extracts the facts of one package. readFile returns a source
// file's bytes, which go/packages does not keep.
func (x *extractor) addPackage(p *packages.Package, readFile func(string) ([]byte, error)) error {
	files, generated := sourceFiles(p)
	pkg := arch.Package{Path: p.PkgPath, Name: p.Name, Generated: generated}
	if p.Module != nil {
		pkg.Module = p.Module.Path
	}
	var methods []methodDecl
	for _, f := range files {
		name := p.Fset.File(f.Pos()).Name()
		src, err := readFile(name)
		if err != nil {
			return &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("read %s: %v", name, err)}
		}
		fc := fileContext{
			pkg:   p,
			fset:  p.Fset,
			path:  relPath(x.dir, name),
			lines: countCodeLines(src),
		}
		pkg.Files = append(pkg.Files, fc.path)
		x.prog.Files = append(
			x.prog.Files,
			arch.File{Path: fc.path, Package: p.PkgPath, Lines: fc.lines.total()},
		)
		pkg.Lines += fc.lines.total()
		x.addImports(&fc, f)
		if x.facts != nil {
			x.facts.linknames(&fc, f)
		}
		methods = append(methods, x.addDecls(&fc, f)...)
	}
	slices.Sort(pkg.Files)
	x.prog.Packages = append(x.prog.Packages, pkg)
	// A method's lines count toward its type, which may be declared in a
	// file read later, so they are added once the whole package is in.
	for _, m := range methods {
		if i, ok := x.types[m.receiver]; ok {
			x.prog.Types[i].Lines += m.lines
		}
	}
	return nil
}

// addImports records the imports of one file, keeping the first position of
// each package-to-package import.
func (x *extractor) addImports(fc *fileContext, f *ast.File) {
	for _, spec := range f.Imports {
		to := strings.Trim(spec.Path.Value, `"`)
		key := [2]string{fc.pkg.PkgPath, to}
		pos := fc.position(spec.Pos())
		if imp, ok := x.imports[key]; ok {
			if comparePositions(pos, imp.Pos) < 0 {
				imp.Pos = pos
			}
			continue
		}
		x.imports[key] = &arch.Import{From: fc.pkg.PkgPath, To: to, Pos: pos}
	}
}

// addDecls records the types and functions a file declares, and the type
// references inside them. It returns the file's methods.
func (x *extractor) addDecls(fc *fileContext, f *ast.File) []methodDecl {
	var methods []methodDecl
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			x.addTypeSpecs(fc, d)
			if x.facts != nil && (d.Tok == token.VAR || d.Tok == token.CONST) {
				x.facts.valueFacts(fc, d)
			}
		case *ast.FuncDecl:
			fn := x.addFunc(fc, d)
			if fn.Receiver != "" {
				methods = append(methods, methodDecl{receiver: fn.Receiver, lines: fn.Lines})
			}
		}
	}
	return methods
}

// addTypeSpecs records each type a declaration declares, with its references.
func (x *extractor) addTypeSpecs(fc *fileContext, d *ast.GenDecl) {
	if d.Tok != token.TYPE {
		return
	}
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		tn, ok := fc.pkg.TypesInfo.Defs[ts.Name].(*types.TypeName)
		if !ok {
			continue
		}
		span := fc.span(ts.Pos(), ts.End())
		t := arch.Type{
			ID:        typeID(tn),
			Package:   fc.pkg.PkgPath,
			Name:      tn.Name(),
			Kind:      kindOf(tn),
			Span:      span,
			Lines:     fc.lines.within(span),
			Fields:    fieldNames(ts),
			Embeds:    embedsOf(tn, x.inScope),
			Directive: directiveOf(ts, d),
		}
		x.types[t.ID] = len(x.prog.Types)
		x.named[t.ID] = tn
		x.prog.Types = append(x.prog.Types, t)
		x.addRefs(fc, tn, ts)
		if x.facts != nil {
			x.facts.typeFacts(fc, ts, t.ID)
		}
	}
}

// addFunc records a function or method and its type references.
func (x *extractor) addFunc(fc *fileContext, d *ast.FuncDecl) arch.Func {
	span := fc.span(d.Pos(), d.End())
	fn := arch.Func{
		Package:    fc.pkg.PkgPath,
		Name:       d.Name.Name,
		Span:       span,
		Lines:      fc.lines.within(span),
		Statements: countStatements(d),
		Complexity: complexityOf(d),
		MaxNesting: maxNesting(d),
	}
	owner := typeOwner(fc.pkg.TypesInfo.Defs[d.Name])
	fn.ID = fc.pkg.PkgPath + "." + d.Name.Name
	if owner != nil && d.Recv != nil {
		fn.Receiver = typeID(owner)
		fn.ID = fc.pkg.PkgPath + "." + owner.Name() + "." + d.Name.Name
		fn.Uses = receiverUses(fc.pkg.TypesInfo, d)
		x.addRefs(fc, owner, d)
	}
	x.addInjections(fc, d)
	if d.Name.Name == "init" || d.Name.Name == "_" {
		// Several may exist; the position keeps the ID unique.
		fn.ID += "@" + fc.path + ":" + strconv.Itoa(span.Start)
	}
	if x.facts != nil {
		x.facts.funcFacts(fc, d, &fn)
	}
	x.prog.Funcs = append(x.prog.Funcs, fn)
	return fn
}

// addRefs records a reference from the type from to every in-scope type an
// identifier inside node refers to, weighted by the identifiers.
func (x *extractor) addRefs(fc *fileContext, from *types.TypeName, node ast.Node) {
	fromID := typeID(from)
	ast.Inspect(node, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		to := typeOwner(fc.pkg.TypesInfo.Uses[id])
		if to == nil || !x.inScope[to.Pkg().Path()] {
			return true
		}
		toID := typeID(to)
		if toID == fromID {
			return true
		}
		pos := fc.position(id.Pos())
		key := [2]string{fromID, toID}
		if r, ok := x.refs[key]; ok {
			r.Weight++
			if comparePositions(pos, r.Pos) < 0 {
				r.Pos = pos
			}
			return true
		}
		x.refs[key] = &arch.Ref{From: fromID, To: toID, Weight: 1, Pos: pos}
		return true
	})
}

// program returns the accumulated facts, sorted.
func (x *extractor) program() arch.Program {
	x.implementations()

	p := x.prog
	if x.facts != nil {
		p.Facts = x.facts.sortFacts()
	}
	slices.SortFunc(p.Packages, func(a, b arch.Package) int { return cmp.Compare(a.Path, b.Path) })
	slices.SortFunc(p.Types, func(a, b arch.Type) int { return cmp.Compare(a.ID, b.ID) })
	slices.SortFunc(p.Funcs, func(a, b arch.Func) int { return cmp.Compare(a.ID, b.ID) })
	slices.SortFunc(p.Files, func(a, b arch.File) int { return cmp.Compare(a.Path, b.Path) })
	slices.SortFunc(p.Injections, func(a, b arch.Injection) int {
		return cmp.Or(
			cmp.Compare(a.CalleePackage, b.CalleePackage),
			cmp.Compare(a.Concrete, b.Concrete),
			cmp.Compare(a.Caller, b.Caller),
			comparePositions(a.Pos, b.Pos),
		)
	})
	for _, r := range x.refs {
		p.Refs = append(p.Refs, *r)
	}
	slices.SortFunc(p.Refs, func(a, b arch.Ref) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
	for _, imp := range x.imports {
		p.Imports = append(p.Imports, *imp)
	}
	slices.SortFunc(p.Imports, func(a, b arch.Import) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
	return p
}

// position returns the relative position of pos.
func (fc *fileContext) position(pos token.Pos) arch.Position {
	p := fc.fset.Position(pos)
	return arch.Position{File: fc.path, Line: p.Line}
}

// span returns the relative line span from start to end.
func (fc *fileContext) span(start, end token.Pos) arch.Span {
	return arch.Span{
		File:  fc.path,
		Start: fc.fset.Position(start).Line,
		End:   fc.fset.Position(end).Line,
	}
}

// typeOwner returns the package-level type that obj belongs to: the type
// itself, or the receiver type of a method. Fields, functions, variables,
// constants, predeclared types, type parameters and local types yield nil.
func typeOwner(obj types.Object) *types.TypeName {
	var tn *types.TypeName
	switch obj := obj.(type) {
	case *types.TypeName:
		tn = obj
	case *types.Func:
		if recv := obj.Signature().Recv(); recv != nil {
			t := recv.Type()
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			if n, ok := t.(*types.Named); ok {
				tn = n.Origin().Obj()
			}
		}
	}
	if tn == nil || tn.Pkg() == nil || tn.Parent() != tn.Pkg().Scope() {
		return nil
	}
	return tn
}

func typeID(tn *types.TypeName) string { return tn.Pkg().Path() + "." + tn.Name() }

// kindOf classifies a named type by its underlying type.
func kindOf(tn *types.TypeName) arch.TypeKind {
	switch tn.Type().Underlying().(type) {
	case *types.Struct:
		return arch.KindStruct
	case *types.Interface:
		return arch.KindInterface
	default:
		return arch.KindOther
	}
}

// fieldNames returns the field names of a struct type spec, an embedded field
// counting under its type's name.
func fieldNames(ts *ast.TypeSpec) []string {
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return nil
	}
	var names []string
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
		if len(f.Names) == 0 {
			names = append(names, embeddedName(f.Type))
		}
	}
	return names
}

// embeddedName returns the name an embedded field is selected by.
func embeddedName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return embeddedName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.IndexExpr:
		return embeddedName(t.X)
	case *ast.IndexListExpr:
		return embeddedName(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

func comparePositions(a, b arch.Position) int {
	return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
}
