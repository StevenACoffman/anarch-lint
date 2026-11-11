package balance_test

import (
	"cmp"
	"fmt"
	"slices"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
)

// Two subsystems that change together without importing each other are a
// hidden-coupling candidate; a pair that imports one way is not, and a sweep
// counts toward each subsystem's commits but toward no pair.
func TestCoChange(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.Program.Funcs = []arch.Func{
		{
			ID:         "m/billing.Bill",
			Package:    "m/billing",
			Name:       "Bill",
			Complexity: arch.Complexity{Modified: 3},
		},
		{
			ID:         "m/web.Serve",
			Package:    "m/web",
			Name:       "Serve",
			Complexity: arch.Complexity{Modified: 2},
		},
	}
	commits := map[string][]string{
		"auth/a.go":    {"c3", "c4"},
		"billing/b.go": {"c1", "c2", "c3", "c4", "c5"},
		"web/w.go":     {"c1", "c2", "c5"},
	}
	for i := range 41 { // c5 also changes 41 other files: a sweep
		commits[fmt.Sprintf("other/f%d.go", i)] = []string{"c5"}
	}
	r := balance.Analyze(&balance.Input{
		Input: in,
		Churn: &arch.Churn{Since: "a", Head: "b", Commits: commits, Authors: map[string]string{
			"c1": "ann@x", "c2": "bob@x", "c3": "ann@x", "c4": "ann@x",
		}},
	})

	// billing has 5 commits, web 3 and auth 2; the sweep shares none. c5 has
	// no known author, which counts as none.
	equals(t, []balance.CoChange{
		{A: "auth", B: "billing", Shared: 2, Score: 0.632456},
		{A: "billing", B: "web", Shared: 2, Score: 0.516398},
	}, r.CoChange)
	var hidden []string
	for _, c := range r.Candidates {
		if c.Rule == "hidden-coupling" {
			hidden = append(hidden, c.Subject+" ↔ "+c.Object)
		}
	}
	equals(t, []string{"billing ↔ web"}, hidden) // billing imports auth
	equals(t, []balance.Hotspot{
		{Package: "m/billing", Commits: 5, Complexity: 3, Authors: 2},
		{Package: "m/web", Commits: 3, Complexity: 2, Authors: 2},
	}, r.Hotspots)
}

// Without a revision range there is no co-change and no hotspot.
func TestCoChangeNeedsARange(t *testing.T) {
	t.Parallel()
	r := balance.Analyze(&balance.Input{Input: fixture()})
	equals(t, []balance.CoChange{}, r.CoChange)
	equals(t, []balance.Hotspot{}, r.Hotspots)
}

