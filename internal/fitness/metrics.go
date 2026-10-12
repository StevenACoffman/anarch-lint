package fitness

import (
	"math/bits"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Lakos holds John Lakos's dependency metrics for one graph, as ArchUnit's
// LakosMetrics reports them.
type Lakos struct {
	Level string  `json:"level"`
	Nodes int     `json:"nodes"`
	CCD   int     `json:"ccd"`
	ACD   float64 `json:"acd"`
	// RACD is the relative ACD, ACD ÷ n: analyze's propagation cost.
	RACD float64 `json:"racd"`
	// NCCD is CCD ÷ the CCD of a balanced binary tree of the same size,
	// Σᵢ₌₁ⁿ (⌊log₂ i⌋ + 1). Below 1, the graph is flatter than a tree; it
	// adjusts for size, so it ratchets more fairly than ACD as a module
	// grows.
	NCCD float64 `json:"nccd"`
}

// PackageVisibility is one package's visibility, after Herbert Dowalil.
type PackageVisibility struct {
	Package  string `json:"package"`
	Elements int    `json:"elements"`
	Visible  int    `json:"visible"`
	// RV is Visible ÷ Elements.
	RV float64 `json:"rv"`
}

// Visibility holds Dowalil's visibility metrics (ArchUnit's
// VisibilityMetrics): how much of each package other packages can see.
type Visibility struct {
	Packages []PackageVisibility `json:"packages"`
	// ARV is the mean RV over packages with elements; GRV is Σ visible ÷
	// Σ elements.
	ARV float64 `json:"arv"`
	GRV float64 `json:"grv"`
}

// ComponentDependency holds ArchUnit's component dependency metrics for one
// package (ComponentDependencyMetrics). They differ from analyze's Martin
// metrics in two ways, recorded among the assumptions: abstractness counts
// exported types only, and a package with no dependencies either way has an
// instability of 1.
type ComponentDependency struct {
	Package            string  `json:"package"`
	Ca                 int     `json:"ca"`
	Ce                 int     `json:"ce"`
	Instability        float64 `json:"instability"`
	Abstractness       float64 `json:"abstractness"`
	NormalizedDistance float64 `json:"normalizedDistance"`
}

// lakosOf computes the Lakos metrics of one graph.
func lakosOf(level string, g arch.Graph) Lakos {
	c := arch.CouplingOf(g)
	l := Lakos{Level: level, Nodes: c.Nodes, CCD: c.CCD, ACD: c.ACD, RACD: c.PropagationCost}
	if tree := balancedCCD(c.Nodes); tree > 0 {
		l.NCCD = round6(float64(c.CCD) / float64(tree))
	}
	return l
}

// balancedCCD is the CCD of a balanced binary tree of n nodes:
// Σᵢ₌₁ⁿ (⌊log₂ i⌋ + 1), the depth of node i counting itself.
func balancedCCD(n int) int {
	total := 0
	for i := 1; i <= n; i++ {
		total += bits.Len(uint(i))
	}
	return total
}

// visibilityOf computes the visibility of every package. An element is a
// package-level type or a function that is not a method; it is visible when
// exported. An internal package stays visible to its parent's tree, which an
// analysis of one module mostly is, so being internal does not lower it.
func visibilityOf(p *arch.Program) Visibility {
	elements, visible := elementsOf(p)
	v := Visibility{Packages: make([]PackageVisibility, 0, len(p.Packages))}
	allElements, allVisible, withElements := 0, 0, 0
	sumRV := 0.0
	for _, pkg := range p.Packages {
		pv := PackageVisibility{
			Package:  pkg.Path,
			Elements: elements[pkg.Path],
			Visible:  visible[pkg.Path],
		}
		if pv.Elements > 0 {
			pv.RV = round6(float64(pv.Visible) / float64(pv.Elements))
			sumRV += pv.RV
			withElements++
		}
		allElements += pv.Elements
		allVisible += pv.Visible
		v.Packages = append(v.Packages, pv)
	}
	if withElements > 0 {
		v.ARV = round6(sumRV / float64(withElements))
	}
	if allElements > 0 {
		v.GRV = round6(float64(allVisible) / float64(allElements))
	}
	return v
}

// elementsOf counts each package's elements, and the visible ones among them.
func elementsOf(p *arch.Program) (elements, visible map[string]int) {
	elements, visible = map[string]int{}, map[string]int{}
	count := func(pkg, name string) {
		elements[pkg]++
		if isExported(name) {
			visible[pkg]++
		}
	}
	for i := range p.Types {
		count(p.Types[i].Package, p.Types[i].Name)
	}
	for i := range p.Funcs {
		if f := &p.Funcs[i]; f.Receiver == "" && f.Name != "init" && f.Name != "_" {
			count(f.Package, f.Name)
		}
	}
	return elements, visible
}

// componentDependencyOf computes ArchUnit's component dependency metrics of
// every package over in-scope imports.
func componentDependencyOf(p *arch.Program, g *arch.Graphs) []ComponentDependency {
	exported, interfaces := map[string]int{}, map[string]int{}
	for i := range p.Types {
		if t := &p.Types[i]; isExported(t.Name) {
			exported[t.Package]++
			if t.Kind == arch.KindInterface {
				interfaces[t.Package]++
			}
		}
	}
	ca, ce := map[string]int{}, map[string]int{}
	for _, e := range g.Packages.Edges() {
		ca[e.To]++
		ce[e.From]++
	}
	out := make([]ComponentDependency, 0, len(p.Packages))
	for _, pkg := range p.Packages {
		c := ComponentDependency{
			Package:     pkg.Path,
			Ca:          ca[pkg.Path],
			Ce:          ce[pkg.Path],
			Instability: 1,
		}
		if c.Ca+c.Ce > 0 {
			c.Instability = round6(float64(c.Ce) / float64(c.Ca+c.Ce))
		}
		if n := exported[pkg.Path]; n > 0 {
			c.Abstractness = round6(float64(interfaces[pkg.Path]) / float64(n))
		}
		d := c.Abstractness + c.Instability - 1
		if d < 0 {
			d = -d
		}
		c.NormalizedDistance = round6(d)
		out = append(out, c)
	}
	return out
}

// isExported reports a Go identifier that begins with an upper-case letter.
func isExported(name string) bool { return name != "" && name[0] >= 'A' && name[0] <= 'Z' }

// round6 rounds to six decimal places, so a ratio prints the same everywhere.
func round6(x float64) float64 {
	const scale = 1e6
	return float64(int64(x*scale+0.5)) / scale
}
