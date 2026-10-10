package arch

import "slices"

// Dominators returns the immediate dominator of every node reachable from
// root: the node every path from root to it passes through last. root
// dominates itself. Unreachable nodes are absent.
//
// It is the Cooper–Harvey–Kennedy iterative algorithm over a reverse
// postorder of a depth-first search in sorted order, so the result is
// deterministic (adonovan/spaghetti's dom.go answers the same question with
// Lengauer–Tarjan; mmi-ref-plan.md §4.5).
func (g Graph) Dominators(root string) map[string]string {
	r, ok := g.index[root]
	if !ok {
		return map[string]string{}
	}
	order := g.postorder(r) // order[i] is the node with postorder number i
	num := make([]int, len(g.nodes))
	for i := range num {
		num[i] = -1
	}
	for i, v := range order {
		num[v] = i
	}
	preds := g.predecessors(num)
	idom := make([]int, len(g.nodes))
	for i := range idom {
		idom[i] = -1
	}
	idom[r] = r
	for changed := true; changed; {
		changed = false
		for i := len(order) - 2; i >= 0; i-- { // reverse postorder, root excluded
			v := order[i]
			if d := newIdom(preds[v], idom, num); d != idom[v] {
				idom[v], changed = d, true
			}
		}
	}
	out := make(map[string]string, len(order))
	for _, v := range order {
		out[g.nodes[v]] = g.nodes[idom[v]]
	}
	return out
}

// postorder returns the nodes reachable from r in depth-first postorder.
func (g Graph) postorder(r int) []int {
	seen := make([]bool, len(g.nodes))
	var order []int
	var visit func(v int)
	visit = func(v int) {
		seen[v] = true
		for _, a := range g.out[v] {
			if !seen[a.to] {
				visit(a.to)
			}
		}
		order = append(order, v)
	}
	visit(r)
	return order
}

// predecessors returns, for each node, the reachable nodes with an arc to it.
// num is each node's postorder number, or -1 when unreachable.
func (g Graph) predecessors(num []int) [][]int {
	preds := make([][]int, len(g.nodes))
	for v, arcs := range g.out {
		if num[v] < 0 {
			continue
		}
		for _, a := range arcs {
			preds[a.to] = append(preds[a.to], v)
		}
	}
	return preds
}

// newIdom intersects the dominators of the processed predecessors of a node.
func newIdom(preds, idom, num []int) int {
	d := -1
	for _, p := range preds {
		switch {
		case idom[p] < 0:
		case d < 0:
			d = p
		default:
			d = intersect(p, d, idom, num)
		}
	}
	return d
}

// intersect walks two nodes up the dominator tree until they meet.
func intersect(a, b int, idom, num []int) int {
	for a != b {
		for num[a] < num[b] {
			a = idom[a]
		}
		for num[b] < num[a] {
			b = idom[b]
		}
	}
	return a
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
