package arch

import (
	"cmp"
	"math"
	"slices"
)

// exactCutLimit is the largest cycle group whose minimum cut is found by
// exhaustive search. The search is O(2^n · n²); above the limit a
// deterministic greedy ordering is used instead (mmi.md §7.5: the exact
// problem is NP-hard, so the method is reported with the value).
const exactCutLimit = 12

// Cut methods, reported with the SDI so a reader knows whether the value is a
// minimum or a heuristic bound.
const (
	// CutExact means every cycle group was small enough to search exhaustively.
	CutExact = "exact"
	// CutGreedy means at least one group used the Eades–Lin–Smyth ordering.
	CutGreedy = "greedy"
)

// Coupling holds von Zitzewitz's coupling and cyclicity metrics for one graph
// (mmi.md §7.5). Ratios are rounded to six decimal places so that encoded
// output is identical on every platform.
type Coupling struct {
	// Nodes is the number of elements, n.
	Nodes int `json:"nodes"`
	// CCD is the Cumulative Component Dependency: the sum of Depends Upon,
	// where each element counts itself.
	CCD int `json:"ccd"`
	// ACD is the Average Component Dependency, CCD ÷ n.
	ACD float64 `json:"acd"`
	// PropagationCost is CCD ÷ n².
	PropagationCost float64 `json:"propagationCost"`
	// RelativeCyclicity is √(Σ size² over the cycle groups) ÷ n.
	RelativeCyclicity float64 `json:"relativeCyclicity"`
	// MaxCycleGroup is the size of the largest cycle group.
	MaxCycleGroup int `json:"maxCycleGroup"`
	// SDI is the Structural Debt Index: 10 × (edges to cut) + Σ (their
	// weights), over a set of edges whose removal breaks every cycle.
	SDI int `json:"sdi"`
	// CutMethod says how the cut was found: CutExact or CutGreedy.
	CutMethod string `json:"cutMethod"`
	// Cuts lists the edges to cut, sorted, as actionable advice.
	Cuts []Edge `json:"cuts"`
}

// eadesState is the shrinking graph greedyOrder works on: which nodes remain,
// and the summed cut cost of each remaining node's incoming and outgoing
// edges.
type eadesState struct {
	g       Graph
	alive   []bool
	in, out []int
	sources [][]arc // per node, the arcs that point to it, seen from their source
}

// CouplingOf computes the coupling metrics of g. An empty graph yields zero
// ratios rather than a division by zero.
func CouplingOf(g Graph) Coupling {
	c := Coupling{Nodes: g.Len(), CutMethod: CutExact, Cuts: make([]Edge, 0)}
	for _, d := range g.DependsUpon() {
		c.CCD += d
	}
	cyclicity := 0
	for _, grp := range g.CycleGroups() {
		cyclicity += len(grp) * len(grp)
		c.MaxCycleGroup = max(c.MaxCycleGroup, len(grp))
		cuts, exact := minimumCut(g.subgraph(grp))
		if !exact {
			c.CutMethod = CutGreedy
		}
		c.Cuts = append(c.Cuts, cuts...)
	}
	for _, e := range c.Cuts {
		c.SDI += cutCost(e)
	}
	slices.SortFunc(c.Cuts, compareEdges)
	if n := float64(c.Nodes); n > 0 {
		c.ACD = round6(float64(c.CCD) / n)
		c.PropagationCost = round6(float64(c.CCD) / (n * n))
		c.RelativeCyclicity = round6(math.Sqrt(float64(cyclicity)) / n)
	}
	return c
}

// minimumCut returns edges of the strongly connected graph g whose removal
// leaves it acyclic, choosing a node order and cutting every edge that runs
// backward in it. The order minimises the total SDI cost when g is small
// enough to search exhaustively; exact reports whether it was.
func minimumCut(g Graph) (cuts []Edge, exact bool) {
	var order []int
	if g.Len() <= exactCutLimit {
		order, exact = exactOrder(g), true
	} else {
		order = greedyOrder(g)
	}
	pos := make([]int, g.Len())
	for i, v := range order {
		pos[v] = i
	}
	for from, arcs := range g.out {
		for _, a := range arcs {
			if pos[from] > pos[a.to] {
				cuts = append(cuts, Edge{From: g.nodes[from], To: g.nodes[a.to], Weight: a.weight})
			}
		}
	}
	return cuts, exact
}

