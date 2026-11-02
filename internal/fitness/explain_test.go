package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// TestExplainImport: a fitness rule explains an import it forbids, and one it
// allows by name; a rule that does neither is not listed.
func TestExplainImport(t *testing.T) {
	t.Parallel()
	m := layered(&arch.Fitness{LayerAccess: []arch.LayerAccess{
		{
			RuleOptions:   arch.RuleOptions{Name: "web"},
			Layer:         "web",
			MayOnlyAccess: []string{"service"},
		},
	}})
	in := arch.Input{Program: program(), Model: m}
	cases := map[string]struct {
		from, to string
		want     []arch.Outcome
	}{
		"allowed by name": {
			from: "m/a/web",
			to:   "m/a/service",
			want: []arch.Outcome{arch.OutcomeAllows},
		},
		"forbidden": {
			from: "m/a/web",
			to:   "m/a/store",
			want: []arch.Outcome{arch.OutcomeForbids},
		},
		"not examined": {from: "m/a/service", to: "m/a/store"},
		"not imported": {from: "m/b/web", to: "m/a/store"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var got []arch.Outcome
			for _, r := range fitness.ExplainImport(&in, tc.from, tc.to) {
				equals(t, "fitness.layerAccess web", r.ModelRule)
				got = append(got, r.Outcome)
			}
			equals(t, tc.want, got)
		})
	}
}
