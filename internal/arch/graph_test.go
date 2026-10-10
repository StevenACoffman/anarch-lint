package arch_test

import (
	"slices"
	"strconv"
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// fixtureTypes is the type graph of the example in mmi.md §6: two type
// cycles, Invoice ↔ Line and Product → Category → Item → Product.
func fixtureTypes() arch.Graph {
	const inv, cat, tax = "fx/billing/invoice.", "fx/shop/catalog.", "fx/billing/tax."
	return arch.NewGraph(nil, []arch.Edge{
		{From: inv + "Invoice", To: inv + "Line"},
		{From: inv + "Line", To: inv + "Invoice"},
		{From: inv + "Total", To: inv + "Invoice"},
		{From: cat + "Product", To: cat + "Category"},
		{From: cat + "Category", To: cat + "Item"},
		{From: cat + "Item", To: cat + "Product"},
		{From: cat + "Item", To: inv + "Invoice"},
		{From: tax + "Rate", To: cat + "Product"},
	})
}

func TestCycleGroupsFixture(t *testing.T) {
	t.Parallel()
	equals(t, [][]string{
		{"fx/billing/invoice.Invoice", "fx/billing/invoice.Line"},
		{"fx/shop/catalog.Category", "fx/shop/catalog.Item", "fx/shop/catalog.Product"},
	}, fixtureTypes().CycleGroups())
}

// The values mmi.md §7.5 derives for the fixture by hand.
func TestCouplingFixture(t *testing.T) {
	t.Parallel()
	c := arch.CouplingOf(fixtureTypes())
	equals(t, 7, c.Nodes)
	equals(t, 28, c.CCD)
	equals(t, 4.0, c.ACD)
	equals(t, 0.571429, c.PropagationCost)
	equals(t, 0.515079, c.RelativeCyclicity)
	equals(t, 3, c.MaxCycleGroup)
	// One edge per cycle, each of weight 1: 2 × (10 + 1).
	equals(t, 22, c.SDI)
	equals(t, arch.CutExact, c.CutMethod)
	equals(t, 2, len(c.Cuts))
}

func TestLevelFixture(t *testing.T) {
	t.Parallel()
	l := arch.LevelOf(fixtureTypes())
	equals(t, 7, l.Elements)
	equals(t, 5, l.InCycles)
	equals(t, 5, l.Refs)
	equals(t, 1, l.Direct)
	equals(t, 3, l.MaxGroup)
}

func TestSDIPrefersLightEdges(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		edges []arch.Edge
		sdi   int
		cuts  []arch.Edge
	}{
		"simple ring cuts one edge": {
			edges: []arch.Edge{{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "c", To: "a"}},
			sdi:   11,
		},
		"heavy pair cuts the light direction": {
			edges: []arch.Edge{{From: "a", To: "b", Weight: 5}, {From: "b", To: "a", Weight: 1}},
			sdi:   11,
			cuts:  []arch.Edge{{From: "b", To: "a", Weight: 1}},
		},
		// von Zitzewitz's second thought experiment, shrunk to 4 nodes: every
		// pair is bidirectional, so half of the 12 edges must go.
		"complete graph cuts half": {
			edges: completeGraph(4),
			sdi:   6 * 11,
		},
		"acyclic graph cuts nothing": {
			edges: []arch.Edge{{From: "a", To: "b"}, {From: "b", To: "c"}},
			sdi:   0,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := arch.CouplingOf(arch.NewGraph(nil, tc.edges))
			equals(t, tc.sdi, c.SDI)
			if tc.cuts != nil {
				equals(t, tc.cuts, c.Cuts)
			}
		})
	}
}

func TestGreedyCutBreaksEveryCycle(t *testing.T) {
	t.Parallel()
	edges := completeGraph(14) // above the exhaustive-search limit
	c := arch.CouplingOf(arch.NewGraph(nil, edges))
	equals(t, arch.CutGreedy, c.CutMethod)
	assert(t, len(withoutEdges(edges, c.Cuts).CycleGroups()) == 0, "cuts left a cycle")
}

func TestEmptyGraph(t *testing.T) {
	t.Parallel()
	c := arch.CouplingOf(arch.Graph{})
	equals(t, arch.Coupling{CutMethod: arch.CutExact, Cuts: []arch.Edge{}}, c)
	equals(t, 0, arch.LevelOf(arch.Graph{}).Elements)
}

