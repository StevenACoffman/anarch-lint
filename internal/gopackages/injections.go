package gopackages

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// addInjections records every call inside node that passes a value of a
// concrete in-scope named type, or a pointer to one, to a parameter of
// interface type: a dependency the callee's package gains at run time without
// importing the concrete type's package (go-arch-lint's deepScan;
// mmi-ref-plan.md §4.4). Calls whose callee cannot be resolved statically,
// such as calls through function values, are skipped.
func (x *extractor) addInjections(fc *fileContext, node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := typeutil.StaticCallee(fc.pkg.TypesInfo, call)
		if callee == nil || callee.Pkg() == nil || !x.inScope[callee.Pkg().Path()] {
			return true
		}
		sig := callee.Signature()
		for i, arg := range call.Args {
			concrete := x.concreteInScope(fc.pkg.TypesInfo.TypeOf(arg))
			if concrete == nil || !types.IsInterface(paramType(sig, i)) {
				continue
			}
			x.prog.Injections = append(x.prog.Injections, arch.Injection{
				Caller:        fc.pkg.PkgPath,
				Callee:        funcID(callee),
				CalleePackage: callee.Pkg().Path(),
				Concrete:      typeID(concrete),
				Pos:           fc.position(arg.Pos()),
			})
		}
		return true
	})
}

// concreteInScope returns the in-scope named type t is, or points to, when it
// is not an interface; otherwise nil.
func (x *extractor) concreteInScope(t types.Type) *types.TypeName {
	if t == nil {
		return nil
	}
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok || types.IsInterface(n) {
		return nil
	}
	obj := n.Origin().Obj()
	if obj.Pkg() == nil || !x.inScope[obj.Pkg().Path()] || obj.Parent() != obj.Pkg().Scope() {
		return nil
	}
	return obj
}

// paramType returns the type of the parameter that argument i binds to,
// following a variadic parameter's element type, or nil past the end.
func paramType(sig *types.Signature, i int) types.Type {
	params := sig.Params()
	switch {
	case params.Len() == 0:
		return nil
	case sig.Variadic() && i >= params.Len()-1:
		if s, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok {
			return s.Elem()
		}
		return nil
	case i < params.Len():
		return params.At(i).Type()
	default:
		return nil
	}
}

// funcID returns a function's ID as arch.Func spells it: the package path,
// then "Recv." for a method, then the name.
func funcID(fn *types.Func) string {
	if owner := typeOwner(fn); owner != nil {
		return fn.Pkg().Path() + "." + owner.Name() + "." + fn.Name()
	}
	return fn.Pkg().Path() + "." + fn.Name()
}
