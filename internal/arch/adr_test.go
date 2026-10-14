package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// docs/adr/0011: a change to any import of a subsystem cycle scopes the
// cycle's finding, not only a change to the import its fix cuts.
func TestADR_0011_AChangeScopesWhatRestsOnIt(t *testing.T) {
	t.Parallel()
	p := fixtureProgram()
	// billing/tax imports shop/catalog and shop/catalog imports
	// billing/invoice: billing and shop form a cycle.
	p.Imports[0].Pos = arch.Position{File: "billing/tax/tax.go", Line: 3}
	p.Imports[1].Pos = arch.Position{File: "shop/catalog/catalog.go", Line: 5}
	r := arch.Analyze(&arch.Input{Program: p, Model: fixtureModel()})
	cycle := findingsOf(&r, arch.RuleSubsystemCycle)
	equals(t, 1, len(cycle))
	baseline := arch.NewBaseline(gateInput(&r))
	baseline.Findings = nil // the cycle is new

	cases := map[string]struct {
		changed arch.ChangedLines
		isNew   bool
	}{
		"the import the fix cuts": {
			changed: arch.ChangedLines{cycle[0].Location.File: {{Start: 1, End: 9}}},
			isNew:   true,
		},
		"the other import of the cycle": {
			changed: arch.ChangedLines{
				otherFile(cycle[0].Location.File): {{Start: 1, End: 9}},
			},
			isNew: true,
		},
		"a line elsewhere in the cycle's files": {
			changed: arch.ChangedLines{
				"billing/tax/tax.go":      {{Start: 7, End: 9}},
				"shop/catalog/catalog.go": {{Start: 7, End: 9}},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := arch.Gate(gateInput(&r), &baseline, &arch.GateOptions{Changed: tc.changed})
			found := false
			for i := range g.New {
				found = found || g.New[i].Rule == arch.RuleSubsystemCycle
			}
			equals(t, tc.isNew, found)
		})
	}
}

// otherFile returns the file of the cycle's other import.
func otherFile(cut string) string {
	if cut == "billing/tax/tax.go" {
		return "shop/catalog/catalog.go"
	}
	return "billing/tax/tax.go"
}
