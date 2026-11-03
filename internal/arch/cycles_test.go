package arch_test

import (
	"slices"
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

func TestElementaryCycles(t *testing.T) {
	t.Parallel()
	edge := func(from, to string) arch.Edge { return arch.Edge{From: from, To: to, Weight: 1} }
	cases := map[string]struct {
		edges     []arch.Edge
		limit     int
		cycles    [][]string
		truncated bool
	}{
		"acyclic": {
			edges: []arch.Edge{edge("a", "b"), edge("b", "c")}, limit: 10,
		},
		"a triangle with a chord": {
			edges:  []arch.Edge{edge("a", "b"), edge("b", "c"), edge("c", "a"), edge("b", "a")},
			limit:  10,
			cycles: [][]string{{"a", "b"}, {"a", "b", "c"}},
		},
		"two disjoint cycles": {
			edges:  []arch.Edge{edge("a", "b"), edge("b", "a"), edge("x", "y"), edge("y", "x")},
			limit:  10,
			cycles: [][]string{{"a", "b"}, {"x", "y"}},
		},
		"the limit truncates in a fixed order": {
			edges:     []arch.Edge{edge("a", "b"), edge("b", "a"), edge("x", "y"), edge("y", "x")},
			limit:     1,
			cycles:    [][]string{{"a", "b"}},
			truncated: true,
		},
		"the complete graph on three nodes": {
			edges: []arch.Edge{
				edge("a", "b"), edge("b", "a"), edge("b", "c"), edge("c", "b"),
				edge("a", "c"), edge("c", "a"),
			},
			limit: 10,
			cycles: [][]string{
				{"a", "b"}, {"a", "b", "c"}, {"a", "c"}, {"a", "c", "b"}, {"b", "c"},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := arch.NewGraph(nil, tc.edges)
			cycles, truncated := g.ElementaryCycles(tc.limit)
			equals(t, tc.cycles, cycles)
			equals(t, tc.truncated, truncated)

			reversed := slices.Clone(tc.edges)
			slices.Reverse(reversed)
			again, _ := arch.NewGraph(nil, reversed).ElementaryCycles(tc.limit)
			equals(t, cycles, again)
		})
	}
}

// TestClosingCycleIsShortest: the closing cycle of an edge is a cycle that
// starts with the edge, and no elementary cycle through the edge is
// shorter; an edge on no cycle closes none.
func TestClosingCycleIsShortest(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		g := arch.NewGraph(nil, rapid.SliceOfN(edgeGen(), 0, 24).Draw(t, "edges"))
		all, truncated := g.ElementaryCycles(100_000)
		if truncated {
			t.Skip("too many cycles to compare with")
		}
		for _, e := range g.Edges() {
			shortest := 0
			for _, c := range all {
				for i := range c {
					if c[i] == e.From && c[(i+1)%len(c)] == e.To &&
						(shortest == 0 || len(c) < shortest) {
						shortest = len(c)
					}
				}
			}
			got := g.ClosingCycle(e.From, e.To)
			if len(got) != shortest {
				t.Fatalf("%s → %s: closing cycle %v, want length %d", e.From, e.To, got, shortest)
			}
			if got == nil {
				continue
			}
			if got[0] != e.From || got[1] != e.To {
				t.Fatalf(
					"%s → %s: closing cycle %v does not start with the edge",
					e.From,
					e.To,
					got,
				)
			}
			for i := range got {
				if g.Weight(got[i], got[(i+1)%len(got)]) == 0 {
					t.Fatalf("%s → %s: %v is not a cycle", e.From, e.To, got)
				}
			}
		}
	})
}

func TestClosingCycle(t *testing.T) {
	t.Parallel()
	g := arch.NewGraph(nil, []arch.Edge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
		{From: "c", To: "a"},
		{From: "b", To: "d"},
		{From: "d", To: "a"},
		{From: "a", To: "e"},
	})
	cases := map[string]struct {
		from, to string
		want     []string
	}{
		"ties go to the sorted path": {from: "a", to: "b", want: []string{"a", "b", "c"}},
		"a longer way round":         {from: "c", to: "a", want: []string{"c", "a", "b"}},
		"an edge on no cycle":        {from: "a", to: "e"},
		"no such edge":               {from: "a", to: "c"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			equals(t, tc.want, g.ClosingCycle(tc.from, tc.to))
		})
	}
}