func TestGraphProperties(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		edges := rapid.SliceOfN(edgeGen(), 0, 40).Draw(t, "edges")
		g := arch.NewGraph(nil, edges)
		groups := g.CycleGroups()

		// The groups are disjoint.
		seen := map[string]bool{}
		for _, grp := range groups {
			for _, n := range grp {
				if seen[n] {
					t.Fatalf("%s is in two cycle groups", n)
				}
				seen[n] = true
			}
		}
		// The result does not depend on the order the edges came in.
		shuffled := slices.Clone(edges)
		slices.Reverse(shuffled)
		h := arch.NewGraph(nil, shuffled)
		if !slices.EqualFunc(groups, h.CycleGroups(), slices.Equal[[]string]) {
			t.Fatalf("cycle groups depend on edge order")
		}
		if arch.CouplingOf(g).SDI != arch.CouplingOf(h).SDI {
			t.Fatalf("SDI depends on edge order")
		}
		// Cutting the reported edges leaves the graph acyclic.
		if left := withoutEdges(edges, arch.CouplingOf(g).Cuts).CycleGroups(); len(left) > 0 {
			t.Fatalf("cuts left cycle groups %v", left)
		}
		// Every node reaches at least itself and at most every node.
		for _, d := range g.DependsUpon() {
			if d < 1 || d > g.Len() {
				t.Fatalf("Depends Upon %d outside [1, %d]", d, g.Len())
			}
		}
	})
}

func edgeGen() *rapid.Generator[arch.Edge] {
	node := rapid.SampledFrom([]string{"a", "b", "c", "d", "e", "f", "g", "h"})
	return rapid.Custom(func(t *rapid.T) arch.Edge {
		return arch.Edge{
			From:   node.Draw(t, "from"),
			To:     node.Draw(t, "to"),
			Weight: rapid.IntRange(1, 5).Draw(t, "weight"),
		}
	})
}

func completeGraph(n int) []arch.Edge {
	var edges []arch.Edge
	for i := range n {
		for j := range n {
			if i != j {
				edges = append(edges, arch.Edge{From: strconv.Itoa(i), To: strconv.Itoa(j)})
			}
		}
	}
	return edges
}

func withoutEdges(edges, cuts []arch.Edge) arch.Graph {
	cut := map[[2]string]bool{}
	for _, c := range cuts {
		cut[[2]string{c.From, c.To}] = true
	}
	var kept []arch.Edge
	for _, e := range edges {
		if !cut[[2]string{e.From, e.To}] {
			kept = append(kept, e)
		}
	}
	return arch.NewGraph(nil, kept)
}

func TestComponentRankAndPackageMetrics(t *testing.T) {
	t.Parallel()
	m := fixtureModel()
	r := analyze(&m)
	// Invoice is reached from Line, Total and, through Item, the catalog
	// cycle: it is the type to read first.
	equals(t, "fx/billing/invoice.Invoice", r.ComponentRank[0].Type)
	total := 0.0
	for _, rt := range r.ComponentRank {
		total += rt.Rank
	}
	assert(t, total > 0.99 && total < 1.01, "ranks of a 7-type graph should sum to 1")

	byPkg := map[string]arch.PackageMetrics{}
	for _, pm := range r.Packages {
		byPkg[pm.Package] = pm
	}
	// catalog imports invoice and is imported by tax.
	equals(t, arch.PackageMetrics{
		Package: "fx/shop/catalog", Ca: 1, Ce: 1, Instability: 0.5, Abstractness: 0, Distance: 0.5,
	}, byPkg["fx/shop/catalog"])
	// invoice imports nothing in scope: maximally stable, and concrete.
	equals(t, 1.0, byPkg["fx/billing/invoice"].Distance)
}

func TestDominators(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		edges []arch.Edge
		want  map[string]string
	}{
		"chain": {
			edges: []arch.Edge{{From: "r", To: "a"}, {From: "a", To: "b"}},
			want:  map[string]string{"r": "r", "a": "r", "b": "a"},
		},
		"diamond joins at the root": {
			edges: []arch.Edge{
				{From: "r", To: "a"},
				{From: "r", To: "b"},
				{From: "a", To: "c"},
				{From: "b", To: "c"},
			},
			want: map[string]string{"r": "r", "a": "r", "b": "r", "c": "r"},
		},
		"unreachable nodes are absent": {
			edges: []arch.Edge{{From: "r", To: "a"}, {From: "x", To: "a"}},
			want:  map[string]string{"r": "r", "a": "r"},
		},
		"a cycle below a single entry": {
			edges: []arch.Edge{
				{From: "r", To: "a"},
				{From: "a", To: "b"},
				{From: "b", To: "a"},
				{From: "b", To: "c"},
			},
			want: map[string]string{"r": "r", "a": "r", "b": "a", "c": "b"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			equals(t, tc.want, arch.NewGraph(nil, tc.edges).Dominators("r"))
		})
	}
}
