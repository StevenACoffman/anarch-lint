package balance

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Co-change thresholds, borrowed from gortex's cochange.Mine and listed among
// the assumptions: a commit touching more files is a sweep (a rename, a
// reformat) that says nothing about coupling; two subsystems couple in time
// when they share at least minShared commits with a score of at least
// hiddenScore.
const (
	maxCommitFiles = 40
	minShared      = 2
	hiddenScore    = 0.5
)

// CoChange is how often two subsystems changed in the same commits of the
// revision range: Shared commits changed both, and Score is Shared ÷
// √(commits of A × commits of B), 1 when every change of each was a change of
// both. It is temporal coupling, which the import graph cannot see.
type CoChange struct {
	A      string  `json:"a"`
	B      string  `json:"b"`
	Shared int     `json:"shared"`
	Score  float64 `json:"score"`
}

// Hotspot is a package that changes often and is complex: Tornhill's
// hotspot, where a change is both likely and costly. Commits counts the
// commits of the range that changed its files, and Complexity sums its
// functions' cyclomatic complexity in the model's variant.
type Hotspot struct {
	Package    string `json:"package"`
	Commits    int    `json:"commits"`
	Complexity int    `json:"complexity"`
}

// history holds what the temporal measures read: the churn of a revision
// range, nil without one, the program, its graphs, and the complexity variant
// the model chooses.
type history struct {
	churn   *arch.Churn
	p       *arch.Program
	g       *arch.Graphs
	variant arch.CCVariant
}

// commitSubsystems are the declared subsystems one commit changed, sorted,
// and whether the commit was a sweep, too large to count toward pairs.
type commitSubsystems struct {
	names []string
	sweep bool
}

// coChange returns every pair of subsystems that changed together in at
// least minShared commits of the range, by score, then by shared commits and
// names; it is empty without a range.
func (h *history) coChange() []CoChange {
	out := make([]CoChange, 0)
	if h.churn == nil {
		return out
	}
	commits := map[string]int{}
	shared := map[[2]string]int{}
	for _, subs := range h.subsystemsByCommit() {
		for i, a := range subs.names {
			commits[a]++
			if subs.sweep {
				continue
			}
			for _, b := range subs.names[i+1:] {
				shared[[2]string{a, b}]++
			}
		}
	}
	for pair, n := range shared {
		if n >= minShared {
			out = append(out, CoChange{
				A: pair[0], B: pair[1], Shared: n,
				Score: round6(float64(n) / math.Sqrt(float64(commits[pair[0]]*commits[pair[1]]))),
			})
		}
	}
	slices.SortFunc(out, func(x, y CoChange) int {
		return cmp.Or(cmp.Compare(y.Score, x.Score), cmp.Compare(y.Shared, x.Shared),
			cmp.Compare(x.A, y.A), cmp.Compare(x.B, y.B))
	})
	return out
}

// subsystemsByCommit returns, for each commit of the range, the declared
// subsystems whose files it changed.
func (h *history) subsystemsByCommit() map[string]commitSubsystems {
	subOf := h.fileSubsystems()
	files := map[string]int{}
	touched := map[string][]string{}
	for file, hashes := range h.churn.Commits {
		sub, declared := subOf[file]
		for _, c := range hashes {
			files[c]++
			if declared {
				touched[c] = append(touched[c], sub)
			}
		}
	}
	out := map[string]commitSubsystems{}
	for c, subs := range touched {
		slices.Sort(subs)
		out[c] = commitSubsystems{names: slices.Compact(subs), sweep: files[c] > maxCommitFiles}
	}
	return out
}

// fileSubsystems maps each file of a package in a declared subsystem to that
// subsystem.
func (h *history) fileSubsystems() map[string]string {
	subOf := map[string]string{}
	for _, pkg := range h.p.Packages {
		if sub, ok := h.g.SubsystemAssignment.Of[pkg.Path]; ok {
			for _, f := range pkg.Files {
				subOf[f] = sub
			}
		}
	}
	return subOf
}

// hiddenCouplings returns a candidate for each pair that changes together
// above the threshold although neither imports the other: coupling the code
// does not show, through a shared schema, a wire format or a copied
// algorithm. It may be coincidence, so it is a candidate, never a finding.
func (h *history) hiddenCouplings(cc []CoChange) []Candidate {
	var out []Candidate
	for _, c := range cc {
		if c.Score < hiddenScore ||
			h.g.Subsystems.Weight(c.A, c.B) > 0 || h.g.Subsystems.Weight(c.B, c.A) > 0 {
			continue
		}
		out = append(out, fingerprinted(&Candidate{
			Rule:    "hidden-coupling",
			Subject: c.A,
			Object:  c.B,
			Message: fmt.Sprintf("%s and %s changed together in %d commits (score %g), "+
				"yet neither imports the other", c.A, c.B, c.Shared, c.Score),
			Fix: "find what both depend on without importing it, such as a schema, a wire " +
				"format or a copied algorithm, and make it explicit in one place, or merge them",
			Evidence: []string{fmt.Sprintf("co-change from %s to %s",
				short(h.churn.Since), short(h.churn.Head))},
		}))
	}
	return out
}

// hotspots returns the packages whose commits times complexity is highest,
// at most maxListed, by that product and then by path; it is empty without a
// range.
func (h *history) hotspots() []Hotspot {
	out := make([]Hotspot, 0)
	if h.churn == nil {
		return out
	}
	complexity := map[string]int{}
	for i := range h.p.Funcs {
		fn := &h.p.Funcs[i]
		complexity[fn.Package] += fn.Complexity.Of(h.variant)
	}
	for _, pkg := range h.p.Packages {
		commits := map[string]bool{}
		for _, f := range pkg.Files {
			for _, c := range h.churn.Commits[f] {
				commits[c] = true
			}
		}
		if len(commits) > 0 && complexity[pkg.Path] > 0 {
			out = append(out, Hotspot{
				Package: pkg.Path, Commits: len(commits), Complexity: complexity[pkg.Path],
			})
		}
	}
	slices.SortFunc(out, func(x, y Hotspot) int {
		return cmp.Or(cmp.Compare(y.Commits*y.Complexity, x.Commits*x.Complexity),
			cmp.Compare(x.Package, y.Package))
	})
	return out[:min(len(out), maxListed)]
}

// coChangeAssumption records the co-change thresholds.
func coChangeAssumption() arch.Assumption {
	return arch.Assumption{
		ID: "co-change",
		Text: fmt.Sprintf("Two subsystems co-change when at least %d commits of the range changed "+
			"both; their score is shared ÷ √(commits of each). A commit changing more than %d "+
			"files is a sweep and counts toward neither pair. A pair scoring %g or more with no "+
			"import either way is a hidden-coupling candidate. A hotspot is commits × summed "+
			"cyclomatic complexity.", minShared, maxCommitFiles, hiddenScore),
		Source: "IMPROVEMENTS.md 4.1 (gortex cochange.Mine; Tornhill's hotspots)",
	}
}
