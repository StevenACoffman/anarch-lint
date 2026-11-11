package arch

import "slices"

// withDispatch returns a copy of the program whose references and imports
// include its run-time wiring: for each dispatch, a reference from the
// caller's receiver type to the concrete type, when the caller is a method,
// and an import from the caller's package to the concrete type's. Nothing
// else of the copy is meant to be read: it exists so the coupling measures
// can be taken again over the same nodes with the dispatch edges added
// (IMPROVEMENTS.md §10.2).
func withDispatch(p *Program) *Program {
	receiver := map[string]string{}
	for i := range p.Funcs {
		receiver[p.Funcs[i].ID] = p.Funcs[i].Receiver
	}
	packageOf := map[string]string{}
	for i := range p.Types {
		packageOf[p.Types[i].ID] = p.Types[i].Package
	}
	out := *p
	out.Refs, out.Imports = slices.Clone(p.Refs), slices.Clone(p.Imports)
	for _, d := range p.Dispatches {
		if recv := receiver[d.Caller]; recv != "" && recv != d.Concrete {
			out.Refs = append(out.Refs, Ref{From: recv, To: d.Concrete, Weight: 1})
		}
		if to := packageOf[d.Concrete]; to != "" && to != d.CallerPackage {
			out.Imports = append(out.Imports, Import{From: d.CallerPackage, To: to, Pos: d.Pos})
		}
	}
	return &out
}

// dispatchCoupling returns the coupling measures with the program's dispatch
// edges added, nil when it carries none. Adding edges never lowers CCD, ACD,
// propagation cost, relative cyclicity or the largest cycle group, so each
// static value and its value here bound an interval; SDI does only when both
// cuts are exact, since a greedy cut is not monotone.
func dispatchCoupling(p *Program, m *Model) *CouplingLevels {
	if len(p.Dispatches) == 0 {
		return nil
	}
	g := BuildGraphs(withDispatch(p), m)
	return &CouplingLevels{
		Types:      CouplingOf(g.Types),
		Packages:   CouplingOf(g.Packages),
		Subsystems: CouplingOf(g.Subsystems),
	}
}
