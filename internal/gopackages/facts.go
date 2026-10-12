package gopackages

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// useKey identifies the references of one function, or one type
// declaration, to one object.
type useKey struct{ object, fn, typ, pkg string }

// factSink accumulates the connascence facts of every package under
// analysis (connascence.md §4), apart from the program facts the extractor
// keeps.
type factSink struct {
	inScope map[string]bool
	facts   arch.Facts
	uses    useTable
	calls   callTable
}

// useTable aggregates the Use facts per object and referring declaration.
type useTable map[useKey]*arch.Use

// callTable aggregates the Call facts per callee and calling declaration.
type callTable map[useKey]*arch.Call

// factWalker extracts the connascence facts of one declaration
// (connascence.md §4). It is created per declaration, so its maps describe
// that declaration only.
type factWalker struct {
	facts   *arch.Facts
	uses    useTable
	calls   callTable
	inScope map[string]bool
	fc      *fileContext
	// fn and typ are the enclosing function and type IDs every site carries.
	fn, typ string
	// recv is a method's receiver variable, for guards and field writes.
	recv types.Object
	// origins maps a local assigned once from a call to the callee's full
	// name; a second assignment maps it to "", which ends the tracking.
	origins map[types.Object]string
	// handled marks identifiers a parent node already recorded, writes the
	// identifiers being assigned, deferred the calls of defer statements.
	handled  map[*ast.Ident]bool
	writes   map[*ast.Ident]bool
	deferred map[*ast.CallExpr]bool
}

// walker returns a walker for one declaration of the file fc.
func (s *factSink) walker(fc *fileContext, fn, typ string) *factWalker {
	return &factWalker{
		facts:    &s.facts,
		uses:     s.uses,
		calls:    s.calls,
		inScope:  s.inScope,
		fc:       fc,
		fn:       fn,
		typ:      typ,
		origins:  map[types.Object]string{},
		handled:  map[*ast.Ident]bool{},
		writes:   map[*ast.Ident]bool{},
		deferred: map[*ast.CallExpr]bool{},
	}
}

func newFactSink(inScope map[string]bool) *factSink {
	return &factSink{inScope: inScope, uses: useTable{}, calls: callTable{}}
}

// addUse adds one reference, folding it into an earlier reference of the
// same declaration to the same object, whose count it raises and whose site
// it keeps when that comes first.
func (t useTable) add(u *arch.Use) {
	key := useKey{object: u.Object, fn: u.Site.Func, typ: u.Site.Type, pkg: u.Site.Package}
	if prev, ok := t[key]; ok {
		prev.Count++
		if compareSites(u.Site, prev.Site) < 0 {
			prev.Site = u.Site
		}
		return
	}
	stored := *u
	t[key] = &stored
}

// add adds one call, folding it into an earlier call of the same callee from
// the same declaration.
func (t callTable) add(c *arch.Call) {
	key := useKey{object: c.Callee, fn: c.Site.Func, typ: c.Site.Type, pkg: c.Site.Package}
	if prev, ok := t[key]; ok {
		prev.Count++
		if compareSites(c.Site, prev.Site) < 0 {
			prev.Site = c.Site
		}
		return
	}
	stored := *c
	t[key] = &stored
}

// site returns the site of pos in the walker's declaration.
func (w *factWalker) site(pos token.Pos) arch.Site {
	p := w.fc.position(pos)
	return arch.Site{
		File: p.File, Line: p.Line, Package: w.fc.pkg.PkgPath, Func: w.fn, Type: w.typ,
	}
}

func (w *factWalker) info() *types.Info { return w.fc.pkg.TypesInfo }

// walk records the facts of every node under n. ast.Inspect visits a parent
// before its children, so a statement marks its writes and deferred calls
// before the identifiers and calls inside it are recorded.
func (w *factWalker) walk(n ast.Node) {
	ast.Inspect(n, func(n ast.Node) bool {
		if stmt, ok := n.(ast.Stmt); ok {
			w.stmt(stmt)
		} else {
			w.expr(n)
		}
		return true
	})
}

