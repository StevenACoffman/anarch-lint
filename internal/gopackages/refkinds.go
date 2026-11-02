package gopackages

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// region is a span of a declaration whose references all have one kind.
type region struct {
	start, end token.Pos
	kind       arch.RefKind
}

// bodyRef is what a method body's syntax says of one identifier: the kind
// of its reference, and, for a field, the type that declares it, which the
// field's own object does not record.
type bodyRef struct {
	kind  arch.RefKind
	owner *types.TypeName
}

// bodyClassifier gathers the classified identifiers of one method body.
type bodyClassifier struct {
	info *types.Info
	out  map[*ast.Ident]bodyRef
}

// refRegions splits a type or method declaration into the regions that give
// a reference its kind (see [fieldRegions]); in a method, its receiver and
// signature. A reference outside every region has the declaration's default
// kind: definition for a type, body for a method.
func refRegions(node ast.Node) (regions []region, fallback arch.RefKind) {
	switch d := node.(type) {
	case *ast.TypeSpec:
		switch t := d.Type.(type) {
		case *ast.StructType:
			return fieldRegions(t.Fields, arch.RefField), arch.RefDefinition
		case *ast.InterfaceType:
			return fieldRegions(t.Methods, arch.RefSignature), arch.RefDefinition
		}
	case *ast.FuncDecl:
		return []region{{d.Recv.Pos(), d.Type.End(), arch.RefSignature}}, arch.RefBody
	}
	return nil, arch.RefDefinition
}

// fieldRegions gives each entry of a struct's fields or an interface's
// methods its region: named entries have the kind named, and embedded ones
// are embeddings.
func fieldRegions(list *ast.FieldList, named arch.RefKind) []region {
	out := make([]region, 0, len(list.List))
	for _, f := range list.List {
		kind := named
		if len(f.Names) == 0 {
			kind = arch.RefEmbed
		}
		out = append(out, region{f.Type.Pos(), f.Type.End(), kind})
	}
	return out
}

// kindAt returns the kind of a reference at pos.
func kindAt(regions []region, fallback arch.RefKind, pos token.Pos) arch.RefKind {
	for _, r := range regions {
		if r.start <= pos && pos < r.end {
			return r.kind
		}
	}
	return fallback
}

// addKind adds kind to a reference's kinds, keeping them hardest to cut
// first.
func addKind(r *arch.Ref, kind arch.RefKind) {
	if slices.Contains(r.Kinds, kind) {
		return
	}
	r.Kinds = append(r.Kinds, kind)
	slices.SortFunc(r.Kinds, func(a, b arch.RefKind) int {
		return a.Order() - b.Order()
	})
}

// bodyRefs classifies the identifiers of a method body by the syntax around
// them (IMPROVEMENTS.md §3.1): a composite literal's type, a conversion's, a
// called method, a field read or written, and a type argument. An
// identifier it does not classify is a plain use in the body. An assignment
// is visited before the selectors in it, so a written field is not also
// classified as read.
func bodyRefs(info *types.Info, body *ast.BlockStmt) map[*ast.Ident]bodyRef {
	c := &bodyClassifier{info: info, out: map[*ast.Ident]bodyRef{}}
	ast.Inspect(body, func(n ast.Node) bool {
		c.literal(n)
		c.conversion(n)
		c.selector(n)
		c.writes(n)
		c.typeArguments(n)
		return true
	})
	return c.out
}

// literal classifies a composite literal: its type is a literal reference,
// and each key naming a field writes that field.
func (c *bodyClassifier) literal(n ast.Node) {
	lit, ok := n.(*ast.CompositeLit)
	if !ok {
		return
	}
	setRef(c.out, typeIdent(lit.Type), bodyRef{kind: arch.RefLiteral})
	owner := namedOwner(c.info.TypeOf(lit))
	if owner == nil {
		return
	}
	for _, e := range lit.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && isField(c.info.Uses[key]) {
				setRef(c.out, key, bodyRef{arch.RefFieldWrite, owner})
			}
		}
	}
}

