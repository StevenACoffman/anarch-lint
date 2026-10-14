package arch

import (
	"slices"

	"gonum.org/v1/gonum/graph/flow"
	"gonum.org/v1/gonum/graph/simple"
)

// Dominators returns the immediate dominator of every node reachable from
// root: the node every path from root to it passes through last. root
// dominates itself. Unreachable nodes are absent. It is gonum's
// Lengauer–Tarjan flow.Dominators; the dominator tree is unique, so the
// result does not depend on the order of the search (mmi-ref-plan.md §4.5).
func (g Graph) Dominators(root string) map[string]string {
	r, ok := g.index[root]
	if !ok {
		return map[string]string{}
	}
	tree := flow.Dominators(simple.Node(int64(r)), asGonum(&g))
	out := map[string]string{root: root}
	for v := range g.nodes {
		if d := tree.DominatorOf(int64(v)); d != nil && v != r {
			out[g.nodes[v]] = g.nodes[d.ID()]
		}
	}
	return out
}

// shortestPath returns a shortest path from one node to another, breadth
// first over arcs in sorted order, or nil when there is none.
func (g Graph) shortestPath(from, to string) []string {
	f, okFrom := g.index[from]
	t, okTo := g.index[to]
	if !okFrom || !okTo {
		return nil
	}
	prev := make([]int, len(g.nodes))
	for i := range prev {
		prev[i] = -1
	}
	prev[f] = f
	queue := []int{f}
	for len(queue) > 0 && prev[t] < 0 {
		v := queue[0]
		queue = queue[1:]
		for _, a := range g.out[v] {
			if prev[a.to] < 0 {
				prev[a.to] = v
				queue = append(queue, a.to)
			}
		}
	}
	if prev[t] < 0 {
		return nil
	}
	var path []string
	for v := t; v != f; v = prev[v] {
		path = append(path, g.nodes[v])
	}
	path = append(path, from)
	slices.Reverse(path)
	return path
}
