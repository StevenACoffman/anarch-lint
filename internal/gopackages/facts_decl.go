package gopackages

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// minHashedStatements is the smallest body whose shape is hashed for clone
// detection: shorter bodies coincide too often to mean anything.
const minHashedStatements = 10

// funcFacts records a function's signature, its nil, nil returns, its error
// guards, a guard on a receiver field, and the facts of its body; it sets
// fn.BodyHash.
func (s *factSink) funcFacts(fc *fileContext, d *ast.FuncDecl, fn *arch.Func) {
	w := s.walker(fc, fn.ID, fn.Receiver)
	if d.Recv != nil && len(d.Recv.List) == 1 && len(d.Recv.List[0].Names) == 1 {
		w.recv = fc.pkg.TypesInfo.Defs[d.Recv.List[0].Names[0]]
	}
	obj, ok := fc.pkg.TypesInfo.Defs[d.Name].(*types.Func)
	if !ok {
		return
	}
	s.facts.Signatures = append(s.facts.Signatures, signatureOf(obj, w.site(d.Pos())))
	if d.Body == nil {
		return
	}
	s.earlyReturns(w, obj.Signature(), d.Body)
	if fn.Statements >= minHashedStatements {
		fn.BodyHash, fn.ExactHash = bodyHash(d.Body), exactHash(fc.fset, d.Body)
	}
	w.walk(d)
}

// earlyReturns records how a body returns early: its nil, nil returns, its
// error guards, and a first statement that guards on a receiver field.
func (s *factSink) earlyReturns(w *factWalker, sig *types.Signature, body *ast.BlockStmt) {
	info := w.info()
	if returnsNillableAndError(sig) {
		for _, pos := range nilNilReturns(info, body) {
			s.facts.NilNils = append(s.facts.NilNils, arch.NilNil{Site: w.site(pos)})
		}
	}
	for _, pos := range errorGuards(info, body) {
		s.facts.ErrorGuards = append(s.facts.ErrorGuards, arch.ErrorGuard{Site: w.site(pos)})
	}
	if field, pos := guardField(info, w.recv, body); field != "" {
		s.facts.Guards = append(s.facts.Guards, arch.FieldAccess{
			Type: w.typ, Field: field, Method: w.fn, Site: w.site(pos),
		})
	}
}

// typeFacts records the facts of a type declaration, and the signature of
// each method an interface type declares, so its contract can be judged by
// the types it mentions (connascence.md §9.2).
func (s *factSink) typeFacts(fc *fileContext, ts *ast.TypeSpec, id string) {
	w := s.walker(fc, "", id)
	w.walk(ts.Type)
	tn, ok := fc.pkg.TypesInfo.Defs[ts.Name].(*types.TypeName)
	if !ok {
		return
	}
	iface, ok := tn.Type().Underlying().(*types.Interface)
	if !ok {
		return
	}
	for i := range iface.NumExplicitMethods() {
		m := iface.ExplicitMethod(i)
		sig := signatureOf(m, w.site(m.Pos()))
		sig.Func = id + "." + m.Name()
		s.facts.Signatures = append(s.facts.Signatures, sig)
	}
}

// valueFacts records a package-level var or const declaration: its
// constants, and the uses in its initializers.
func (s *factSink) valueFacts(fc *fileContext, d *ast.GenDecl) {
	w := s.walker(fc, "", "")
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		if d.Tok == token.CONST {
			s.constants(fc, vs, w)
		}
		for _, v := range vs.Values {
			w.walk(v)
		}
		if vs.Type != nil {
			w.walk(vs.Type)
		}
	}
}

// constants records the constants one spec declares.
func (s *factSink) constants(fc *fileContext, vs *ast.ValueSpec, w *factWalker) {
	for i, name := range vs.Names {
		c, ok := fc.pkg.TypesInfo.Defs[name].(*types.Const)
		if !ok || name.Name == "_" {
			continue
		}
		alias := false
		if i < len(vs.Values) {
			alias = namesConst(fc.pkg.TypesInfo, vs.Values[i])
		}
		s.facts.Constants = append(s.facts.Constants, arch.Constant{
			ID:       objectID(c, ""),
			Type:     types.TypeString(c.Type(), nil),
			Value:    c.Val().ExactString(),
			Exported: c.Exported(),
			Alias:    alias,
			Site:     w.site(name.Pos()),
		})
	}
}

// stdlibCall records calls to the standard library's algorithm, time, lock
// and reflect functions.
func (w *factWalker) stdlibCall(fn *types.Func, c *ast.CallExpr) {
	path := fn.Pkg().Path()
	if strings.Contains(strings.Split(path, "/")[0], ".") {
		return
	}
	site := w.site(c.Pos())
	facts := w.facts
	switch path {
	case "time":
		if fn.Signature().Recv() == nil {
			facts.TimeCalls = append(
				facts.TimeCalls,
				arch.TimeCall{Callee: calleeName(fn), Site: site},
			)
		}
	case "sync":
		w.lockCall(fn, c, site)
	case "reflect":
		w.reflectCall(fn, c, site)
	default:
		if algo := algorithmOf(path, fn.Signature().Recv()); algo != "" {
			facts.AlgoCalls = append(facts.AlgoCalls, arch.AlgoCall{Algorithm: algo, Site: site})
		}
	}
}

