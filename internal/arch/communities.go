package arch

import (
	"cmp"
	"maps"
	"math/rand/v2"
	"slices"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/community"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

// Seeds of the source Louvain orders its moves by. Fixed, so a run is
// reproducible; any value would do. They are codetospec's.
const (
	louvainSeed1 uint64 = 0x636f6465746f7370
	louvainSeed2 uint64 = 0x636f6d6d756e6974
)

// Communities are the groups of packages the import graph itself suggests,
// set beside the declared subsystems (IMPROVEMENTS.md 9.3). They are advice:
// a small edit can move a package between communities, so they never gate.
type Communities struct {
	// Groups are the communities, each sorted, by their first member.
	Groups []Community `json:"groups"`
	// Misplaced lists the packages whose community mostly lies in another
	// declared subsystem, by package.
	Misplaced []Misplaced `json:"misplaced"`
}

// Community is a group of packages that import each other more than they
// import the rest, by Louvain modularity, and that are connected.
type Community struct {
	Packages []string `json:"packages"`
	// Subsystem is the declared subsystem holding most of its packages, ""
	// when none holds any.
	Subsystem string `json:"subsystem,omitempty"`
}

// Misplaced is a package declared in Subsystem, which has other packages,
// whose community mostly lies in Community, another subsystem: a sign that
// it belongs there, or that it couples the two. A subsystem of one package
// has no siblings to sit apart from, so its package is never misplaced.
type Misplaced struct {
	Package   string `json:"package"`
	Subsystem string `json:"subsystem"`
	Community string `json:"community"`
}

// communitiesOf groups the packages of the package graph into communities
// and compares them with the declared subsystems, of in assigned.
//
// The graph is undirected, an edge weighing the imports either way. The
// weights are whole numbers, so their sums are exact in any order. Louvain
// (gonum's community.Modularize at resolution 1, with a fixed source) can
// join packages that are not connected, so each of its communities is split
// into its connected parts, the guarantee Leiden adds over Louvain.
//
// Ensures: deterministic for a given graph and gonum version.
func communitiesOf(packages *Graph, assigned map[string]string) Communities {
	groups := make([]Community, 0)
	for _, part := range louvainParts(packages) {
		groups = append(groups, Community{Packages: part, Subsystem: majority(part, assigned)})
	}
	slices.SortFunc(groups, func(a, b Community) int {
		return cmp.Compare(a.Packages[0], b.Packages[0])
	})
	size := map[string]int{}
	for _, sub := range assigned {
		size[sub]++
	}
	misplaced := make([]Misplaced, 0)
	for _, c := range groups {
		for _, pkg := range c.Packages {
			sub, ok := assigned[pkg]
			if ok && size[sub] > 1 && c.Subsystem != "" && !holdsMost(c, sub, assigned) {
				misplaced = append(misplaced,
					Misplaced{Package: pkg, Subsystem: sub, Community: c.Subsystem})
			}
		}
	}
	slices.SortFunc(
		misplaced,
		func(a, b Misplaced) int { return cmp.Compare(a.Package, b.Package) },
	)
	return Communities{Groups: groups, Misplaced: misplaced}
}

// louvainParts returns the connected parts of the Louvain communities of
// the package graph, each sorted.
func louvainParts(packages *Graph) [][]string {
	if packages.Len() == 0 {
		return nil
	}
	g := simple.NewWeightedUndirectedGraph(0, 0)
	for i := range packages.nodes {
		g.AddNode(simple.Node(int64(i)))
	}
	for from, arcs := range packages.out {
		for _, a := range arcs {
			u, v := int64(min(from, a.to)), int64(max(from, a.to))
			w := float64(a.weight)
			if e := g.WeightedEdge(u, v); e != nil {
				w += e.Weight()
			}
			g.SetWeightedEdge(g.NewWeightedEdge(simple.Node(u), simple.Node(v), w))
		}
	}
	reduced := community.Modularize(g, 1, rand.NewPCG(louvainSeed1, louvainSeed2))
	var parts [][]string
	for _, members := range reduced.Communities() {
		for _, part := range topo.ConnectedComponents(induced(g, members)) {
			names := make([]string, len(part))
			for i, n := range part {
				names[i] = packages.nodes[n.ID()]
			}
			slices.Sort(names)
			parts = append(parts, names)
		}
	}
	return parts
}

// induced is the subgraph of g on members.
func induced(g graph.Undirected, members []graph.Node) graph.Undirected {
	sub := simple.NewUndirectedGraph()
	in := make(map[int64]bool, len(members))
	for _, n := range members {
		sub.AddNode(n)
		in[n.ID()] = true
	}
	for _, n := range members {
		neighbors := g.From(n.ID())
		for neighbors.Next() {
			if m := neighbors.Node(); in[m.ID()] && n.ID() < m.ID() {
				sub.SetEdge(simple.Edge{F: n, T: m})
			}
		}
	}
	return sub
}

// majority returns the declared subsystem holding most of the packages, the
// first by name of those holding equally many, or "" when none holds any.
func majority(packages []string, assigned map[string]string) string {
	counts := map[string]int{}
	for _, pkg := range packages {
		if sub, ok := assigned[pkg]; ok {
			counts[sub]++
		}
	}
	best := ""
	for _, sub := range slices.Sorted(maps.Keys(counts)) {
		if counts[sub] > counts[best] {
			best = sub
		}
	}
	return best
}

// holdsMost reports whether sub holds as many of the community's packages as
// any subsystem does.
func holdsMost(c Community, sub string, assigned map[string]string) bool {
	counts := map[string]int{}
	for _, pkg := range c.Packages {
		counts[assigned[pkg]]++
	}
	return counts[sub] == counts[c.Subsystem]
}
