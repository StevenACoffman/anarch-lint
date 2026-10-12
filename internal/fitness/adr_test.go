package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// docs/adr/0003: an element is visible when exported, and an internal
// package does not hide it.
func TestADR_0003_VisibilityCountsExportedElements(t *testing.T) {
	t.Parallel()
	r := fitness.Analyze(&arch.Input{
		Model: arch.Model{Version: 1},
		Program: arch.Program{
			Packages: []arch.Package{{Path: "m/internal/core", Name: "core"}},
			Types: []arch.Type{
				{ID: "m/internal/core.Shown", Package: "m/internal/core", Name: "Shown"},
				{ID: "m/internal/core.hidden", Package: "m/internal/core", Name: "hidden"},
			},
			Funcs: []arch.Func{
				{ID: "m/internal/core.Run", Package: "m/internal/core", Name: "Run"},
				{
					ID: "m/internal/core.Shown.Method", Package: "m/internal/core",
					Name: "Method", Receiver: "m/internal/core.Shown",
				},
			},
		},
	})
	equals(t, []fitness.PackageVisibility{
		{Package: "m/internal/core", Elements: 3, Visible: 2, RV: 0.666667},
	}, r.Visibility.Packages)
}

// docs/adr/0004: a rule that matches nothing is a finding, unless it is
// optional.
func TestADR_0004_EmptyRuleFailsUnlessOptional(t *testing.T) {
	t.Parallel()
	for optional, want := range map[bool][]string{
		false: {"empty-selector nothing "},
		true:  nil,
	} {
		m := layered(&arch.Fitness{Dependents: []arch.Dependents{{
			RuleOptions: arch.RuleOptions{Name: "nothing", Reason: "r", Optional: optional},
			Packages:    []string{"m/zzz/..."},
		}}})
		r := analyze(&m)
		equals(t, want, rules(&r))
	}
}