// lockCall records a mutex method call with its receiver expression.
func (w *factWalker) lockCall(fn *types.Func, c *ast.CallExpr, site arch.Site) {
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok || !isLockMethod(fn.Name()) {
		return
	}
	w.facts.LockCalls = append(w.facts.LockCalls, arch.LockCall{
		Expr:     types.ExprString(sel.X),
		Method:   fn.Name(),
		Deferred: w.deferred[c],
		Site:     site,
	})
}

// reflectCall records a reflect lookup by name, with the name when constant.
func (w *factWalker) reflectCall(fn *types.Func, c *ast.CallExpr, site arch.Site) {
	if fn.Name() != "FieldByName" && fn.Name() != "MethodByName" {
		return
	}
	name := ""
	if len(c.Args) == 1 {
		name, _ = stringConst(w.info(), c.Args[0])
	}
	rb := arch.ReflectByName{Method: calleeName(fn), Name: name, Site: site}
	if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok {
		rb.Type = reflectedType(w.info(), sel.X)
	}
	w.facts.ReflectByNames = append(w.facts.ReflectByNames, rb)
}

// reflectedType returns the ID of the package-level type a reflect value or
// type was made from, as in reflect.ValueOf(x).Elem().FieldByName: the
// static type of x, through Elem and reflect.Indirect; "" when the chain
// does not start at reflect.ValueOf or reflect.TypeOf of a value of a named
// type.
func reflectedType(info *types.Info, e ast.Expr) string {
	for e != nil {
		var typ string
		e, typ = reflectStep(info, e)
		if typ != "" {
			return typ
		}
	}
	return ""
}

// reflectStep takes one step back along a chain of reflect calls: it
// returns the expression an Elem or reflect.Indirect call reflects, or the
// type a reflect.ValueOf or reflect.TypeOf call starts from; nil and "" for
// anything else.
func reflectStep(info *types.Info, e ast.Expr) (ast.Expr, string) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil, ""
	}
	fn, _ := typeutil.Callee(info, call).(*types.Func)
	if fn == nil {
		return nil, ""
	}
	switch name := calleeName(fn); {
	case elemOf(fn, call) != nil:
		return elemOf(fn, call), ""
	case name == "reflect.Indirect" && len(call.Args) == 1:
		return call.Args[0], ""
	case (name == "reflect.ValueOf" || name == "reflect.TypeOf") && len(call.Args) == 1:
		return nil, namedTypeID(info, call.Args[0])
	}
	return nil, ""
}

// elemOf returns what a call of an Elem method with no arguments is called
// on, such as v in v.Elem(); nil for any other call.
func elemOf(fn *types.Func, call *ast.CallExpr) ast.Expr {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || fn.Name() != "Elem" || len(call.Args) != 0 {
		return nil
	}
	return sel.X
}

// namedTypeID returns the ID of the package-level named type an expression
// has, or points to; "" for any other type.
func namedTypeID(info *types.Info, e ast.Expr) string {
	if named := namedOf(info.TypeOf(e)); named != nil {
		if tn := typeOwner(named.Origin().Obj()); tn != nil {
			return typeID(tn)
		}
	}
	return ""
}

// sortFacts returns the facts with every slice sorted by site, then by the rest of the fact, so
// that equal facts are the only ones whose order is arbitrary.
func (s *factSink) sortFacts() arch.Facts {
	f := &s.facts
	for _, u := range s.uses {
		f.Uses = append(f.Uses, *u)
	}
	for _, c := range s.calls {
		f.Calls = append(f.Calls, *c)
	}
	sortBySite(f.Calls, func(c arch.Call) arch.Site { return c.Site })
	sortBySite(f.Uses, func(u arch.Use) arch.Site { return u.Site })
	sortBySite(f.Signatures, func(s arch.Signature) arch.Site { return s.Site })
	sortBySite(f.ConstExprs, func(c arch.ConstExpr) arch.Site { return c.Site })
	sortBySite(f.Constants, func(c arch.Constant) arch.Site { return c.Site })
	sortBySite(f.NilNils, func(n arch.NilNil) arch.Site { return n.Site })
	sortBySite(f.UnkeyedLits, func(u arch.UnkeyedLit) arch.Site { return u.Site })
	sortBySite(f.ConstIndexes, func(c arch.ConstIndex) arch.Site { return c.Site })
	sortBySite(f.Switches, func(s arch.Switch) arch.Site { return s.Site })
	sortBySite(f.GlobalUses, func(g arch.GlobalUse) arch.Site { return g.Site })
	sortBySite(f.PointerEqs, func(p arch.PointerEq) arch.Site { return p.Site })
	sortBySite(f.AlgoCalls, func(a arch.AlgoCall) arch.Site { return a.Site })
	sortBySite(f.TimeCalls, func(t arch.TimeCall) arch.Site { return t.Site })
	sortBySite(f.Guards, func(g arch.FieldAccess) arch.Site { return g.Site })
	sortBySite(f.ErrorGuards, func(g arch.ErrorGuard) arch.Site { return g.Site })
	sortBySite(f.FieldWrites, func(g arch.FieldAccess) arch.Site { return g.Site })
	sortBySite(f.LockCalls, func(l arch.LockCall) arch.Site { return l.Site })
	sortBySite(f.Linknames, func(l arch.Linkname) arch.Site { return l.Site })
	sortBySite(f.UnsafeConvs, func(u arch.UnsafeConv) arch.Site { return u.Site })
	sortBySite(f.ReflectByNames, func(r arch.ReflectByName) arch.Site { return r.Site })
	return *f
}
