package arch_test

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// encapsulationNodes are the elements of the generated graphs.
var encapsulationNodes = []string{"a", "b", "c", "d", "e", "f", "g", "h"}

// packageOf maps each type of g to its package, the text before the last dot.
func packageOf(g arch.Graph) map[string]string {
	moduleOf := map[string]string{}
	for _, n := range g.Nodes() {
		moduleOf[n] = n[:strings.LastIndex(n, ".")]
	}
	return moduleOf
}

// The values the fixture's packages give by hand: invoice has 3 internal
// edges and Item → Invoice crossing in; catalog has 3 internal edges, Item →
// Invoice crossing out and Rate → Product crossing in; tax has only Rate →
// Product.
func TestClusterFactorsFixture(t *testing.T) {
	t.Parallel()
	g := fixtureTypes()
	equals(t, []arch.ClusterFactor{
		{Module: "fx/billing/invoice", Internal: 3, Boundary: 1, Factor: 0.857143},
		{Module: "fx/billing/tax", Internal: 0, Boundary: 1, Factor: 0},
		{Module: "fx/shop/catalog", Internal: 3, Boundary: 2, Factor: 0.75},
	}, arch.ClusterFactorsOf(g, packageOf(g)))
}

// Line, Total and Category stay inside their packages; Invoice and Product
// are only reached from outside; Item and Rate only reach outside.
func TestDependencyProfileFixture(t *testing.T) {
	t.Parallel()
	g := fixtureTypes()
	sizeOf := map[string]int{}
	for _, n := range g.Nodes() {
		sizeOf[n] = 1
	}
	equals(t, arch.DependencyProfile{Internal: 3, Inbound: 2, Outbound: 2},
		arch.DependencyProfileOf(g, packageOf(g), sizeOf))
}

// moduleOfGen assigns most elements to one of three modules and leaves some
// in none.
func moduleOfGen() *rapid.Generator[map[string]string] {
	module := rapid.SampledFrom([]string{"m1", "m2", "m3", ""})
	return rapid.Custom(func(t *rapid.T) map[string]string {
		moduleOf := map[string]string{}
		for _, n := range encapsulationNodes {
			if m := module.Draw(t, n); m != "" {
				moduleOf[n] = m
			}
		}
		return moduleOf
	})
}

// factorOf returns module m's entry in the cluster factors of g.
func factorOf(g arch.Graph, moduleOf map[string]string, m string) arch.ClusterFactor {
	for _, cf := range arch.ClusterFactorsOf(g, moduleOf) {
		if cf.Module == m {
			return cf
		}
	}
	return arch.ClusterFactor{Module: m}
}

// membersOf returns the elements moduleOf assigns to m.
func membersOf(moduleOf map[string]string, m string) []string {
	var members []string
	for _, n := range encapsulationNodes {
		if moduleOf[n] == m {
			members = append(members, n)
		}
	}
	return members
}

// docs/adr/0021: the cluster factor satisfies Briand, Morasca and Basili's
// four cohesion properties (CS-TR-3368 §3.4), and the dependency profile
// partitions the elements it profiles.
func TestADR_0021_CohesionIsMeasuredAtTheBoundary(t *testing.T) {
	t.Parallel()
	t.Run("Cohesion.1 non-negativity and normalization", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := arch.NewGraph(
				encapsulationNodes,
				rapid.SliceOfN(edgeGen(), 0, 40).Draw(t, "edges"),
			)
			for _, cf := range arch.ClusterFactorsOf(g, moduleOfGen().Draw(t, "moduleOf")) {
				if cf.Factor < 0 || cf.Factor > 1 {
					t.Fatalf("%s: factor %v outside [0, 1]", cf.Module, cf.Factor)
				}
			}
		})
	})
	t.Run("Cohesion.2 null value", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := arch.NewGraph(
				encapsulationNodes,
				rapid.SliceOfN(edgeGen(), 0, 40).Draw(t, "edges"),
			)
			for _, cf := range arch.ClusterFactorsOf(g, moduleOfGen().Draw(t, "moduleOf")) {
				if cf.Internal == 0 && cf.Factor != 0 {
					t.Fatalf("%s: no internal edges but factor %v", cf.Module, cf.Factor)
				}
			}
		})
	})
	t.Run("Cohesion.3 monotonicity", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			edges := rapid.SliceOfN(edgeGen(), 0, 40).Draw(t, "edges")
			moduleOf := moduleOfGen().Draw(t, "moduleOf")
			m := rapid.SampledFrom([]string{"m1", "m2", "m3"}).Draw(t, "module")
			members := membersOf(moduleOf, m)
			if len(members) < 2 {
				t.Skip("an internal edge needs two members")
			}
			from := rapid.SampledFrom(members).Draw(t, "from")
			to := rapid.SampledFrom(members).
				Filter(func(n string) bool { return n != from }).
				Draw(t, "to")
			before := factorOf(arch.NewGraph(encapsulationNodes, edges), moduleOf, m)
			added := append(edges[:len(edges):len(edges)], arch.Edge{From: from, To: to, Weight: 1})
			after := factorOf(arch.NewGraph(encapsulationNodes, added), moduleOf, m)
			if after.Factor < before.Factor {
				t.Fatalf(
					"%s: an internal edge lowered the factor from %v to %v",
					m,
					before.Factor,
					after.Factor,
				)
			}
		})
	})
	t.Run("Cohesion.4 cohesive modules", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			moduleOf := moduleOfGen().Draw(t, "moduleOf")
			// m1 and m2 have no relationships between them.
			var edges []arch.Edge
			for _, e := range rapid.SliceOfN(edgeGen(), 0, 40).Draw(t, "edges") {
				ends := moduleOf[e.From] + moduleOf[e.To]
				if ends != "m1m2" && ends != "m2m1" {
					edges = append(edges, e)
				}
			}
			g := arch.NewGraph(encapsulationNodes, edges)
			largest := max(factorOf(g, moduleOf, "m1").Factor, factorOf(g, moduleOf, "m2").Factor)
			merged := map[string]string{}
			for n, m := range moduleOf {
				if m == "m2" {
					m = "m1"
				}
				merged[n] = m
			}
			if f := factorOf(g, merged, "m1").Factor; f > largest {
				t.Fatalf("merging unrelated modules raised the factor from %v to %v", largest, f)
			}
		})
	})
	t.Run("the profile partitions the profiled elements", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := arch.NewGraph(
				encapsulationNodes,
				rapid.SliceOfN(edgeGen(), 0, 40).Draw(t, "edges"),
			)
			moduleOf := moduleOfGen().Draw(t, "moduleOf")
			sizeOf, total := map[string]int{}, 0
			for _, n := range encapsulationNodes {
				sizeOf[n] = rapid.IntRange(0, 100).Draw(t, "size "+n)
				if _, ok := moduleOf[n]; ok {
					total += sizeOf[n]
				}
			}
			if d := arch.DependencyProfileOf(g, moduleOf, sizeOf); d.Total() != total {
				t.Fatalf("profile %+v totals %d, want %d", d, d.Total(), total)
			}
		})
	})
}
