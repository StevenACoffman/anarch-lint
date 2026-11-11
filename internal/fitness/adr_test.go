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

// TestADR_0019_APassSaysWhatItCouldNotSee pins docs/adr/0019: a dynamic call
// that may reach a forbidden callee is a warning that never fails the rule,
// a dynamic call nothing resolves is unseen, and a rule counts only the
// constructs that can hide what its kind reads.
func TestADR_0019_APassSaysWhatItCouldNotSee(t *testing.T) {
	t.Parallel()
	site := arch.Site{File: "core/c.go", Line: 4, Package: "m/core", Func: "m/core.Print"}
	p := arch.Program{
		Packages: []arch.Package{{Path: "m/core"}},
		Facts: arch.Facts{
			DynamicCalls:    []arch.Call{{Callee: "(*os.File).Write", Count: 1, Site: site}},
			UnresolvedCalls: []arch.Site{site},
			UnsafeConvs:     []arch.UnsafeConv{{Type: "m/core.T", Package: "m/core", Site: site}},
		},
	}
	m := arch.Model{Version: 1, Fitness: arch.Fitness{
		Calls: []arch.CallRule{{
			RuleOptions: arch.RuleOptions{Name: "pure-core", Reason: "r"},
			From:        []string{"m/core"}, Callees: []string{"os.*"}, Dispatch: true,
		}},
		Fields: []arch.FieldRule{{
			RuleOptions: arch.RuleOptions{Name: "sealed", Reason: "r", Optional: true},
			From:        []string{"m/core"}, Fields: []string{"m/other.T.*"},
		}},
	}}
	r := fitness.Analyze(&arch.Input{Program: p, Model: m})
	equals(t, []string{"possible-forbidden-call m/core.Print pure-core: (*os.File).Write"},
		rules(&r))
	equals(t, arch.SeverityWarning, r.Findings[0].Severity)
	unseen := map[string]map[string]int{}
	for _, rule := range r.Rules {
		equals(t, fitness.StatusPassed, rule.Status)
		unseen[rule.Name] = rule.Unseen
	}
	equals(t, map[string]map[string]int{
		"pure-core": {"unresolved-dynamic-call": 1},
		"sealed":    {"unsafe-conversion": 1},
	}, unseen)
}
