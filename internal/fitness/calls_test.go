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

// A calls rule that opts into dispatch also reads the dynamic calls: one
// that may reach a forbidden callee is a warning, which never fails the
// rule, and a dynamic call the graph resolves to nothing is unseen.
func TestCallRuleDispatch(t *testing.T) {
	t.Parallel()
	site := arch.Site{File: "a/a.go", Line: 7, Package: "m/a", Func: "m/a.Report"}
	p := arch.Program{
		Packages: []arch.Package{{Path: "m/a"}},
		Facts: arch.Facts{
			DynamicCalls:    []arch.Call{{Callee: "(*os.File).Write", Count: 1, Site: site}},
			UnresolvedCalls: []arch.Site{site},
		},
	}
	for _, dispatch := range []bool{false, true} {
		m := arch.Model{Version: 1, Fitness: arch.Fitness{Calls: []arch.CallRule{{
			RuleOptions: arch.RuleOptions{Name: "no-io", Reason: "a pure core"},
			From:        []string{"m/..."}, Callees: []string{"(*os.File).*"}, Dispatch: dispatch,
		}}}}
		r := fitness.Analyze(&arch.Input{Program: p, Model: m})
		var expected []string
		var unseen map[string]int
		if dispatch {
			expected = []string{"possible-forbidden-call m/a.Report no-io: (*os.File).Write"}
			unseen = map[string]int{"unresolved-dynamic-call": 1}
		}
		equals(t, expected, rules(&r))
		equals(t, fitness.StatusPassed, r.Rules[0].Status)
		equals(t, unseen, r.Rules[0].Unseen)
		if dispatch {
			equals(t, arch.SeverityWarning, r.Findings[0].Severity)
			equals(t, arch.ProvenanceDispatch, r.Findings[0].Provenance)
		}
	}
}

// A rule naming who alone may call warns of a dynamic call from anywhere
// else that may reach its callees.
func TestCallRestrictionDispatch(t *testing.T) {
	t.Parallel()
	site := arch.Site{File: "b/b.go", Line: 5, Package: "m/b", Func: "m/b.Send"}
	p := arch.Program{
		Packages: []arch.Package{{Path: "m/a"}, {Path: "m/b"}},
		Facts: arch.Facts{
			DynamicCalls: []arch.Call{{Callee: "(*net/http.Client).Do", Count: 1, Site: site}},
		},
	}
	m := arch.Model{Version: 1, Fitness: arch.Fitness{Calls: []arch.CallRule{{
		RuleOptions: arch.RuleOptions{Name: "only-a-sends", Reason: "one gateway", Optional: true},
		From:        []string{"m/a"}, Callees: []string{"(*net/http.Client).*"},
		Only: true, Dispatch: true,
	}}}}
	r := fitness.Analyze(&arch.Input{Program: p, Model: m})
	equals(t, []string{
		"possible-call-restriction m/b.Send only-a-sends: (*net/http.Client).Do",
	}, rules(&r))
}
