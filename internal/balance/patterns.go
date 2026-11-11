package balance

import (
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Thresholds of the hotspot patterns, adapted to packages from Mo, Cai,
// Kazman and Xiao's file-level definitions (WICSA 2015, TSE 2021) and listed
// among the assumptions: a package needs this many importers under analysis
// to be an interface, and this many imports too to be a crossing; an
// interface is unstable when it changed with at least this share of its
// importers, each in at least minShared commits.
const (
	minDependents   = 3
	minDependencies = 3
	unstableShare   = 0.5
)

// packageHistory is the import graph and the co-change of the packages under
// analysis that the hotspot patterns read.
type packageHistory struct {
	importers, imports map[string][]string
	// shared counts the commits of the range, sweeps left out, that changed
	// both packages of a pair, keyed in sorted order.
	shared map[[2]string]int
}

// hotspotPatterns returns the Unstable Interface and Crossing candidates of
// the revision range: packages that many others import and that change with
// them. They may be coincidence, so they are candidates, never findings;
// nothing without a range.
func (h *history) hotspotPatterns() []Candidate {
	if h.churn == nil {
		return nil
	}
	ph := h.packageHistory()
	var out []Candidate
	for _, pkg := range h.g.Packages.Nodes() {
		if c := ph.unstableInterface(pkg); c != nil {
			out = append(out, fingerprinted(c))
		}
		if c := ph.crossing(pkg); c != nil {
			out = append(out, fingerprinted(c))
		}
	}
	return out
}

// packageHistory reads the package graph and the commits of the range.
func (h *history) packageHistory() *packageHistory {
	ph := &packageHistory{
		importers: map[string][]string{}, imports: map[string][]string{},
		shared: map[[2]string]int{},
	}
	for _, e := range h.g.Packages.Edges() {
		if e.From != e.To {
			ph.imports[e.From] = append(ph.imports[e.From], e.To)
			ph.importers[e.To] = append(ph.importers[e.To], e.From)
		}
	}
	for _, pkgs := range h.packagesByCommit() {
		for i, a := range pkgs {
			for _, b := range pkgs[i+1:] {
				ph.shared[[2]string{a, b}]++
			}
		}
	}
	return ph
}

// packagesByCommit returns, for each commit of the range that is no sweep,
// the packages under analysis whose files it changed, sorted.
func (h *history) packagesByCommit() [][]string {
	pkgOf := map[string]string{}
	for _, pkg := range h.p.Packages {
		for _, f := range pkg.Files {
			pkgOf[f] = pkg.Path
		}
	}
	files, touched := map[string]int{}, map[string][]string{}
	for file, hashes := range h.churn.Commits {
		pkg, ok := pkgOf[file]
		for _, c := range hashes {
			files[c]++
			if ok {
				touched[c] = append(touched[c], pkg)
			}
		}
	}
	var out [][]string
	for c, pkgs := range touched {
		if files[c] <= maxCommitFiles {
			slices.Sort(pkgs)
			out = append(out, slices.Compact(pkgs))
		}
	}
	return out
}

// coChanged returns those of others that changed with pkg in at least
// minShared commits, each with the count, sorted.
func (ph *packageHistory) coChanged(pkg string, others []string) []string {
	var out []string
	for _, o := range slices.Sorted(slices.Values(others)) {
		key := [2]string{min(pkg, o), max(pkg, o)}
		if n := ph.shared[key]; n >= minShared {
			out = append(out, fmt.Sprintf("%s (%d commits)", o, n))
		}
	}
	return out
}

// unstableInterface returns the candidate of a package that at least
// minDependents packages import and that changed with at least unstableShare
// of them: its interface does not hold still, so its importers change when
// it does. nil for any other package.
func (ph *packageHistory) unstableInterface(pkg string) *Candidate {
	importers := ph.importers[pkg]
	with := ph.coChanged(pkg, importers)
	if len(importers) < minDependents ||
		float64(len(with)) < unstableShare*float64(len(importers)) {
		return nil
	}
	return &Candidate{
		Rule:    arch.RuleUnstableInterface,
		Subject: pkg,
		Message: fmt.Sprintf("%s changed with %d of the %d packages that import it: "+
			"its importers change when it does", pkg, len(with), len(importers)),
		Fix: "find what its importers keep changing for, and hide that decision behind " +
			"a narrower interface that can hold still",
		Evidence: append([]string{"changed with its importers:"}, with...),
	}
}

// crossing returns the candidate of a package that at least minDependents
// packages import, that imports at least minDependencies, and that changed
// with both an importer and an import: changes cross it in both directions.
// nil for any other package.
func (ph *packageHistory) crossing(pkg string) *Candidate {
	importers, imports := ph.importers[pkg], ph.imports[pkg]
	up, down := ph.coChanged(pkg, importers), ph.coChanged(pkg, imports)
	if len(importers) < minDependents || len(imports) < minDependencies ||
		len(up) == 0 || len(down) == 0 {
		return nil
	}
	evidence := append([]string{"changed with importers:"}, up...)
	evidence = append(append(evidence, "changed with imports:"), down...)
	return &Candidate{
		Rule:    arch.RuleCrossing,
		Subject: pkg,
		Message: fmt.Sprintf("%s is imported by %d packages, imports %d, and changed with "+
			"both: changes cross it", pkg, len(importers), len(imports)),
		Fix:      "split what its importers need from what it needs, so a change on one side stops at it",
		Evidence: evidence,
	}
}
