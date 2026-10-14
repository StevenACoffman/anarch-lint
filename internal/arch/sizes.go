package arch

import (
	"cmp"
	"slices"
)

// Sizes holds the size and complexity aggregates behind MMI criteria 1.1.3,
// 1.1.4 and 1.3.1–1.3.4 (mmi.md §7.2).
type Sizes struct {
	// Lines is the lines of code of every analyzed package.
	Lines int `json:"lines"`
	Types int `json:"types"`
	Funcs int `json:"funcs"`
	// Generated counts the generated files left out of every measure, and
	// BuildExcluded those the build constraints leave out.
	Generated     int `json:"generated"`
	BuildExcluded int `json:"buildExcluded"`
	// LinesInLargeTypes sums the lines of types above largeTypeLines.
	LinesInLargeTypes int `json:"linesInLargeTypes"`
	// LinesInLargeFuncs sums the lines of functions above largeFuncLines.
	LinesInLargeFuncs int `json:"linesInLargeFuncs"`
	// TypesInLargePackages counts types in packages with more than
	// largePackageTypes types.
	TypesInLargePackages int `json:"typesInLargePackages"`
	// ComplexFuncs counts functions above highComplexity in the chosen variant.
	ComplexFuncs int `json:"complexFuncs"`
	// Subsystems and Layers give the lines of code of each group, sorted by
	// name, for the size-relationship criteria.
	Subsystems []GroupSize `json:"subsystems,omitempty"`
	Layers     []GroupSize `json:"layers,omitempty"`
}

// GroupSize is the lines of code of one subsystem or layer.
type GroupSize struct {
	Name  string `json:"name"`
	Lines int    `json:"lines"`
}

// Cohesion is the LCOM4 of one struct type with two or more methods: the
// number of connected components in the graph of its methods and the fields
// they use, where a method is linked to each field and method it selects
// through its receiver (mmi.md §7.5). 1 is cohesive; more suggests a split.
type Cohesion struct {
	Type  string `json:"type"`
	LCOM4 int    `json:"lcom4"`
}

// sizesOf computes the size aggregates of p under the limits t.
func sizesOf(p *Program, g *Graphs, t *Thresholds) Sizes {
	z := Sizes{Types: len(p.Types), Funcs: len(p.Funcs)}
	z.LinesInLargeTypes, z.TypesInLargePackages = typeSizes(p, t)
	z.LinesInLargeFuncs, z.ComplexFuncs = funcSizes(p, t)
	for _, f := range p.Excluded {
		switch f.Reason {
		case ExcludedGenerated:
			z.Generated++
		case ExcludedByBuild:
			z.BuildExcluded++
		}
	}
	subLines, layerLines := map[string]int{}, map[string]int{}
	for _, pkg := range p.Packages {
		z.Lines += pkg.Lines
		if sub, ok := g.SubsystemAssignment.Of[pkg.Path]; ok {
			subLines[sub] += pkg.Lines
		}
		if layer, ok := g.LayerAssignment.Of[pkg.Path]; ok {
			layerLines[layer] += pkg.Lines
		}
	}
	z.Subsystems, z.Layers = groupSizes(subLines), groupSizes(layerLines)
	return z
}

// typeSizes returns the lines of code in large types and the number of types
// in large packages (criteria 1.3.1 and 1.3.3).
func typeSizes(p *Program, t *Thresholds) (linesInLarge, inLargePackages int) {
	perPkg := map[string]int{}
	for i := range p.Types {
		perPkg[p.Types[i].Package]++
		if p.Types[i].Lines > t.LargeTypeLines {
			linesInLarge += p.Types[i].Lines
		}
	}
	for i := range p.Types {
		if perPkg[p.Types[i].Package] > t.LargePackageTypes {
			inLargePackages++
		}
	}
	return linesInLarge, inLargePackages
}

// funcSizes returns the lines of code in large functions and the number of
// functions above the complexity limit (criteria 1.3.2 and 1.3.4).
func funcSizes(p *Program, t *Thresholds) (linesInLarge, overComplexity int) {
	for i := range p.Funcs {
		if p.Funcs[i].Lines > t.LargeFuncLines {
			linesInLarge += p.Funcs[i].Lines
		}
		if p.Funcs[i].Complexity.Of(t.ComplexityVariant) > t.HighComplexity {
			overComplexity++
		}
	}
	return linesInLarge, overComplexity
}

// CohesionOf returns the LCOM4 of every struct type with two or more methods,
// sorted by type.
func CohesionOf(p *Program) []Cohesion {
	methods := map[string][]*Func{}
	for i := range p.Funcs {
		if fn := &p.Funcs[i]; fn.Receiver != "" {
			methods[fn.Receiver] = append(methods[fn.Receiver], fn)
		}
	}
	out := make([]Cohesion, 0)
	for i := range p.Types {
		t := &p.Types[i]
		if t.Kind != KindStruct || len(methods[t.ID]) < 2 {
			continue
		}
		out = append(out, Cohesion{Type: t.ID, LCOM4: lcom4(t, methods[t.ID])})
	}
	slices.SortFunc(out, func(a, b Cohesion) int { return cmp.Compare(a.Type, b.Type) })
	return out
}

// lcom4 counts the connected components among a type's methods and the
// fields its methods use. A field no method uses is left out, so it doesn't
// count as a component of its own.
func lcom4(t *Type, methods []*Func) int {
	fields := map[string]bool{}
	for _, f := range t.Fields {
		fields[f] = true
	}
	var nodes []string
	var edges []Edge
	for _, m := range methods {
		node := "m:" + m.Name
		nodes = append(nodes, node)
		for _, use := range m.Uses {
			switch {
			case fields[use]:
				edges = append(edges, Edge{From: node, To: "f:" + use})
			default:
				edges = append(edges, Edge{From: node, To: "m:" + use})
			}
		}
	}
	// Components of the undirected graph: add each edge both ways and count
	// the strongly connected components.
	undirected := make([]Edge, 0, 2*len(edges))
	for _, e := range edges {
		undirected = append(undirected, e, Edge{From: e.To, To: e.From})
	}
	return len(NewGraph(nodes, undirected).components())
}

// groupSizes turns per-group line counts into a slice sorted by name.
func groupSizes(lines map[string]int) []GroupSize {
	var gs []GroupSize
	for name, n := range lines {
		gs = append(gs, GroupSize{Name: name, Lines: n})
	}
	slices.SortFunc(gs, func(a, b GroupSize) int { return cmp.Compare(a.Name, b.Name) })
	return gs
}
