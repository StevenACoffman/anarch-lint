package fitness_test

import (
	"maps"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// A rule counts only what its own check cannot see through, and only in the
// packages whose code it reads: a reflection lookup hides a call but no
// import, and one outside the rule's packages hides nothing from it.
func TestUnseen(t *testing.T) {
	t.Parallel()
	site := func(pkg string, line int) arch.Site {
		return arch.Site{Package: pkg, File: pkg[2:] + "/x.go", Line: line}
	}
	p := arch.Program{
		Packages: []arch.Package{{Path: "m/a"}, {Path: "m/b"}},
		Facts: arch.Facts{
			ReflectByNames: []arch.ReflectByName{
				{Method: "reflect.Value.MethodByName", Site: site("m/a", 3)},
				{Method: "reflect.Value.MethodByName", Site: site("m/b", 4)},
			},
			Linknames: []arch.Linkname{{Local: "now", Target: "time.now", Site: site("m/a", 9)}},
		},
		Imports: []arch.Import{
			{From: "m/a", To: "plugin", Pos: arch.Position{File: "a/x.go", Line: 1}},
			{From: "m/b", To: "C", Pos: arch.Position{File: "b/x.go", Line: 1}, Inactive: true},
		},
	}
	m := arch.Model{Version: 1, Fitness: arch.Fitness{
		Calls: []arch.CallRule{{
			RuleOptions: arch.RuleOptions{Name: "a-calls", Reason: "r"},
			From:        []string{"m/a"}, Callees: []string{"time.Sleep"},
		}},
		FanOut: []arch.FanOutRule{{
			RuleOptions: arch.RuleOptions{Name: "a-fan-out", Reason: "r"},
			Packages:    []string{"m/a"}, Max: 5,
		}},
	}}
	r := fitness.Analyze(&arch.Input{Program: p, Model: m})
	expected := map[string]map[string]int{
		// The inactive cgo file is not built, and m/b's lookup is outside m/a.
		"a-calls": {
			"linkname":           1,
			"plugin":             1,
			"reflection-by-name": 1,
		},
		"a-fan-out": {"linkname": 1, "plugin": 1},
	}
	for _, rule := range r.Rules {
		if !maps.Equal(expected[rule.Name], rule.Unseen) {
			t.Errorf("%s: unseen %v, expected %v", rule.Name, rule.Unseen, expected[rule.Name])
		}
	}
	if len(r.Unseen) != 4 {
		t.Errorf("report lists %d unseen constructs, expected 4: %v", len(r.Unseen), r.Unseen)
	}
}