// exactOrder finds a node order with the least total cost of backward edges by
// dynamic programming over subsets: best[S] is the cheapest cost of placing
// the nodes of S first. Ties keep the lower-numbered node, so the result is
// deterministic.
//
// Requires: g.Len() <= exactCutLimit.
func exactOrder(g Graph) []int {
	n := g.Len()
	// backTo[v][u] is the cost of the edge v→u, paid when v is placed after u.
	backTo := make([][]int, n)
	for v := range n {
		backTo[v] = make([]int, n)
		for _, a := range g.out[v] {
			backTo[v][a.to] = cutCost(Edge{Weight: a.weight})
		}
	}
	full := 1<<n - 1
	best := make([]int, full+1)
	last := make([]int, full+1)
	for s := 1; s <= full; s++ {
		best[s] = math.MaxInt
		for v := range n {
			if s&(1<<v) == 0 {
				continue
			}
			prev := s &^ (1 << v)
			if cost := best[prev] + placedAfter(backTo[v], prev); cost < best[s] {
				best[s], last[s] = cost, v
			}
		}
	}
	order := make([]int, n)
	for s, i := full, n-1; i >= 0; i-- {
		order[i] = last[s]
		s &^= 1 << last[s]
	}
	return order
}

// placedAfter is the cost of the backward edges created by placing a node
// after every node in the set placed: the sum of its edge costs into them.
func placedAfter(costTo []int, placed int) int {
	cost := 0
	for u, c := range costTo {
		if placed&(1<<u) != 0 {
			cost += c
		}
	}
	return cost
}

// greedyOrder returns the Eades–Lin–Smyth ordering of g: repeatedly move a
// sink to the back or a source to the front, and otherwise move to the front
// the node whose outgoing cost most exceeds its incoming cost. Ties go to the
// lower-numbered node.
func greedyOrder(g Graph) []int {
	s := newEadesState(g)
	var front, back []int
	for range g.Len() {
		v := s.pick()
		if s.out[v] == 0 {
			back = append(back, v)
		} else {
			front = append(front, v)
		}
		s.remove(v)
	}
	slices.Reverse(back)
	return append(front, back...)
}

func newEadesState(g Graph) *eadesState {
	n := g.Len()
	s := &eadesState{
		g:       g,
		alive:   make([]bool, n),
		in:      make([]int, n),
		out:     make([]int, n),
		sources: make([][]arc, n),
	}
	for v, arcs := range g.out {
		s.alive[v] = true
		for _, a := range arcs {
			cost := cutCost(Edge{Weight: a.weight})
			s.out[v] += cost
			s.in[a.to] += cost
			s.sources[a.to] = append(s.sources[a.to], arc{to: v, weight: a.weight})
		}
	}
	return s
}

// pick chooses the next node: a sink or a source first, else the node with
// the largest out−in cost.
func (s *eadesState) pick() int {
	bestV, bestScore := -1, math.MinInt
	for v, alive := range s.alive {
		if !alive {
			continue
		}
		score := s.out[v] - s.in[v]
		if s.out[v] == 0 || s.in[v] == 0 {
			score = math.MaxInt
		}
		if score > bestScore {
			bestV, bestScore = v, score
		}
	}
	return bestV
}

// remove takes v out of the graph, with every edge that touches it.
func (s *eadesState) remove(v int) {
	s.alive[v] = false
	for _, a := range s.g.out[v] {
		s.in[a.to] -= cutCost(Edge{Weight: a.weight})
	}
	for _, a := range s.sources[v] {
		s.out[a.to] -= cutCost(Edge{Weight: a.weight})
	}
}

// cutCost is the SDI cost of cutting one edge: 10 for the decision to break
// it plus its weight for the usages to rewrite.
func cutCost(e Edge) int { return 10 + e.Weight }

func compareEdges(a, b Edge) int {
	return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
}

// round6 rounds x to six decimal places.
func round6(x float64) float64 {
	scaled := x * 1e6
	return math.Round(scaled) / 1e6
}
