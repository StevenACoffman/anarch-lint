package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// TestBoundariesRule: a changed line of a file the rule names that imports
// or uses a package it keeps out is a violation, once per line and
// package; the same use on an unchanged line, or in another file, is not.
func TestBoundariesRule(t *testing.T) {
	t.Parallel()
	site := func(file string, line int) arch.Site {
		return arch.Site{File: file, Line: line, Package: "m/web"}
	}
	program := arch.Program{
		Packages: []arch.Package{{Path: "m/legacy"}, {Path: "m/store"}, {Path: "m/web"}},
		Imports: []arch.Import{
			{From: "m/web", To: "m/legacy", Pos: arch.Position{File: "web/a.go", Line: 3}},
			{From: "m/web", To: "m/store", Pos: arch.Position{File: "web/a.go", Line: 4}},
		},
		Facts: arch.Facts{Uses: []arch.Use{
			{Object: "m/legacy.Get", Package: "m/legacy", Count: 1, Site: site("web/a.go", 10)},
			{Object: "m/legacy.Put", Package: "m/legacy", Count: 1, Site: site("web/a.go", 10)},
			{Object: "m/legacy.Get", Package: "m/legacy", Count: 1, Site: site("web/a.go", 20)},
			{Object: "m/store.Get", Package: "m/store", Count: 1, Site: site("web/a.go", 11)},
			{Object: "m/legacy.Get", Package: "m/legacy", Count: 1, Site: site("cmd/main.go", 5)},
		}},
	}
	rule := arch.BoundaryRule{
		RuleOptions: arch.RuleOptions{Name: "retire-legacy", Reason: "r"},
		Files:       []string{"web/**"}, Uses: []string{"m/legacy"},
	}
	cases := map[string]struct {
		changed  arch.ChangedLines
		status   fitness.RuleStatus
		matched  int
		findings []string
	}{
		"no change set": {status: fitness.StatusNoChange},
		"a changed use and import": {
			changed: arch.ChangedLines{
				"web/a.go":    {{Start: 3, End: 3}, {Start: 10, End: 11}},
				"cmd/main.go": {{Start: 5, End: 5}},
			},
			status: fitness.StatusFailed, matched: 1,
			findings: []string{
				"boundary-crossed web/a.go: m/legacy.Get retire-legacy",
				"boundary-crossed web/a.go: the import m/legacy retire-legacy",
			},
		},
		"unchanged uses": {
			changed: arch.ChangedLines{"web/a.go": {{Start: 12, End: 19}}},
			status:  fitness.StatusPassed, matched: 1,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := arch.Model{Fitness: arch.Fitness{Boundaries: []arch.BoundaryRule{rule}}}
			r := fitness.Analyze(&arch.Input{Program: program, Model: m, Changed: tc.changed})
			equals(t, tc.findings, rules(&r))
			equals(t, tc.status, r.Rules[0].Status)
			equals(t, tc.matched, r.Rules[0].Matched)
		})
	}
}
