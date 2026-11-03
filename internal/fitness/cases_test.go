package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// TestCases: a declared import is decided by the same rules that check the
// code, whether or not the code has it; a case the rules decide otherwise
// is a violation, with every rule's decision as evidence, and a case naming
// a package not under analysis matches nothing.
func TestCases(t *testing.T) {
	t.Parallel()
	program := arch.Program{Packages: []arch.Package{
		{Path: "m/billing"}, {Path: "m/billing/internal"}, {Path: "m/shipping"}, {Path: "m/web"},
	}}
	model := arch.Model{
		Layers: []arch.Layer{
			{Name: "ui", Packages: []string{"m/web"}},
			{Name: "core", Packages: []string{"m/billing/...", "m/shipping"}},
		},
		Isolations: []arch.Isolation{{
			From: "m/billing/internal", MayImport: []string{"m/billing"}, Reason: "r",
		}},
	}
	cases := map[string]struct {
		c        arch.RuleCase
		matched  int
		findings []string
		evidence []string // the first finding's
	}{
		"forbidden by its rule": {
			c: arch.RuleCase{
				From: "m/billing/internal", Imports: "m/shipping",
				Expect: arch.ExpectForbidden, By: "isolations[0]",
			},
			matched: 1,
		},
		"forbidden by another rule than it names": {
			c: arch.RuleCase{
				From: "m/billing/internal", Imports: "m/shipping",
				Expect: arch.ExpectForbidden, By: "layers",
			},
			matched:  1,
			findings: []string{"model-case-contradicted m/billing/internal → m/shipping case"},
		},
		"allowed": {
			c: arch.RuleCase{
				From:    "m/web",
				Imports: "m/shipping",
				Expect:  arch.ExpectAllowed,
			},
			matched: 1,
		},
		"allowed, but forbidden": {
			c: arch.RuleCase{
				From:    "m/shipping",
				Imports: "m/web",
				Expect:  arch.ExpectAllowed,
			},
			matched:  1,
			findings: []string{"model-case-contradicted m/shipping → m/web case"},
			evidence: []string{
				"layers forbids it: m/shipping (core) imports m/web (ui), which sits above it",
			},
		},
		"a package not under analysis": {
			c: arch.RuleCase{
				From:    "m/admin",
				Imports: "m/web",
				Expect:  arch.ExpectForbidden,
			},
			findings: []string{"empty-selector case "},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := model
			tc.c.RuleOptions = arch.RuleOptions{Name: "case", Reason: "r"}
			m.Fitness.Cases = []arch.RuleCase{tc.c}
			r := fitness.Analyze(&arch.Input{Program: program, Model: m})
			equals(t, tc.findings, rules(&r))
			equals(t, tc.matched, r.Rules[len(r.Rules)-1].Matched)
			if tc.evidence != nil {
				equals(t, tc.evidence, r.Findings[0].Evidence)
			}
		})
	}
}
