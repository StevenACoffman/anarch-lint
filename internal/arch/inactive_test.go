package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// The import rules check the imports of files the build constraints
// exclude, and say so in the finding's evidence; the measures do not see
// them.
func TestImportRulesCheckInactiveImports(t *testing.T) {
	t.Parallel()
	windows := arch.Position{File: "app/app_windows.go", Line: 3}
	p := arch.Program{
		Packages: []arch.Package{{Path: "fx/app"}, {Path: "fx/infra"}},
		InactiveImports: []arch.Import{
			{From: "fx/app", To: "fx/infra", Pos: windows, Inactive: true},
			{From: "fx/app", To: "os", Pos: windows, Inactive: true},
		},
	}
	m := arch.Model{
		Version: 1,
		ForbiddenImports: []arch.ForbiddenImport{
			{From: []string{"fx/app"}, Imports: []string{"os"}, Reason: "no I/O"},
		},
		Isolations: []arch.Isolation{{From: "fx/app", MayImport: []string{}, Reason: "alone"}},
	}
	r := arch.Analyze(&arch.Input{Program: p, Model: m})
	for _, rule := range []string{arch.RuleForbiddenImport, arch.RuleIsolation} {
		fs := findingsOf(&r, rule)
		if len(fs) != 1 {
			t.Fatalf("%s: want one finding, got %v", rule, fs)
		}
		equals(t, []string{arch.InactiveEvidence}, fs[0].Evidence)
		equals(t, "app/app_windows.go", fs[0].Location.File)
	}
	for _, pkg := range r.Packages {
		if pkg.Ce != 0 {
			t.Fatalf("an inactive import counted toward %s's efferent coupling", pkg.Package)
		}
	}
}

// RuleImports lists the build's imports before the inactive ones.
func TestRuleImports(t *testing.T) {
	t.Parallel()
	p := arch.Program{
		Imports:         []arch.Import{{From: "b", To: "c"}},
		InactiveImports: []arch.Import{{From: "a", To: "c", Inactive: true}},
	}
	equals(t, []arch.Import{{From: "b", To: "c"}, {From: "a", To: "c", Inactive: true}},
		p.RuleImports())
}

// A package whose own import of a forbidden package is inactive gets the
// plain finding alone, not also the transitive one of the same identity.
func TestInactiveImportIsDirectForTransitiveRules(t *testing.T) {
	t.Parallel()
	pos := arch.Position{File: "app/app.go", Line: 3}
	p := arch.Program{
		Packages: []arch.Package{{Path: "fx/app"}, {Path: "fx/infra"}},
		Imports: []arch.Import{
			{From: "fx/app", To: "fx/infra", Pos: pos},
			{From: "fx/infra", To: "os", Pos: arch.Position{File: "infra/infra.go", Line: 3}},
		},
		InactiveImports: []arch.Import{{
			From: "fx/app", To: "os", Inactive: true,
			Pos: arch.Position{File: "app/app_windows.go", Line: 3},
		}},
	}
	m := arch.Model{Version: 1, ForbiddenImports: []arch.ForbiddenImport{{
		From: []string{"fx/app"}, Imports: []string{"os"}, Reason: "no I/O", Transitive: true,
	}}}
	r := arch.Analyze(&arch.Input{Program: p, Model: m})
	fs := findingsOf(&r, arch.RuleForbiddenImport)
	if len(fs) != 1 {
		t.Fatalf("want one finding, got %d: %v", len(fs), fs)
	}
	equals(t, []string{arch.InactiveEvidence}, fs[0].Evidence)
}
