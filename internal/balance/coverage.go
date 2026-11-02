package balance

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Thresholds of an untested dependency, listed among the assumptions: a
// package at least minUntestedCa packages import, less than minCoveredShare
// of whose statements its tests run.
const (
	minUntestedCa   = 3
	minCoveredShare = 0.5
)

// PackageCoverage is the statement coverage of one package with its
// afferent coupling: a change to a package many import, which tests rarely
// run, can break its dependents unnoticed.
type PackageCoverage struct {
	Package    string  `json:"package"`
	Ca         int     `json:"ca"`
	Statements int     `json:"statements"`
	Covered    int     `json:"covered"`
	Share      float64 `json:"share"`
}

// coverageOf sums the profile's coverage by package and returns the packages
// it instruments, by Ca × uncovered statements, then by path, at most
// maxListed of them; it is empty without a profile.
func coverageOf(cov *arch.Coverage, p *arch.Program, packages []PackageBalance) []PackageCoverage {
	out := make([]PackageCoverage, 0)
	if cov == nil {
		return out
	}
	pkgOf := map[string]string{}
	for i := range p.Funcs {
		pkgOf[p.Funcs[i].ID] = p.Funcs[i].Package
	}
	byPkg := map[string]*PackageCoverage{}
	for _, pb := range packages {
		byPkg[pb.Path] = &PackageCoverage{Package: pb.Path, Ca: pb.Ca}
	}
	for _, fc := range cov.Funcs {
		if pc := byPkg[pkgOf[fc.Func]]; pc != nil {
			pc.Statements += fc.Statements
			pc.Covered += fc.Covered
		}
	}
	for _, pb := range packages {
		if pc := byPkg[pb.Path]; pc.Statements > 0 {
			pc.Share = round6(float64(pc.Covered) / float64(pc.Statements))
			out = append(out, *pc)
		}
	}
	slices.SortFunc(out, func(a, b PackageCoverage) int {
		return cmp.Or(cmp.Compare(b.Ca*(b.Statements-b.Covered), a.Ca*(a.Statements-a.Covered)),
			cmp.Compare(a.Package, b.Package))
	})
	return out[:min(len(out), maxListed)]
}

// untestedDependencies returns a candidate for each package many import
// whose tests run less than minCoveredShare of its statements. A profile may
// leave out tests that exist, so it is a candidate, never a finding.
func untestedDependencies(pcs []PackageCoverage) []Candidate {
	var out []Candidate
	for _, pc := range pcs {
		if pc.Ca < minUntestedCa || pc.Share >= minCoveredShare {
			continue
		}
		out = append(out, fingerprinted(&Candidate{
			Rule:    arch.RuleUntestedDependency,
			Subject: pc.Package,
			Message: fmt.Sprintf("%d packages import %s, whose tests run %d of its %d statements",
				pc.Ca, pc.Package, pc.Covered, pc.Statements),
			Fix: "test the behaviour its dependents rely on before changing it, or narrow " +
				"what they import from it",
			Evidence: []string{fmt.Sprintf("coverage %.0f%%, Ca %d", 100*pc.Share, pc.Ca)},
		}))
	}
	return out
}

// coverageAssumption records the coverage thresholds.
func coverageAssumption() arch.Assumption {
	return arch.Assumption{
		ID: "coverage",
		Text: fmt.Sprintf("Coverage attributes each block of a go test -coverprofile profile to "+
			"the function holding its first line. A package at least %d packages import, whose "+
			"tests run less than %.0f%% of its statements, is an untested-dependency candidate.",
			minUntestedCa, 100*minCoveredShare),
		Source: "IMPROVEMENTS.md 6.4 (codegrapher coverage)",
	}
}
