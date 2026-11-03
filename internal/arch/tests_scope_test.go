package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestImportRulesTestScope: a forbidden import or an isolation checks the
// imports of test files only when it opts in with tests, and its finding
// says the import is a test's and keeps apart from one about the package.
func TestImportRulesTestScope(t *testing.T) {
	t.Parallel()
	p := arch.Program{
		Packages: []arch.Package{{Path: "m/a"}, {Path: "m/a/internal"}, {Path: "m/b"}},
		Imports:  []arch.Import{{From: "m/b", To: "m/a"}},
		TestImports: []arch.Import{{
			From: "m/b", To: "m/a/internal",
			Pos: arch.Position{File: "b/b_test.go", Line: 5}, Test: true,
		}},
	}
	cases := map[string]struct {
		tests    bool
		findings []string
	}{
		"without tests": {},
		"with tests": {
			tests: true,
			findings: []string{
				"forbidden-import m/b m/a/internal (test): m/b in a test imports m/a/internal: r",
				"isolation-violation m/b m/a/internal (test): m/b in a test imports m/a/internal: r",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := arch.Model{
				Version: 1,
				ForbiddenImports: []arch.ForbiddenImport{{
					From: []string{"m/b"}, Imports: []string{"m/a/internal"}, Reason: "r",
					Tests: tc.tests,
				}},
				Isolations: []arch.Isolation{{
					From: "m/b", MayImport: []string{"m/a"}, Reason: "r", Tests: tc.tests,
				}},
			}
			r := arch.Analyze(&arch.Input{Program: p, Model: m})
			var got []string
			for _, f := range r.Findings {
				if f.Rule == arch.RuleForbiddenImport || f.Rule == arch.RuleIsolation {
					got = append(got, f.Rule+" "+f.Subject+" "+f.Object+": "+f.Message)
				}
			}
			equals(t, tc.findings, got)
		})
	}
}