// stmt records the facts a statement carries itself.
func (w *factWalker) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.SwitchStmt:
		w.switchStmt(s)
	case *ast.AssignStmt:
		w.assign(s)
	case *ast.IncDecStmt:
		w.markWrite(s.X)
	case *ast.DeferStmt:
		w.deferred[s.Call] = true
	}
}

// expr records the facts of an expression node.
func (w *factWalker) expr(n ast.Node) {
	switch n := n.(type) {
	case *ast.Ident:
		w.ident(n)
	case *ast.SelectorExpr:
		w.selector(n)
	case *ast.CompositeLit:
		w.compositeLit(n)
	case *ast.BinaryExpr:
		w.binary(n)
	case *ast.CallExpr:
		w.call(n)
	case *ast.IndexExpr:
		w.index(n)
	}
}

// ident records a use of a package-level object or a method, and a read or
// write of a package variable. Fields are recorded by their selector or
// composite literal, which know the field's type.
func (w *factWalker) ident(id *ast.Ident) {
	obj := w.inScopeUse(id)
	if obj == nil {
		return
	}
	if fn, ok := obj.(*types.Func); ok {
		if owner := typeOwner(fn); owner != nil {
			w.addUse(fn, arch.ObjectMethod, typeID(owner), id.Pos())
		} else if fn.Signature().Recv() == nil {
			w.addUse(fn, arch.ObjectFunc, "", id.Pos())
		}
		return
	}
	if obj.Parent() != obj.Pkg().Scope() {
		return // a local, or a field reached without a selector
	}
	switch obj.(type) {
	case *types.TypeName:
		w.addUse(obj, arch.ObjectType, "", id.Pos())
	case *types.Const:
		w.addUse(obj, arch.ObjectConst, "", id.Pos())
	case *types.Var:
		w.globalUse(id, obj)
	}
}

// inScopeUse returns the in-scope object id refers to, or nil for a
// definition, an identifier a parent recorded, or an object outside the scope.
func (w *factWalker) inScopeUse(id *ast.Ident) types.Object {
	if w.handled[id] {
		return nil
	}
	obj := w.info().Uses[id]
	if obj == nil || obj.Pkg() == nil || !w.inScope[obj.Pkg().Path()] {
		return nil
	}
	return obj
}

// globalUse records a read or write of a package-level variable.
func (w *factWalker) globalUse(id *ast.Ident, obj types.Object) {
	w.facts.GlobalUses = append(w.facts.GlobalUses, arch.GlobalUse{
		Var:      objectID(obj, ""),
		Package:  obj.Pkg().Path(),
		Exported: obj.Exported(),
		Write:    w.writes[id],
		Site:     w.site(id.Pos()),
	})
	w.addUse(obj, arch.ObjectVar, "", id.Pos())
}

// selector records a field selected through a value.
func (w *factWalker) selector(sel *ast.SelectorExpr) {
	s, ok := w.info().Selections[sel]
	if !ok || s.Kind() != types.FieldVal {
		return
	}
	owner := fieldOwner(s.Recv(), s.Index())
	if owner == nil || !w.inScope[owner.Pkg().Path()] {
		return
	}
	w.handled[sel.Sel] = true
	w.addUse(s.Obj(), arch.ObjectField, typeID(owner), sel.Sel.Pos())
}

// compositeLit records the fields a keyed literal names, and an unkeyed
// literal of a named struct type.
func (w *factWalker) compositeLit(lit *ast.CompositeLit) {
	named := namedOf(w.info().TypeOf(lit))
	if named == nil || len(lit.Elts) == 0 {
		return
	}
	if _, ok := named.Underlying().(*types.Struct); !ok {
		return
	}
	tn := named.Origin().Obj()
	if _, keyed := lit.Elts[0].(*ast.KeyValueExpr); !keyed {
		w.facts.UnkeyedLits = append(w.facts.UnkeyedLits, arch.UnkeyedLit{
			Type: typeID(tn), Package: tn.Pkg().Path(), Site: w.site(lit.Pos()),
		})
		return
	}
	if !w.inScope[tn.Pkg().Path()] {
		return
	}
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if obj := w.info().Uses[key]; obj != nil {
			w.handled[key] = true
			w.addUse(obj, arch.ObjectField, typeID(tn), key.Pos())
		}
	}
}

