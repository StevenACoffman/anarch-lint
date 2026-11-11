package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// A calls rule reads the test files' calls only when it opts in with tests:
// the build's calls always count.
func TestCallRuleTests(t *testing.T) {
	t.Parallel()
	call := func(fn, callee string) arch.Call {
		return arch.Call{Callee: callee, Count: 1, Site: arch.Site{
			File: "a/a.go", Line: 3, Package: "m/a", Func: fn,
		}}
	}
	cases := map[string]struct {
		tests    bool
		expected []string
	}{
		"without tests": {
			expected: []string{"forbidden-call m/a.Wait no-sleep: time.Sleep"},
		},
		"with tests": {
			tests: true,
			expected: []string{
				"forbidden-call m/a.TestWait no-sleep: time.Sleep",
				"forbidden-call m/a.Wait no-sleep: time.Sleep",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := arch.Program{
				Packages:  []arch.Package{{Path: "m/a"}},
				Facts:     arch.Facts{Calls: []arch.Call{call("m/a.Wait", "time.Sleep")}},
				TestCalls: []arch.Call{call("m/a.TestWait", "time.Sleep")},
			}
			m := arch.Model{Version: 1, Fitness: arch.Fitness{Calls: []arch.CallRule{{
				RuleOptions: arch.RuleOptions{Name: "no-sleep", Reason: "wait on a signal"},
				From:        []string{"m/..."}, Callees: []string{"time.Sleep"}, Tests: tc.tests,
			}}}}
			r := fitness.Analyze(&arch.Input{Program: p, Model: m})
			equals(t, tc.expected, rules(&r))
		})
	}
}
