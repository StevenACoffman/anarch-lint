package arch_test

import (
	"slices"
	"testing"

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
