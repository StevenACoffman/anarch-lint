package arch_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// A rule set's rules follow the model's own, the sets in path order however
// the model lists them, and the merged rules change the model's hash.
func TestWithRuleSets(t *testing.T) {
	t.Parallel()
	calls := func(name string) arch.CallRule {
		return arch.CallRule{
			RuleOptions: arch.RuleOptions{Name: name, Reason: "r"},
			From:        []string{"m/..."}, Callees: []string{"time.Sleep"},
		}
	}
	m := arch.Model{Version: 1, Fitness: arch.Fitness{Calls: []arch.CallRule{calls("own")}}}
	sets := []arch.RuleSet{
		{Path: "z.yaml", Fitness: arch.RuleSetFitness{Calls: []arch.CallRule{calls("z")}}},
		{Path: "a.yaml", Fitness: arch.RuleSetFitness{Calls: []arch.CallRule{calls("a")}}},
	}
	merged, err := m.WithRuleSets(sets)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range merged.Fitness.Rules() {
		names = append(names, r.Name)
	}
	equals(t, []string{"own", "a", "z"}, names)
	equals(t, 1, len(m.Fitness.Calls))
	if merged.Hash() == m.Hash() {
		t.Fatal("the merged rules do not change the hash")
	}
}

// A rule set may not name a rule as the model does, nor leave a fitness rule
// unnamed, nor hold a rule that names the model's own layers.
func TestRuleSetProblems(t *testing.T) {
	t.Parallel()
	m := arch.Model{Version: 1, Fitness: arch.Fitness{Calls: []arch.CallRule{{
		RuleOptions: arch.RuleOptions{Name: "clock", Reason: "r"},
		From:        []string{"m/..."}, Callees: []string{"time.Now"},
	}}}}
	cases := map[string]struct {
		set     arch.RuleSet
		message string
	}{
		"a duplicate name": {
			set: arch.RuleSet{Path: "s.yaml", Fitness: arch.RuleSetFitness{Calls: []arch.CallRule{{
				RuleOptions: arch.RuleOptions{Name: "clock", Reason: "r"},
				From:        []string{"m/..."}, Callees: []string{"time.Since"},
			}}}},
			message: "model: duplicate-name:",
		},
		"an unnamed rule": {
			set: arch.RuleSet{Path: "s.yaml", Fitness: arch.RuleSetFitness{Calls: []arch.CallRule{{
				RuleOptions: arch.RuleOptions{Reason: "r"},
				From:        []string{"m/..."}, Callees: []string{"time.Since"},
			}}}},
			message: "model: incomplete-rule: rule set s.yaml: rule calls[0] has no name",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := m.WithRuleSets([]arch.RuleSet{tc.set})
			if !strings.Contains(arch.ErrorMessage(err), tc.message) {
				t.Fatalf("want %q, got %v", tc.message, err)
			}
		})
	}
	_, err := arch.DecodeRuleSet(strings.NewReader(`{"fitness": {"layerAccess": []}}`), "s.json")
	if !strings.Contains(arch.ErrorMessage(err), "model: malformed: rule set s.json:") {
		t.Fatalf("want a layer rule refused, got %v", err)
	}
}
