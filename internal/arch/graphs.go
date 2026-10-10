package arch

// Graphs are the three dependency graphs of a program under a model
// (mmi.md §3): types, packages, and subsystems.
type Graphs struct {
	// Types has a node per package-level type and an edge per type pair that
	// one refers to the other, weighted by the referring identifiers.
	Types Graph
	// Packages has a node per in-scope package and an edge per import
	// between them.
	Packages Graph
	// Subsystems aggregates Packages onto subsystems. An edge's weight is the
	// number of package imports it stands for. A package no subsystem claims
	// is a subsystem of its own, so it still appears in cycles (mmi.md §5).
	Subsystems Graph
	// PackageOf maps each type to its package.
	PackageOf map[string]string
	// SubsystemOf maps each package to its subsystem, or to itself.
	SubsystemOf map[string]string
	// SubsystemAssignment and LayerAssignment record how the model's patterns
	// matched, including unassigned packages and stale patterns.
	SubsystemAssignment Assignment
	LayerAssignment     Assignment
}

// BuildGraphs builds the three graphs of p under m. Neither is modified.
//
// Requires: p is sorted, as [Program] documents.
func BuildGraphs(p *Program, m *Model) Graphs {
	pkgs := packagePaths(p)
	g := Graphs{
		PackageOf:           map[string]string{},
		SubsystemOf:         map[string]string{},
		SubsystemAssignment: assign(m.subsystemGroups(), pkgs),
		LayerAssignment:     assign(m.layerGroups(), pkgs),
	}

	typeIDs := make([]string, len(p.Types))
	for i := range p.Types {
		typeIDs[i] = p.Types[i].ID
		g.PackageOf[p.Types[i].ID] = p.Types[i].Package
	}
	var typeEdges []Edge
	for _, r := range p.Refs {
		typeEdges = append(typeEdges, Edge{From: r.From, To: r.To, Weight: r.Weight})
	}
	g.Types = NewGraph(typeIDs, typeEdges)

	inScope := make(map[string]bool, len(pkgs))
	for _, path := range pkgs {
		inScope[path] = true
		g.SubsystemOf[path] = containerOf(g.SubsystemAssignment.Of, path)
	}
	var pkgEdges, subEdges []Edge
	for _, imp := range p.Imports {
		if !inScope[imp.To] {
			continue
		}
		pkgEdges = append(pkgEdges, Edge{From: imp.From, To: imp.To, Weight: 1})
		subEdges = append(subEdges, Edge{
			From:   g.SubsystemOf[imp.From],
			To:     g.SubsystemOf[imp.To],
			Weight: 1,
		})
	}
	g.Packages = NewGraph(pkgs, pkgEdges)
	subsystems := make([]string, 0, len(pkgs))
	for _, path := range pkgs {
		subsystems = append(subsystems, g.SubsystemOf[path])
	}
	g.Subsystems = NewGraph(subsystems, subEdges)
	return g
}

// Ordering returns the ordering measures of the three graphs.
func (g *Graphs) Ordering() Ordering {
	return NewOrdering(g.Types, g.Packages, g.Subsystems, g.PackageOf, g.SubsystemOf)
}

// packagePaths returns the paths of p's packages, in p's sorted order.
func packagePaths(p *Program) []string {
	paths := make([]string, len(p.Packages))
	for i, pkg := range p.Packages {
		paths[i] = pkg.Path
	}
	return paths
}
