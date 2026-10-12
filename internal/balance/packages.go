package balance

import (
	"fmt"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// ruleExposure is the finding of a widely used package with a strong
// upstream dependency (connascence.md §9.6).
const ruleExposure = "strong-upstream-exposure"

// minExposureCa is the afferent coupling from which a strong upstream
// dependency is a finding: borrowed guidance, listed among the assumptions.
const minExposureCa = 10

// packageView weighs each package's coupling by integration strength.
type packageView struct {
	p        *arch.Program
	g        *arch.Graphs
	severity arch.Severity
}

// PackageBalance weighs a package's Martin metrics by integration strength
// (connascence.md §9.6).
type PackageBalance struct {
	Path string `json:"path"`
	Ca   int    `json:"ca"`
	Ce   int    `json:"ce"`
	// Upstream is the strongest level among the package's dependencies on
	// other subsystems, with whether a fact shows it.
	Upstream      Level      `json:"upstream"`
	UpstreamBasis arch.Basis `json:"upstreamBasis,omitempty"`
	// WeightedInstability is instability over cross-subsystem dependencies,
	// each weighted by its level's ordinal; reported, never gated.
	WeightedInstability float64 `json:"weightedInstability"`
	// Dependents counts the packages that reach this one through imports:
	// where a change behind its upstream dependency can cascade.
	Dependents int `json:"dependents"`
}

// packageEdge is one package's dependency on another.
type packageEdge struct{ from, to string }

// packages weighs each package's coupling by the levels of its
// cross-subsystem dependencies.
func (v *packageView) packages(ev []evidence) []PackageBalance {
	levels := edgeLevels(ev)
	ca, ce := map[string]int{}, map[string]int{}
	for _, e := range v.g.Packages.Edges() {
		ce[e.From]++
		ca[e.To]++
	}
	dependents := v.dependents()
	out := make([]PackageBalance, 0, len(v.p.Packages))
	for _, pkg := range v.p.Packages {
		pb := PackageBalance{
			Path: pkg.Path, Ca: ca[pkg.Path], Ce: ce[pkg.Path],
			Dependents: len(dependents[pkg.Path]),
		}
		pb.weigh(levels)
		out = append(out, pb)
	}
	return out
}

// edgeLevels returns the strongest evidence of each package dependency.
func edgeLevels(ev []evidence) map[packageEdge]*evidence {
	levels := map[packageEdge]*evidence{}
	for i := range ev {
		e := &ev[i]
		k := packageEdge{e.fromPkg, e.toPkg}
		if prev, ok := levels[k]; !ok || compareEvidence(e, prev) < 0 {
			levels[k] = e
		}
	}
	return levels
}

// weigh sets the package's strongest upstream level and its weighted
// instability from the levels of the dependencies in and out of it.
func (pb *PackageBalance) weigh(levels map[packageEdge]*evidence) {
	var inWeight, outWeight float64
	for k, e := range levels {
		w := float64(levelOrdinal(e.level))
		switch pb.Path {
		case k.from:
			outWeight += w
			if e.level > pb.Upstream || (e.level == pb.Upstream && e.basis == arch.BasisFact) {
				pb.Upstream, pb.UpstreamBasis = e.level, e.basis
			}
		case k.to:
			inWeight += w
		}
	}
	if outWeight+inWeight > 0 {
		pb.WeightedInstability = round6(outWeight / (outWeight + inWeight))
	}
}

// dependents maps each package to the packages that reach it.
func (v *packageView) dependents() map[string][]string {
	importers := map[string][]string{}
	for _, e := range v.g.Packages.Edges() {
		importers[e.To] = append(importers[e.To], e.From)
	}
	out := map[string][]string{}
	for _, pkg := range v.p.Packages {
		out[pkg.Path] = reachable(pkg.Path, importers)
	}
	return out
}

// exposureFindings report packages many others depend on whose own upstream
// dependency is intrusive, or functional on facts: a change behind that
// dependency can cascade to every dependent.
func (v *packageView) exposureFindings(pkgs []PackageBalance) []arch.Finding {
	dependents := v.dependents()
	var fs []arch.Finding
	for _, pb := range pkgs {
		strong := pb.Upstream == LevelIntrusive ||
			(pb.Upstream == LevelFunctional && pb.UpstreamBasis == arch.BasisFact)
		if pb.Ca < minExposureCa || !strong {
			continue
		}
		f := arch.NewFinding(
			arch.FindingID{Rule: ruleExposure, Subject: pb.Path, Object: ""},
			v.severity,
		)
		f.Message = fmt.Sprintf("%s has %d importers and a %s dependency on another subsystem; "+
			"a change behind it can reach %d packages", pb.Path, pb.Ca, pb.Upstream, pb.Dependents)
		deps := dependents[pb.Path]
		for _, d := range deps[:min(len(deps), maxListed)] {
			f.Evidence = append(f.Evidence, "dependent: "+d)
		}
		f.Fix = "lower the upstream dependency to a contract before more packages depend on this one"
		fs = append(fs, f)
	}
	return fs
}

// levelOrdinal is archfit's strength ordinal of a level, without the
// symmetric case, which a single dependency does not have.
func levelOrdinal(l Level) int {
	return strengthOrdinal(&Pair{Level: l})
}

// round6 rounds to six decimal places, so a ratio prints the same everywhere.
func round6(x float64) float64 {
	const scale = 1e6
	return float64(int64(x*scale+0.5)) / scale
}
