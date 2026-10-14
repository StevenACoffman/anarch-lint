package arch

import (
	"cmp"
	"slices"

	"gonum.org/v1/gonum/graph/topo"
)

// Edge is a directed, weighted dependency between two named elements.
//
// Weight counts the relationships the edge stands for: the identifiers that
// refer from one type to another, or the package imports a subsystem edge
// aggregates. It is at least 1.
type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Weight int    `json:"weight"`
}

// Graph is an immutable directed graph over named elements. Self-loops are
// never stored: an element that refers to itself is not a cycle between
// elements (mmi.md §5).
//
// Construct one with [NewGraph]. The zero Graph is empty and ready to use.
type Graph struct {
	nodes []string       // sorted, unique
	index map[string]int // node name -> position in nodes
	out   [][]arc        // per node, sorted by target position
}

// arc is an edge seen from its source: the target's position and the weight.
type arc struct {
	to, weight int
}

// NewGraph returns the graph with the given nodes and edges. Nodes named only
// by an edge are added; duplicate edges have their weights summed; an edge
// with a weight below 1 counts as 1. Neither argument is modified.
func NewGraph(nodes []string, edges []Edge) Graph {
	names := slices.Clone(nodes)
	for _, e := range edges {
		names = append(names, e.From, e.To)
	}
	slices.Sort(names)
	names = slices.Compact(names)

	g := Graph{nodes: names, index: make(map[string]int, len(names))}
	for i, n := range names {
		g.index[n] = i
	}
	weights := make([]map[int]int, len(names))
	for _, e := range edges {
		from, to := g.index[e.From], g.index[e.To]
		if from == to {
			continue
		}
		if weights[from] == nil {
			weights[from] = map[int]int{}
		}
		weights[from][to] += max(e.Weight, 1)
	}
	g.out = make([][]arc, len(names))
	for from, targets := range weights {
		for to, w := range targets {
			g.out[from] = append(g.out[from], arc{to: to, weight: w})
		}
		slices.SortFunc(g.out[from], func(a, b arc) int { return cmp.Compare(a.to, b.to) })
	}
	return g
}

// Len returns the number of nodes.
func (g Graph) Len() int { return len(g.nodes) }

// Nodes returns the node names in sorted order.
func (g Graph) Nodes() []string { return slices.Clone(g.nodes) }

// Edges returns every edge, sorted by source and then target.
func (g Graph) Edges() []Edge {
	var edges []Edge
	for from, arcs := range g.out {
		for _, a := range arcs {
			edges = append(edges, Edge{From: g.nodes[from], To: g.nodes[a.to], Weight: a.weight})
		}
	}
	return edges
}

// Weight returns the weight of the edge from one node to another, or 0 when
// there is no such edge.
func (g Graph) Weight(from, to string) int {
	f, okFrom := g.index[from]
	t, okTo := g.index[to]
	if !okFrom || !okTo {
		return 0
	}
	if i, found := slices.BinarySearchFunc(g.out[f], t, func(a arc, t int) int {
		return cmp.Compare(a.to, t)
	}); found {
		return g.out[f][i].weight
	}
	return 0
}

// CycleGroups returns the strongly connected components with more than one
// node, which Lilienthal calls cycle groups (mmi.md §4). Each group is sorted,
// and the groups are sorted by their first member.
//
// Ensures: the groups are disjoint, and every member of a group can reach
// every other member; the result depends only on the graph, never on the
// order in which its edges were supplied.
func (g Graph) CycleGroups() [][]string {
	groups := make([][]string, 0)
	for _, comp := range g.components() {
		if len(comp) < 2 {
			continue
		}
		group := make([]string, len(comp))
		for i, v := range comp {
			group[i] = g.nodes[v]
		}
		slices.Sort(group)
		groups = append(groups, group)
	}
	slices.SortFunc(groups, func(a, b []string) int { return cmp.Compare(a[0], b[0]) })
	return groups
}

// DependsUpon returns, for every node in sorted order, the number of nodes it
// reaches directly or indirectly, counting itself (Lakos's Depends Upon,
// mmi.md §7.5).
func (g Graph) DependsUpon() []int {
	comps := g.components()
	compOf := make([]int, len(g.nodes))
	for c, comp := range comps {
		for _, v := range comp {
			compOf[v] = c
		}
	}
	// Tarjan emits a component only after every component it reaches, so one
	// pass in emission order sees each successor's closure already complete.
	reach := make([]bitset, len(comps))
	for c, comp := range comps {
		reach[c] = g.componentReach(c, comp, compOf, reach)
	}
	counts := make([]int, len(g.nodes))
	for v := range g.nodes {
		counts[v] = reach[compOf[v]].count()
	}
	return counts
}

// componentReach returns the nodes reachable from component c, whose members
// are comp: the members themselves and the reach of every component their
// arcs lead to. compOf maps a node to its component.
//
// Requires: reach is complete for every component c's members can reach,
// which Tarjan's emission order guarantees.
func (g Graph) componentReach(c int, comp, compOf []int, reach []bitset) bitset {
	r := newBitset(len(g.nodes))
	for _, v := range comp {
		r.set(v)
		for _, a := range g.out[v] {
			if compOf[a.to] != c {
				r.or(reach[compOf[a.to]])
			}
		}
	}
	return r
}

// subgraph returns the graph induced by the given nodes.
func (g Graph) subgraph(members []string) Graph {
	in := make(map[string]bool, len(members))
	for _, m := range members {
		in[m] = true
	}
	var edges []Edge
	for _, e := range g.Edges() {
		if in[e.From] && in[e.To] {
			edges = append(edges, e)
		}
	}
	return NewGraph(members, edges)
}

// components returns every strongly connected component, singletons
// included, in the order Tarjan's algorithm emits them: a component comes
// after every component it can reach. It is gonum's topo.TarjanSCC over the
// nodes in sorted order and arcs in target order, so the result is
// deterministic.
func (g Graph) components() [][]int {
	sccs := topo.TarjanSCC(asGonum(&g))
	comps := make([][]int, len(sccs))
	for i, scc := range sccs {
		comps[i] = make([]int, len(scc))
		for j, n := range scc {
			comps[i][j] = int(n.ID())
		}
	}
	return comps
}
