package arch

import "slices"

// johnson holds the state of Johnson's elementary-cycle enumeration
// ("Finding all the elementary circuits of a directed graph", 1975) from one
// start node.
type johnson struct {
	// nodes and out are the graph's: its sorted names and its arcs.
	nodes []string
	out   [][]arc
	// comp maps each node to its strongly connected component; only nodes of
	// the start's component, and no smaller than the start, are searched.
	comp    []int
	start   int
	blocked []bool
	// unblockBy[w] lists the nodes to unblock when w is unblocked.
	unblockBy [][]int
	stack     []int
	cycles    [][]string
	limit     int
	truncated bool
}

// ElementaryCycles returns the graph's elementary cycles, each a list of
// nodes starting at its smallest, at most limit of them. Start nodes are
// taken in sorted order and arcs in target order, so the first limit cycles
// are the same on every run, unlike an enumeration over a hash map (TODO.md,
// "Ideas from ArchUnit", Cycles). truncated reports that more exist.
//
// Requires: limit > 0.
func (g Graph) ElementaryCycles(limit int) (cycles [][]string, truncated bool) {
	comp := make([]int, len(g.nodes))
	for c, members := range g.components() {
		for _, v := range members {
			comp[v] = c
		}
	}
	j := &johnson{
		nodes: g.nodes, out: g.out, comp: comp, limit: limit,
		blocked:   make([]bool, len(g.nodes)),
		unblockBy: make([][]int, len(g.nodes)),
	}
	for s := range g.nodes {
		if j.truncated {
			break
		}
		j.start = s
		for v := range g.nodes {
			j.blocked[v], j.unblockBy[v] = false, nil
		}
		j.circuit(s)
	}
	return j.cycles, j.truncated
}

// circuit searches for cycles through v back to the start, and reports
// whether it found one.
func (j *johnson) circuit(v int) bool {
	found := false
	j.stack = append(j.stack, v)
	j.blocked[v] = true
	for _, a := range j.out[v] {
		w := a.to
		if j.truncated || !j.searchable(w) {
			continue
		}
		switch {
		case w == j.start:
			j.record()
			found = true
		case !j.blocked[w] && j.circuit(w):
			found = true
		}
	}
	if found {
		j.unblock(v)
	} else {
		j.wait(v)
	}
	j.stack = j.stack[:len(j.stack)-1]
	return found
}

// wait keeps v blocked until one of its successors is unblocked: no cycle
// through v exists until then.
func (j *johnson) wait(v int) {
	for _, a := range j.out[v] {
		if w := a.to; j.searchable(w) && !slices.Contains(j.unblockBy[w], v) {
			j.unblockBy[w] = append(j.unblockBy[w], v)
		}
	}
}

// searchable reports a node in the start's component and no smaller than
// the start: every cycle whose smallest node is the start lies there.
func (j *johnson) searchable(w int) bool {
	return w >= j.start && j.comp[w] == j.comp[j.start]
}

// record keeps the cycle on the stack, or marks the result truncated once
// the limit is reached.
func (j *johnson) record() {
	if len(j.cycles) == j.limit {
		j.truncated = true
		return
	}
	cycle := make([]string, len(j.stack))
	for i, v := range j.stack {
		cycle[i] = j.nodes[v]
	}
	j.cycles = append(j.cycles, cycle)
}

// unblock unblocks v and, transitively, the nodes waiting on it.
func (j *johnson) unblock(v int) {
	j.blocked[v] = false
	waiting := j.unblockBy[v]
	j.unblockBy[v] = nil
	for _, w := range waiting {
		if j.blocked[w] {
			j.unblock(w)
		}
	}
}
