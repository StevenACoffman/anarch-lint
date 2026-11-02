package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestRules: the catalog names every rule once, each with the report that
// has it and a summary.
func TestRules(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, r := range arch.Rules() {
		if seen[r.Name] {
			t.Fatalf("rule %s is listed twice", r.Name)
		}
		seen[r.Name] = true
		if r.Summary == "" {
			t.Fatalf("rule %s has no summary", r.Name)
		}
		switch r.Report {
		case "analyze", "fitness", "connascence", "balance":
		default:
			t.Fatalf("rule %s names unknown report %q", r.Name, r.Report)
		}
	}
}

func TestRuleNamed(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		name   string
		report string // "" when there is no such rule
	}{
		"analyze":         {name: arch.RuleLayerOrder, report: "analyze"},
		"fitness":         {name: arch.RuleLayerAccess, report: "fitness"},
		"connascence":     {name: arch.StrengthMeaning.Rule(), report: "connascence"},
		"contranascence":  {name: "connascence-contranascence", report: "connascence"},
		"balance":         {name: arch.TightCouplingRule("intrusive"), report: "balance"},
		"unknown":         {name: "no-such-rule"},
		"a family prefix": {name: "tight-coupling-"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, ok := arch.RuleNamed(tc.name)
			equals(t, tc.report != "", ok)
			equals(t, tc.report, r.Report)
		})
	}
}
