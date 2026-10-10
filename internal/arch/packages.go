package arch

import (
	"cmp"
	"math"
	"slices"
)

// Component Rank parameters (von Zitzewitz, ch. 9): the probability of
// following a dependency rather than stopping, and a fixed number of
// iterations, so the result never depends on a convergence test.
const (
	rankDamping    = 0.8
	rankIterations = 50
	rankListed     = 10
)

// PackageMetrics holds Robert C. Martin's package metrics for one in-scope
// package, counted over in-scope imports only.
type PackageMetrics struct {
	Package string `json:"package"`
	// Ca (afferent coupling) counts the in-scope packages that import it.
	Ca int `json:"ca"`
	// Ce (efferent coupling) counts the in-scope packages it imports.
	Ce int `json:"ce"`
	// Instability is Ce ÷ (Ca + Ce), and 0 for a package with neither.
	Instability float64 `json:"instability"`
	// Abstractness is the share of its named types that are interfaces.
	Abstractness float64 `json:"abstractness"`
	// Distance from the main sequence: |Abstractness + Instability − 1|.
	Distance float64 `json:"distance"`
}

// RankedType is a type with its Component Rank: the probability that a random
// walk along dependencies ends at it. The highest-ranked types are the ones a
// newcomer should read first, because so much depends on them.
type RankedType struct {
	Type string  `json:"type"`
	Rank float64 `json:"rank"`
}

// packageMetricsOf computes Martin's metrics for every package, sorted by
// path.
func packageMetricsOf(p *Program, g *Graphs) []PackageMetrics {
	interfaces, types := map[string]int{}, map[string]int{}
	for i := range p.Types {
		types[p.Types[i].Package]++
		if p.Types[i].Kind == KindInterface {
			interfaces[p.Types[i].Package]++
		}
	}
	ca, ce := map[string]int{}, map[string]int{}
	for _, e := range g.Packages.Edges() {
		ca[e.To]++
		ce[e.From]++
	}
	out := make([]PackageMetrics, 0, len(p.Packages))
	for _, pkg := range p.Packages {
		m := PackageMetrics{Package: pkg.Path, Ca: ca[pkg.Path], Ce: ce[pkg.Path]}
		if m.Ca+m.Ce > 0 {
			m.Instability = round6(float64(m.Ce) / float64(m.Ca+m.Ce))
		}
		if types[pkg.Path] > 0 {
			m.Abstractness = round6(float64(interfaces[pkg.Path]) / float64(types[pkg.Path]))
		}
		m.Distance = round6(math.Abs(m.Abstractness + m.Instability - 1))
		out = append(out, m)
	}
	return out
}

// componentRank returns the highest-ranked types of the type graph, at most
// rankListed of them, by rank and then name.
//
// The ranks follow PageRank over dependencies: each step, a type passes
// rankDamping of its rank, shared equally, to the types it depends on; a type
// that depends on nothing spreads its share over every type. Every product is
// converted explicitly before it is added, so no platform fuses the operations
// into an FMA instruction that would round differently.
func componentRank(g Graph) []RankedType {
	n := g.Len()
	if n == 0 {
		return make([]RankedType, 0)
	}
	rank := make([]float64, n)
	for v := range rank {
		rank[v] = 1 / float64(n)
	}
	base := float64((1 - rankDamping) / float64(n))
	for range rankIterations {
		rank = rankStep(g, rank, base)
	}
	ranked := make([]RankedType, n)
	for v, r := range rank {
		ranked[v] = RankedType{Type: g.nodes[v], Rank: round6(r)}
	}
	slices.SortStableFunc(ranked, func(a, b RankedType) int {
		return cmp.Or(cmp.Compare(b.Rank, a.Rank), cmp.Compare(a.Type, b.Type))
	})
	return ranked[:min(n, rankListed)]
}

// rankStep performs one PageRank iteration and returns the new ranks.
func rankStep(g Graph, rank []float64, base float64) []float64 {
	n := g.Len()
	next := make([]float64, n)
	sinks := 0.0
	for v, arcs := range g.out {
		if len(arcs) == 0 {
			sinks += rank[v]
			continue
		}
		share := float64(rankDamping * rank[v] / float64(len(arcs)))
		for _, a := range arcs {
			next[a.to] += share
		}
	}
	spread := float64(rankDamping * sinks / float64(n))
	for v := range next {
		next[v] += base
		next[v] += spread
	}
	return next
}