// addUse records a reference to obj, unless obj is declared within the
// walker's own function or type: such a reference is local, not connascence
// between two entities.
func (w *factWalker) addUse(obj types.Object, kind arch.ObjectKind, owner string, pos token.Pos) {
	id := objectID(obj, owner)
	if id == w.fn || (owner != "" && owner == w.typ) || id == w.typ {
		return
	}
	w.uses.add(&arch.Use{
		Object:   id,
		Kind:     kind,
		Package:  obj.Pkg().Path(),
		Exported: obj.Exported(),
		Owner:    owner,
		Count:    1,
		Site:     w.site(pos),
	})
}

// binary records constants compared with == or !=, and pointer comparisons.
func (w *factWalker) binary(b *ast.BinaryExpr) {
	if b.Op != token.EQL && b.Op != token.NEQ {
		return
	}
	x, y := w.info().Types[b.X], w.info().Types[b.Y]
	if (x.Value == nil) != (y.Value == nil) {
		for _, e := range []ast.Expr{b.X, b.Y} {
			w.constExpr(e, arch.ContextCompare, "", 0)
		}
	}
	if ptr, ok := x.Type.(*types.Pointer); ok && !y.IsNil() && !x.IsNil() {
		if named := namedOf(ptr.Elem()); named != nil && named.Obj().Pkg() != nil {
			tn := named.Origin().Obj()
			w.facts.PointerEqs = append(w.facts.PointerEqs, arch.PointerEq{
				Type: typeID(tn), Package: tn.Pkg().Path(), Site: w.site(b.Pos()),
			})
		}
	}
}

// switchStmt records a switch on a named basic type and its constant cases.
func (w *factWalker) switchStmt(s *ast.SwitchStmt) {
	if s.Tag == nil {
		return
	}
	hasDefault := false
	for _, stmt := range s.Body.List {
		cc, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		if cc.List == nil {
			hasDefault = true
		}
		for _, e := range cc.List {
			w.constExpr(e, arch.ContextCase, "", 0)
		}
	}
	named := namedOf(w.info().TypeOf(s.Tag))
	if named == nil || named.Obj().Pkg() == nil {
		return
	}
	if _, basic := named.Underlying().(*types.Basic); !basic {
		return
	}
	tn := named.Origin().Obj()
	w.facts.Switches = append(w.facts.Switches, arch.Switch{
		Type: typeID(tn), Package: tn.Pkg().Path(), HasDefault: hasDefault, Site: w.site(s.Pos()),
	})
}

// constExpr records e when it is constant-valued.
func (w *factWalker) constExpr(e ast.Expr, ctx arch.ConstContext, callee string, arg int) {
	tv := w.info().Types[e]
	if tv.Value == nil || tv.Type == nil {
		return
	}
	w.facts.ConstExprs = append(w.facts.ConstExprs, arch.ConstExpr{
		Type:    types.TypeString(types.Default(tv.Type), nil),
		Value:   tv.Value.ExactString(),
		Literal: isLiteral(e),
		Context: ctx,
		Callee:  callee,
		Arg:     arg,
		Site:    w.site(e.Pos()),
	})
}

// call records constant arguments, conversions through unsafe.Pointer, and
// calls to algorithm, time, lock and reflect functions.
func (w *factWalker) call(c *ast.CallExpr) {
	if tv := w.info().Types[c.Fun]; tv.IsType() && len(c.Args) == 1 {
		w.conversion(tv.Type, c)
		return
	}
	fn, ok := typeutil.Callee(w.info(), c).(*types.Func)
	if !ok || fn.Pkg() == nil {
		return
	}
	name := fn.FullName()
	w.calls.add(&arch.Call{Callee: name, Count: 1, Site: w.site(c.Pos())})
	for i, a := range c.Args {
		w.constExpr(a, arch.ContextArgument, name, i)
	}
	w.stdlibCall(fn, c)
}