// A coverage profile ranks the packages by Ca × uncovered statements; one
// that three or more packages import and whose tests run less than half of
// it is an untested-dependency candidate.
func TestCoverage(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.Program.Imports = append(
		in.Program.Imports,
		arch.Import{
			From: "m/cmd/client",
			To:   "m/auth",
			Pos:  arch.Position{File: "cmd/client/main.go", Line: 3},
		},
		arch.Import{
			From: "m/cmd/server",
			To:   "m/auth",
			Pos:  arch.Position{File: "cmd/server/main.go", Line: 3},
		},
	)
	slices.SortFunc(in.Program.Imports, func(a, b arch.Import) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
	in.Program.Funcs = []arch.Func{
		{ID: "m/auth.Check", Package: "m/auth", Name: "Check"},
		{ID: "m/billing.Bill", Package: "m/billing", Name: "Bill"},
	}
	cov := &arch.Coverage{Hash: "h", Funcs: []arch.FuncCoverage{
		{Func: "m/auth.Check", Statements: 10, Covered: 2},
		{Func: "m/billing.Bill", Statements: 4, Covered: 4},
	}}
	in.Coverage = cov
	r := balance.Analyze(&balance.Input{Input: in})
	equals(t, "h", r.Inputs.Coverage)
	equals(t, []balance.PackageCoverage{
		{Package: "m/auth", Ca: 3, Statements: 10, Covered: 2, Share: 0.2},
		{Package: "m/billing", Ca: 1, Statements: 4, Covered: 4, Share: 1},
	}, r.Coverage)
	var untested []string
	for _, c := range r.Candidates {
		if c.Rule == "untested-dependency" {
			untested = append(untested, c.Subject)
		}
	}
	equals(t, []string{"m/auth"}, untested)
}

// TestHotspotsAreCapped: past 20 hotspots the report lists the 20 highest
// and says how many there were.
func TestHotspotsAreCapped(t *testing.T) {
	t.Parallel()
	in := &arch.Input{}
	commits := map[string][]string{}
	for i := range 25 {
		pkg, file := fmt.Sprintf("m/p%02d", i), fmt.Sprintf("p%02d/a.go", i)
		in.Program.Packages = append(in.Program.Packages,
			arch.Package{Path: pkg, Name: "p", Module: "m", Files: []string{file}})
		in.Program.Funcs = append(in.Program.Funcs, arch.Func{
			ID: pkg + ".F", Package: pkg, Name: "F", Complexity: arch.Complexity{Modified: i + 1},
		})
		commits[file] = []string{"c1"}
	}
	r := balance.Analyze(&balance.Input{
		Input: in, Churn: &arch.Churn{Since: "a", Head: "b", Commits: commits},
	})
	equals(t, 20, len(r.Hotspots))
	equals(t, "m/p24", r.Hotspots[0].Package)
	equals(t, []string{"hotspots: 20 of 25"}, r.Capped)
}

// A hotspot scoring above the model's limit is a finding at its most complex
// function, an error when balance gates and a warning otherwise; a limit of
// 0, the default, reports none.
func TestHotspotFindings(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		window   arch.HistoryWindow
		gate     bool
		expected []string
	}{
		"no limit": {window: arch.HistoryWindow{Commits: 10}},
		"above the limit, gated": {
			window: arch.HistoryWindow{Commits: 10, MaxHotspot: 10}, gate: true,
			expected: []string{"hotspot m/billing error billing/b.go:4"},
		},
		"above the limit, ungated": {
			window:   arch.HistoryWindow{Commits: 10, MaxHotspot: 10},
			expected: []string{"hotspot m/billing warning billing/b.go:4"},
		},
		"under the limit": {window: arch.HistoryWindow{Commits: 10, MaxHotspot: 15}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := fixture()
			in.Model.Balance.Gate = tc.gate
			in.Model.Balance.History = tc.window
			in.Program.Funcs = []arch.Func{
				{
					ID: "m/billing.Bill", Package: "m/billing", Name: "Bill",
					Span:       arch.Span{File: "billing/b.go", Start: 4, End: 9},
					Complexity: arch.Complexity{Modified: 2},
				},
				{
					ID: "m/billing.Total", Package: "m/billing", Name: "Total",
					Span:       arch.Span{File: "billing/b.go", Start: 11, End: 12},
					Complexity: arch.Complexity{Modified: 1},
				},
			}
			r := balance.Analyze(&balance.Input{Input: in, Churn: &arch.Churn{
				Since: "a", Head: "b",
				Commits: map[string][]string{"billing/b.go": {"c1", "c2", "c3", "c4", "c5"}},
				Authors: map[string]string{"c1": "ann@x", "c2": "bob@x"},
			}})
			var got []string
			for i := range r.Findings {
				if f := &r.Findings[i]; f.Rule == arch.RuleHotspot {
					got = append(got, fmt.Sprintf("%s %s %s %s:%d", f.Rule, f.Subject, f.Severity,
						f.Location.File, f.Location.Start))
					equals(t, []string{
						"m/billing.Bill has complexity 2", "m/billing.Total has complexity 1",
						"2 authors changed it",
					}, f.Evidence)
				}
			}
			equals(t, tc.expected, got)
		})
	}
}
