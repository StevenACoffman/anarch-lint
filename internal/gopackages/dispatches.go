package gopackages

import (
	"cmp"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// dispatchesOf records the run-time wiring of the packages under analysis:
// every dynamic call, through an interface or a function value, that a VTA
// call graph, seeded with CHA, resolves to a concrete method of a type in
// another package under analysis (mmi.md §7.2: the run-time model lives in
// the composition root). VTA over-approximates, but far less than CHA, which
// links an interface call to every implementation in the program, wired or
// not. It is deterministic for one algorithm and one version of x/tools.
//
// SSA is built from syntax for the packages under analysis, and from type
// information alone for their dependencies, which the loader reads from
// export data; their bodies are not needed to find the wiring between the
// packages under analysis.
func dispatchesOf(pkgs []*packages.Package, dir string, inScope map[string]bool) []arch.Dispatch {
	if len(pkgs) == 0 {
		return nil
	}
	prog := ssaProgram(pkgs)
	graph := vta.CallGraph(ssautil.AllFunctions(prog), cha.CallGraph(prog))
	var out []arch.Dispatch
	for fn, node := range graph.Nodes {
		if fn == nil || fn.Pkg == nil || !inScope[fn.Pkg.Pkg.Path()] {
			continue
		}
		for _, edge := range node.Out {
			if d, ok := dispatchOf(prog.Fset, dir, inScope, edge); ok {
				out = append(out, d)
			}
		}
	}
	slices.SortFunc(out, func(a, b arch.Dispatch) int { return compareDispatches(&a, &b) })
	return slices.CompactFunc(out, func(a, b arch.Dispatch) bool {
		return compareDispatches(&a, &b) == 0
	})
}

// ssaProgram builds SSA for the packages from their syntax, and creates
// their dependencies from type information alone.
func ssaProgram(pkgs []*packages.Package) *ssa.Program {
	prog := ssa.NewProgram(pkgs[0].Fset, ssa.InstantiateGenerics)
	created := map[*types.Package]bool{}
	for _, p := range pkgs {
		prog.CreatePackage(p.Types, p.Syntax, p.TypesInfo, true)
		created[p.Types] = true
	}
	var createDeps func(tp *types.Package)
	createDeps = func(tp *types.Package) {
		for _, imp := range tp.Imports() {
			if !created[imp] {
				created[imp] = true
				createDeps(imp)
				prog.CreatePackage(imp, nil, nil, true)
			}
		}
	}
	for _, p := range pkgs {
		createDeps(p.Types)
	}
	prog.Build()
	return prog
}

// dispatchOf returns the dispatch an edge of the call graph is: a dynamic
// call from one package under analysis to a method of a concrete type of
// another; false for any other edge.
func dispatchOf(
	fset *token.FileSet, dir string, inScope map[string]bool, edge *callgraph.Edge,
) (arch.Dispatch, bool) {
	caller, method := dynamicCall(edge)
	if caller == nil || method == nil || !inScope[method.Pkg().Path()] {
		return arch.Dispatch{}, false
	}
	concrete := typeOwner(method)
	callerPkg := caller.Pkg().Path()
	if concrete == nil || callerPkg == method.Pkg().Path() || types.IsInterface(concrete.Type()) {
		return arch.Dispatch{}, false
	}
	p := fset.Position(edge.Site.Pos())
	return arch.Dispatch{
		Caller: funcID(caller), CallerPackage: callerPkg,
		Callee: funcID(method), Concrete: typeID(concrete),
		Pos: arch.Position{File: relPath(dir, p.Filename), Line: p.Line},
	}, true
}

// dynamicCall returns the declared caller and the method of an edge whose
// call is dynamic, through an interface or a function value; nil for a
// static call or a callee that is no declared method.
func dynamicCall(edge *callgraph.Edge) (caller, method *types.Func) {
	site, callee := edge.Site, edge.Callee.Func
	if site == nil || site.Common().StaticCallee() != nil || callee == nil || callee.Pkg == nil {
		return nil, nil
	}
	method, _ = callee.Object().(*types.Func)
	return declaringFunc(edge.Caller.Func), method
}

// declaringFunc returns the declared function an SSA function belongs to:
// itself, or the function a closure is written in.
func declaringFunc(fn *ssa.Function) *types.Func {
	for fn != nil {
		if obj, ok := fn.Object().(*types.Func); ok {
			return obj
		}
		fn = fn.Parent()
	}
	return nil
}

// compareDispatches orders dispatches by caller, callee and position.
func compareDispatches(a, b *arch.Dispatch) int {
	return cmp.Or(
		cmp.Compare(a.Caller, b.Caller),
		cmp.Compare(a.Callee, b.Callee),
		arch.ComparePositions(a.Pos, b.Pos),
	)
}
