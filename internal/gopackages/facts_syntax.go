package gopackages

// Pure functions over syntax and types that the fact walker and the fact
// sink share: they read the AST and go/types and return values, recording
// nothing.

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/constant"
	"go/format"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// objectID returns the ID of a declared object: the package path and name,
// with the owning type in between for a method or field.
func objectID(obj types.Object, owner string) string {
	if owner != "" {
		return owner + "." + obj.Name()
	}
	return obj.Pkg().Path() + "." + obj.Name()
}

// fieldOwner returns the named struct type that declares the field reached
// from recv through the embedding path index.
func fieldOwner(recv types.Type, index []int) *types.TypeName {
	var owner *types.TypeName
	t := recv
	for _, i := range index {
		named := namedOf(t)
		if named != nil {
			owner = named.Origin().Obj()
		}
		st, ok := deref(t).Underlying().(*types.Struct)
		if !ok {
			return nil
		}
		t = st.Field(i).Type()
	}
	if owner == nil || owner.Pkg() == nil {
		return nil
	}
	return owner
}

// namedOf returns the named type t is, or points to; nil otherwise.
func namedOf(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	named, _ := types.Unalias(deref(t)).(*types.Named)
	return named
}

func deref(t types.Type) types.Type {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

// isLiteral reports a basic literal, possibly negated or parenthesized.
func isLiteral(e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.BasicLit:
		return true
	case *ast.UnaryExpr:
		return isLiteral(e.X)
	default:
		return false
	}
}

func isUnsafePointer(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.UnsafePointer
}

// stringConst returns the value of a constant string expression.
func stringConst(info *types.Info, e ast.Expr) (string, bool) {
	tv := info.Types[e]
	if tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// receiverField returns the field sel selects directly on the receiver
// variable recv, or "".
func receiverField(info *types.Info, recv types.Object, sel *ast.SelectorExpr) string {
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok || recv == nil || info.Uses[id] != recv {
		return ""
	}
	if s, ok := info.Selections[sel]; ok && s.Kind() == types.FieldVal {
		return sel.Sel.Name
	}
	return ""
}

// namesConst reports an expression that is just the name of a constant.
func namesConst(info *types.Info, e ast.Expr) bool {
	var id *ast.Ident
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	default:
		return false
	}
	_, ok := info.Uses[id].(*types.Const)
	return ok
}

// signatureOf returns a function's signature fact.
func signatureOf(fn *types.Func, site arch.Site) arch.Signature {
	sig := fn.Signature()
	refs := func(t *types.Tuple) []arch.TypeRef {
		out := make([]arch.TypeRef, t.Len())
		for i := range t.Len() {
			out[i] = typeRefOf(t.At(i).Type())
		}
		return out
	}
	return arch.Signature{
		Func:     funcID(fn),
		Exported: fn.Exported(),
		Params:   refs(sig.Params()),
		Results:  refs(sig.Results()),
		Variadic: sig.Variadic(),
		Site:     site,
	}
}

// typeRefOf describes a type in a signature.
func typeRefOf(t types.Type) arch.TypeRef {
	ref := arch.TypeRef{Type: types.TypeString(t, nil)}
	if b, ok := types.Unalias(t).(*types.Basic); ok {
		ref.Basic = b.Kind() != types.UnsafePointer
		return ref
	}
	core := t
	for {
		switch u := types.Unalias(core).(type) {
		case *types.Pointer:
			core = u.Elem()
			continue
		case *types.Slice:
			core = u.Elem()
			continue
		case *types.Array:
			core = u.Elem()
			continue
		case *types.Map:
			core = u.Elem()
			continue
		case *types.Named:
			if tn := u.Origin().Obj(); tn.Pkg() != nil {
				ref.Named, ref.Package = typeID(tn), tn.Pkg().Path()
			}
		}
		return ref
	}
}

// returnsNillableAndError reports results of a nillable value and an error.
func returnsNillableAndError(sig *types.Signature) bool {
	r := sig.Results()
	if r.Len() != 2 || !types.Identical(r.At(1).Type(), types.Universe.Lookup("error").Type()) {
		return false
	}
	switch r.At(0).Type().Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Interface, *types.Chan, *types.Signature:
		return true
	default:
		return false
	}
}

// nilNilReturns returns every return of nil, nil in body, outside function
// literals.
func nilNilReturns(info *types.Info, body *ast.BlockStmt) []token.Pos {
	var out []token.Pos
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(n.Results) == 2 && info.Types[n.Results[0]].IsNil() &&
				info.Types[n.Results[1]].IsNil() {
				out = append(out, n.Pos())
			}
		}
		return true
	})
	return out
}

