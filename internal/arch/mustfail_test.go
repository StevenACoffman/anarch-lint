package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestStructuralRulesFail shows each structural rule that no fixture breaks
// failing on a program built to break it, and passing on one that does not
// (IMPROVEMENTS.md §8.2).
func TestStructuralRulesFail(t *testing.T) {
	t.Parallel()
	pkgs := []arch.Package{{Path: "m/a"}, {Path: "m/b"}}
	twoSubsystems := []arch.Subsystem{
		{Name: "a", Packages: []string{"m/a"}, Interface: []string{"m/a"}},
		{Name: "b", Packages: []string{"m/b"}, Interface: []string{"m/b"}},
	}
	importOf := func(from, to string) []arch.Import {
		return []arch.Import{{From: from, To: to, Pos: arch.Position{File: "f.go", Line: 3}}}
	}
	fn := func(cc, cognitive int) []arch.Func {
		return []arch.Func{
			{
				ID:      "m/a.F",
				Package: "m/a",
				Name:    "F",
				Span:    arch.Span{File: "a/f.go", Start: 1, End: 9},
				Complexity: arch.Complexity{
					Plain:    cc,
					Modified: cc,
					Extended: cc,
				},
				Cognitive: cognitive,
			},
		}
	}
	cases := map[string]struct {
		rule    string
		program arch.Program
		model   arch.Model
		want    int
	}{
		"a package two subsystems claim alike": {
			rule:    arch.RuleAmbiguousPackage,
			program: arch.Program{Packages: pkgs},
			model: arch.Model{Subsystems: []arch.Subsystem{
				{Name: "x", Packages: []string{"m/a"}}, {Name: "y", Packages: []string{"m/a"}},
			}},
			want: 1,
		},
		"an import up the subsystem order": {
			rule:    arch.RuleSubsystemOrder,
			program: arch.Program{Packages: pkgs, Imports: importOf("m/b", "m/a")},
			model:   arch.Model{Subsystems: twoSubsystems, SubsystemOrder: []string{"a", "b"}},
			want:    1,
		},
		"an import down the subsystem order": {
			rule:    arch.RuleSubsystemOrder,
			program: arch.Program{Packages: pkgs, Imports: importOf("m/a", "m/b")},
			model:   arch.Model{Subsystems: twoSubsystems, SubsystemOrder: []string{"a", "b"}},
		},
		"a function above the complexity threshold": {
			rule:    arch.RuleComplexFunc,
			program: arch.Program{Packages: pkgs, Funcs: fn(16, 0)},
			want:    1,
		},
		"a function at the complexity threshold": {
			rule:    arch.RuleComplexFunc,
			program: arch.Program{Packages: pkgs, Funcs: fn(15, 0)},
		},
		"a function above the cognitive threshold": {
			rule:    arch.RuleCognitive,
			program: arch.Program{Packages: pkgs, Funcs: fn(1, 16)},
			want:    1,
		},
		"a function at the cognitive threshold": {
			rule:    arch.RuleCognitive,
			program: arch.Program{Packages: pkgs, Funcs: fn(1, 15)},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc.model.Version = 1
			r := arch.Analyze(&arch.Input{Program: tc.program, Model: tc.model})
			equals(t, tc.want, len(findingsOf(&r, tc.rule)))
		})
	}
}
