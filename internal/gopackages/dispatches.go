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

// callGraph is a VTA call graph, seeded with CHA, and the SSA program it is
// built from.
type callGraph struct {
	prog  *ssa.Program
	graph *callgraph.Graph
}

// dynamicCalls collects what dynamicCallsOf returns. A generic function's
// call sites recur in each instance, so calls are keyed by position: a call
// counts once, and is unresolved only when no instance resolves it.
type dynamicCalls struct {
	cg       *callGraph
	dir      string
	calls    callTable
	seen     map[dynamicCallee]bool
	sites    map[token.Pos]arch.Site
	resolves map[token.Pos]bool
}

// dynamicCallee is one callee of the dynamic call at a position.
type dynamicCallee struct {
	pos    token.Pos
	callee string
}

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
func dispatchesOf(cg *callGraph, dir string, inScope map[string]bool) []arch.Dispatch {
	prog, graph := cg.prog, cg.graph
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

// callGraphOf builds the call graph of the packages, which
// dispatchesOf and dynamicCallsOf both read, so the whole-program build is
// paid once; nil without packages.
func callGraphOf(pkgs []*packages.Package) *callGraph {
	if len(pkgs) == 0 {
		return nil
	}
	prog := ssaProgram(pkgs)
	return &callGraph{
		prog:  prog,
		graph: vta.CallGraph(ssautil.AllFunctions(prog), cha.CallGraph(prog)),
	}
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

// dynamicCallsOf returns the callees the call graph resolves each dynamic
// call of a function under analysis to, one per caller and callee with a
// count and its first site, and the site of each dynamic call it resolves
// to nothing, sorted. A callee is any declared function, in the analysis or
// not, so a calls rule can name (*os.File).Write; one that is no declared
// function, such as a closure, is in the analysis, and its own calls are
// facts already.
func dynamicCallsOf(cg *callGraph, dir string, inScope map[string]bool) ([]arch.Call, []arch.Site) {
	d := dynamicCalls{
		cg: cg, dir: dir, calls: callTable{}, seen: map[dynamicCallee]bool{},
		sites: map[token.Pos]arch.Site{}, resolves: map[token.Pos]bool{},
	}
	for fn, node := range cg.graph.Nodes {
		if fn != nil && fn.Pkg != nil && inScope[fn.Pkg.Pkg.Path()] {
			d.function(fn, node)
		}
	}
	return d.results()
}

// function records fn's dynamic calls and the edges the graph gives them.
func (d *dynamicCalls) function(fn *ssa.Function, node *callgraph.Node) {
	for _, site := range dynamicSites(fn) {
		d.sites[site.Pos()] = callSite(d.cg.prog.Fset, d.dir, fn, site.Pos())
	}
	for _, e := range node.Out {
		if e.Site == nil || !isDynamic(e.Site) {
			continue
		}
		d.resolves[e.Site.Pos()] = true
		callee, ok := e.Callee.Func.Object().(*types.Func)
		if !ok || callee.Pkg() == nil {
			continue
		}
		key := dynamicCallee{e.Site.Pos(), calleeName(callee)}
		if !d.seen[key] {
			d.seen[key] = true
			d.calls.add(&arch.Call{
				Callee: key.callee, Count: 1, Site: callSite(d.cg.prog.Fset, d.dir, fn, key.pos),
			})
		}
	}
}

// results returns the calls and the unresolved sites, each sorted.
func (d *dynamicCalls) results() ([]arch.Call, []arch.Site) {
	out := make([]arch.Call, 0, len(d.calls))
	for _, c := range d.calls {
		out = append(out, *c)
	}
	slices.SortFunc(out, func(a, b arch.Call) int {
		return cmp.Or(arch.CompareSites(a.Site, b.Site), cmp.Compare(a.Callee, b.Callee))
	})
	var unresolved []arch.Site
	for pos, site := range d.sites {
		if !d.resolves[pos] {
			unresolved = append(unresolved, site)
		}
	}
	slices.SortFunc(unresolved, arch.CompareSites)
	return out, slices.Compact(unresolved)
}

// dynamicSites returns fn's calls through an interface or a function value.
func dynamicSites(fn *ssa.Function) []ssa.CallInstruction {
	var out []ssa.CallInstruction
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if site, ok := instr.(ssa.CallInstruction); ok && isDynamic(site) {
				out = append(out, site)
			}
		}
	}
	return out
}

// isDynamic reports a call through an interface or a function value: one
// with no static callee that is not a builtin, such as append or len, which
// SSA also calls without one.
func isDynamic(site ssa.CallInstruction) bool {
	common := site.Common()
	_, builtin := common.Value.(*ssa.Builtin)
	return common.StaticCallee() == nil && !builtin && site.Pos().IsValid()
}

// callSite is the site of a call in fn, attributed as the facts attribute
// one: to the declared function fn is or is written in, and its receiver.
func callSite(fset *token.FileSet, dir string, fn *ssa.Function, pos token.Pos) arch.Site {
	p := fset.Position(pos)
	site := arch.Site{File: relPath(dir, p.Filename), Line: p.Line, Package: fn.Pkg.Pkg.Path()}
	if decl := declaringFunc(fn); decl != nil {
		site.Func = funcID(decl)
		if owner := typeOwner(decl); owner != nil {
			site.Type = typeID(owner)
		}
	}
	return site
}