// errorGuards returns the position of each statement of body, at its top
// level, that is an error guard (arch.ErrorGuard): an if with no init
// statement whose body ends in a return whose last result is an error and
// not nil, or in a call of the builtin panic. A guard nested deeper, or a
// switch, is not one: validation comes first in a body, and the shape is
// kept narrow so what it finds is worth a person's look.
func errorGuards(info *types.Info, body *ast.BlockStmt) []token.Pos {
	var out []token.Pos
	for _, stmt := range body.List {
		ifs, ok := stmt.(*ast.IfStmt)
		if !ok || ifs.Init != nil || len(ifs.Body.List) == 0 {
			continue
		}
		if refuses(info, ifs.Body.List[len(ifs.Body.List)-1]) {
			out = append(out, ifs.Pos())
		}
	}
	return out
}

// refuses reports a statement that ends a guard by refusing: a return whose
// last result is a non-nil error, or a call of the builtin panic.
func refuses(info *types.Info, stmt ast.Stmt) bool {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		if len(s.Results) == 0 {
			return false
		}
		last := info.Types[s.Results[len(s.Results)-1]]
		errType := types.Universe.Lookup("error").Type()
		return !last.IsNil() && last.Type != nil &&
			types.Implements(last.Type, errType.Underlying().(*types.Interface))
	case *ast.ExprStmt:
		call, ok := s.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		id, ok := ast.Unparen(call.Fun).(*ast.Ident)
		if !ok {
			return false
		}
		b, ok := info.Uses[id].(*types.Builtin)
		return ok && b.Name() == "panic"
	}
	return false
}

// guardField returns the receiver field a method's first statement tests
// before returning early, the signature of a required call order, and the
// position of that statement; "" when the method has no such guard.
func guardField(info *types.Info, recv types.Object, body *ast.BlockStmt) (string, token.Pos) {
	if recv == nil || len(body.List) == 0 {
		return "", token.NoPos
	}
	ifs, ok := body.List[0].(*ast.IfStmt)
	if !ok || len(ifs.Body.List) == 0 {
		return "", token.NoPos
	}
	if _, ok := ifs.Body.List[len(ifs.Body.List)-1].(*ast.ReturnStmt); !ok {
		return "", token.NoPos
	}
	field := ""
	ast.Inspect(ifs.Cond, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && field == "" {
			field = receiverField(info, recv, sel)
		}
		return field == ""
	})
	return field, ifs.Pos()
}

func isLockMethod(name string) bool {
	switch name {
	case "Lock", "Unlock", "RLock", "RUnlock":
		return true
	default:
		return false
	}
}

// algorithmOf names the algorithm a call into a hash, crypto or encoding
// package uses: the package, plus a concrete method's receiver type. Calls
// through an interface, such as hash.Hash's Write, name no algorithm.
func algorithmOf(path string, recv *types.Var) string {
	if !strings.HasPrefix(path, "crypto/") && !strings.HasPrefix(path, "hash/") &&
		!strings.HasPrefix(path, "encoding/") {
		return ""
	}
	if recv == nil {
		return path
	}
	named := namedOf(recv.Type())
	if named == nil || types.IsInterface(named) {
		return ""
	}
	return path + "." + named.Obj().Name()
}

// bodyHash hashes the shape of a body: its node types and operators, with
// identifiers and literal values left out.
func bodyHash(body *ast.BlockStmt) string {
	var b strings.Builder
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			b.WriteString(")")
			return true
		}
		fmt.Fprintf(&b, "(%T", n)
		switch n := n.(type) {
		case *ast.BinaryExpr:
			b.WriteString(n.Op.String())
		case *ast.UnaryExpr:
			b.WriteString(n.Op.String())
		case *ast.AssignStmt:
			b.WriteString(n.Tok.String())
		case *ast.IncDecStmt:
			b.WriteString(n.Tok.String())
		case *ast.BasicLit:
			b.WriteString(n.Kind.String())
		case *ast.BranchStmt:
			b.WriteString(n.Tok.String())
		}
		return true
	})
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:16])
}

// exactHash hashes a body as go/format prints it. A block statement prints
// without its comments, so a comment does not tell two copies apart
// (docs/adr/0013); blank lines, which the printer keeps from the source, are
// dropped for the same reason.
func exactHash(fset *token.FileSet, body *ast.BlockStmt) string {
	var b strings.Builder
	if err := format.Node(&b, fset, body); err != nil {
		return "" // a body that does not print has no copy to find
	}
	var lines []string
	for line := range strings.Lines(b.String()) {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "")))
	return hex.EncodeToString(sum[:16])
}

// sortBySite sorts facts by site and then by their printed form, a total
// order over every field.
func sortBySite[T any](xs []T, site func(T) arch.Site) {
	slices.SortFunc(xs, func(a, b T) int {
		return cmp.Or(
			arch.CompareSites(site(a), site(b)),
			cmp.Compare(fmt.Sprintf("%+v", a), fmt.Sprintf("%+v", b)),
		)
	})
}
