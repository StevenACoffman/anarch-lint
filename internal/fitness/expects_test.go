package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// An expects rule holds when one package it names imports one it expects
// it to; without such an import, or with only a test's, the dependency is
// missing.
func TestExpects(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		imports  []arch.Import
		expected []string
	}{
		"the shell uses the core": {
			imports: []arch.Import{{From: "m/shell/cli", To: "m/core"}},
		},
		"the shell does not": {
			imports:  []arch.Import{{From: "m/shell/cli", To: "m/other"}},
			expected: []string{"expected-dependency-missing shell-uses-core "},
		},
		"only a test does": {
			imports:  []arch.Import{{From: "m/shell/cli", To: "m/core", Test: true}},
			expected: []string{"expected-dependency-missing shell-uses-core "},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := arch.Program{
				Packages: []arch.Package{
					{Path: "m/core"},
					{Path: "m/other"},
					{Path: "m/shell/cli"},
				},
				Imports: tc.imports,
			}
			m := arch.Model{Version: 1, Fitness: arch.Fitness{Expects: []arch.ExpectRule{{
				RuleOptions: arch.RuleOptions{Name: "shell-uses-core", Reason: "a thin shell"},
				From:        []string{"m/shell/..."}, To: []string{"m/core"},
			}}}}
			r := fitness.Analyze(&arch.Input{Program: p, Model: m})
			equals(t, tc.expected, rules(&r))
		})
	}
}