// conversion classifies the type a call converts to.
func (c *bodyClassifier) conversion(n ast.Node) {
	if call, ok := n.(*ast.CallExpr); ok {
		if tv, ok := c.info.Types[call.Fun]; ok && tv.IsType() {
			setRef(c.out, typeIdent(call.Fun), bodyRef{kind: arch.RefConversion})
		}
	}
}

// selector classifies a selector: a field read, unless an assignment
// already marked it written, or a method called or taken as a value. A
// qualified identifier, pkg.Name, has no selection and is left alone.
func (c *bodyClassifier) selector(n ast.Node) {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return
	}
	s := c.info.Selections[sel]
	switch {
	case s == nil:
	case s.Kind() == types.FieldVal:
		if _, written := c.out[sel.Sel]; !written {
			if owner := selectedFieldOwner(s); owner != nil {
				setRef(c.out, sel.Sel, bodyRef{arch.RefFieldRead, owner})
			}
		}
	default:
		setRef(c.out, sel.Sel, bodyRef{kind: arch.RefCall})
	}
}

// writes marks the fields an assignment or an increment writes.
func (c *bodyClassifier) writes(n ast.Node) {
	var targets []ast.Expr
	switch n := n.(type) {
	case *ast.AssignStmt:
		targets = n.Lhs
	case *ast.IncDecStmt:
		targets = []ast.Expr{n.X}
	}
	for _, lhs := range targets {
		sel := writtenSelector(lhs)
		if sel == nil {
			continue
		}
		if s := c.info.Selections[sel]; s != nil && s.Kind() == types.FieldVal {
			if owner := selectedFieldOwner(s); owner != nil {
				setRef(c.out, sel.Sel, bodyRef{arch.RefFieldWrite, owner})
			}
		}
	}
}

// typeArguments marks each index of an instantiation that is a type as a
// type argument.
func (c *bodyClassifier) typeArguments(n ast.Node) {
	var indices []ast.Expr
	switch n := n.(type) {
	case *ast.IndexExpr:
		indices = []ast.Expr{n.Index}
	case *ast.IndexListExpr:
		indices = n.Indices
	}
	for _, idx := range indices {
		if tv, ok := c.info.Types[idx]; ok && tv.IsType() {
			setRef(c.out, typeIdent(idx), bodyRef{kind: arch.RefTypeArgument})
		}
	}
}

// writtenSelector returns the selector an assignment's left-hand side
// writes through: x.f itself, or x.f in x.f[i] or *x.f; nil when there is
// none.
func writtenSelector(lhs ast.Expr) *ast.SelectorExpr {
	for {
		switch e := lhs.(type) {
		case *ast.ParenExpr:
			lhs = e.X
		case *ast.IndexExpr:
			lhs = e.X
		case *ast.StarExpr:
			lhs = e.X
		case *ast.SelectorExpr:
			return e
		default:
			return nil
		}
	}
}

// setRef records an identifier's reference unless id is nil.
func setRef(out map[*ast.Ident]bodyRef, id *ast.Ident, r bodyRef) {
	if id != nil {
		out[id] = r
	}
}

// typeIdent returns the identifier naming the type a type expression
// denotes: T, pkg.T, *T, or the generic T of T[A].
func typeIdent(expr ast.Expr) *ast.Ident {
	for {
		switch e := expr.(type) {
		case *ast.Ident:
			return e
		case *ast.SelectorExpr:
			return e.Sel
		case *ast.ParenExpr:
			expr = e.X
		case *ast.StarExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		default:
			return nil
		}
	}
}

// selectedFieldOwner returns the package-level type that declares a selected
// field, following embedded fields, or nil for a field of an unnamed struct.
func selectedFieldOwner(s *types.Selection) *types.TypeName {
	return typeOwner(fieldOwner(s.Recv(), s.Index()))
}

// namedOwner returns the package-level type a type or a pointer to it names,
// or nil.
func namedOwner(t types.Type) *types.TypeName {
	if n := namedOf(t); n != nil {
		return typeOwner(n.Origin().Obj())
	}
	return nil
}

// isField reports whether obj is a struct field.
func isField(obj types.Object) bool {
	v, ok := obj.(*types.Var)
	return ok && v.IsField()
}