// conversion records a conversion between unsafe.Pointer and a pointer to a
// named type.
func (w *factWalker) conversion(to types.Type, c *ast.CallExpr) {
	from := w.info().TypeOf(c.Args[0])
	var target types.Type
	switch {
	case isUnsafePointer(to):
		target = from
	case isUnsafePointer(from):
		target = to
	default:
		return
	}
	ptr, ok := target.(*types.Pointer)
	if !ok {
		return
	}
	if named := namedOf(ptr.Elem()); named != nil && named.Obj().Pkg() != nil {
		tn := named.Origin().Obj()
		w.facts.UnsafeConvs = append(w.facts.UnsafeConvs, arch.UnsafeConv{
			Type: typeID(tn), Package: tn.Pkg().Path(), Site: w.site(c.Pos()),
		})
	}
}

// index records a constant index into a call's result.
func (w *factWalker) index(ix *ast.IndexExpr) {
	tv := w.info().Types[ix.Index]
	if tv.Value == nil {
		return
	}
	var origin string
	switch x := ast.Unparen(ix.X).(type) {
	case *ast.CallExpr:
		if fn, ok := typeutil.Callee(w.info(), x).(*types.Func); ok {
			origin = fn.FullName()
		}
	case *ast.Ident:
		origin = w.origins[w.info().Uses[x]]
	}
	if origin == "" {
		return
	}
	w.facts.ConstIndexes = append(w.facts.ConstIndexes, arch.ConstIndex{
		Origin: origin, Index: tv.Value.ExactString(), Site: w.site(ix.Pos()),
	})
}

// assign marks assigned identifiers as writes, records writes to the
// receiver's fields, and tracks locals assigned from a call.
func (w *factWalker) assign(a *ast.AssignStmt) {
	for _, lhs := range a.Lhs {
		w.markWrite(lhs)
	}
	if len(a.Lhs) != 1 || len(a.Rhs) != 1 {
		return
	}
	id, ok := a.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}
	obj := w.info().ObjectOf(id)
	if obj == nil {
		return
	}
	if _, seen := w.origins[obj]; seen {
		w.origins[obj] = ""
		return
	}
	if call, ok := ast.Unparen(a.Rhs[0]).(*ast.CallExpr); ok {
		if fn, ok := typeutil.Callee(w.info(), call).(*types.Func); ok {
			w.origins[obj] = fn.FullName()
			return
		}
	}
	w.origins[obj] = ""
}

// markWrite marks the identifier e assigns, and a receiver field it writes.
func (w *factWalker) markWrite(e ast.Expr) {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		w.writes[e] = true
	case *ast.SelectorExpr:
		if _, qualified := w.info().Uses[e.Sel].(*types.Var); qualified && w.isPackageName(e.X) {
			w.writes[e.Sel] = true
			return
		}
		if field := receiverField(w.info(), w.recv, e); field != "" {
			w.facts.FieldWrites = append(w.facts.FieldWrites, arch.FieldAccess{
				Type: w.typ, Field: field, Method: w.fn, Site: w.site(e.Pos()),
			})
		}
	}
}

// isPackageName reports an identifier that names an imported package.
func (w *factWalker) isPackageName(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = w.info().Uses[id].(*types.PkgName)
	return ok
}

// linknames records the file's //go:linkname directives that name a target.
func (s *factSink) linknames(fc *fileContext, f *ast.File) {
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			rest, ok := strings.CutPrefix(c.Text, "//go:linkname ")
			if !ok {
				continue
			}
			fields := strings.Fields(rest)
			if len(fields) != 2 {
				continue
			}
			pkg := fields[1]
			if i := strings.LastIndex(pkg, "."); i > strings.LastIndex(pkg, "/") {
				pkg = pkg[:i]
			}
			p := fc.position(c.Pos())
			s.facts.Linknames = append(s.facts.Linknames, arch.Linkname{
				Local:   fields[0],
				Target:  fields[1],
				Package: pkg,
				Site:    arch.Site{File: p.File, Line: p.Line, Package: fc.pkg.PkgPath},
			})
		}
	}
}
