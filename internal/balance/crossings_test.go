package balance_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
)

// TestUntestedCrossings: a changed function that calls into another
// subsystem and that no test runs is a candidate; a covered one, an
// unchanged one, or one calling within its subsystem is not.
func TestUntestedCrossings(t *testing.T) {
	t.Parallel()
	call := func(fn, callee string, line int) arch.Call {
		return arch.Call{Callee: callee, Count: 1, Site: arch.Site{
			File: "web/w.go", Line: line, Package: "m/web", Func: fn,
		}}
	}
	program := arch.Program{
		Packages: []arch.Package{
			{Path: "m/api", Name: "api", Module: "m", Files: []string{"api/a.go"}},
			{Path: "m/web", Name: "web", Module: "m", Files: []string{"web/w.go"}},
		},
		Funcs: []arch.Func{
			{
				ID:      "m/web.Handle",
				Package: "m/web",
				Name:    "Handle",
				Span:    arch.Span{File: "web/w.go", Start: 5, End: 10},
			},
			{
				ID:      "m/web.Local",
				Package: "m/web",
				Name:    "Local",
				Span:    arch.Span{File: "web/w.go", Start: 12, End: 15},
			},
		},
		Facts: arch.Facts{Calls: []arch.Call{
			call("m/web.Handle", "m/api.Get", 7),
			call("m/web.Local", "m/web.helper", 13),
		}},
	}
	model := arch.Model{Subsystems: []arch.Subsystem{
		{Name: "api", Packages: []string{"m/api"}}, {Name: "web", Packages: []string{"m/web"}},
	}}
	cases := map[string]struct {
		changed  arch.ChangedLines
		covered  int
		subjects []string
	}{
		"changed and untested": {
			changed:  arch.ChangedLines{"web/w.go": {{Start: 6, End: 13}}},
			subjects: []string{"m/web.Handle"},
		},
		"changed and tested": {
			changed: arch.ChangedLines{"web/w.go": {{Start: 6, End: 6}}},
			covered: 1,
		},
		"unchanged":     {changed: arch.ChangedLines{"web/w.go": {{Start: 20, End: 21}}}},
		"no change set": {},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := balance.Analyze(&balance.Input{Input: &arch.Input{
				Program: program, Model: model, Changed: tc.changed,
				Coverage: &arch.Coverage{Funcs: []arch.FuncCoverage{
					{Func: "m/web.Handle", Statements: 4, Covered: tc.covered},
					{Func: "m/web.Local", Statements: 2},
				}},
			}})
			var subjects []string
			for _, c := range r.Candidates {
				if c.Rule == arch.RuleUntestedCrossing {
					subjects = append(subjects, c.Subject)
				}
			}
			equals(t, tc.subjects, subjects)
		})
	}
}
