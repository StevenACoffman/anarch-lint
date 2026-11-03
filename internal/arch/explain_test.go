package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestExplainImport: each structural rule that examines an import says
// whether it allows or forbids it and why; an import that does not exist
// has no location, a rule whose gate is off is left out, and a waiver turns
// a forbidding rule into a waived one until it expires.
func TestExplainImport(t *testing.T) {
	t.Parallel()
	at := func(file string) arch.Position { return arch.Position{File: file, Line: 3} }
	in := arch.Input{
		Program: arch.Program{
			Packages: []arch.Package{{Path: "m/app"}, {Path: "m/infra"}, {Path: "m/web"}},
			Imports: []arch.Import{
				{From: "m/app", To: "m/infra", Pos: at("app/app.go")},
				{From: "m/infra", To: "m/app", Pos: at("infra/infra.go")},
				{From: "m/web", To: "m/app", Pos: at("web/web.go")},
			},
		},
		Model: arch.Model{
			Version: 1,
			Layers: []arch.Layer{
				{Name: "web", Packages: []string{"m/web"}},
				{Name: "app", Packages: []string{"m/app"}},
				{Name: "infra", Packages: []string{"m/infra"}},
			},
			ForbiddenImports: []arch.ForbiddenImport{
				{From: []string{"m/web"}, Imports: []string{"m/infra"}, Reason: "thin web"},
				{
					From:    []string{"m/web"},
					Imports: []string{"m/app"},
					Reason:  "off",
					Gate:    arch.GateOff,
				},
			},
		},
	}
	type got struct {
		rule    string
		outcome arch.Outcome
	}
	outcomes := func(e *arch.ImportExplanation) []got {
		var out []got
		for _, r := range e.Rules {
			out = append(out, got{r.ModelRule, r.Outcome})
		}
		return out
	}
	cases := map[string]struct {
		from, to string
		want     []got
	}{
		"allowed": {
			from: "m/web",
			to:   "m/app",
			want: []got{
				{"forbiddenImports[0]", arch.OutcomeAllows},
				{"layers", arch.OutcomeAllows},
			},
		},
		"forbidden": {
			from: "m/infra", to: "m/app",
			want: []got{{"layers", arch.OutcomeForbids}},
		},
		"the layer order alone": {
			from: "m/app", to: "m/infra", want: []got{{"layers", arch.OutcomeAllows}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := arch.ExplainImport(&in, tc.from, tc.to)
			assert(t, e.Location != nil, "the import exists")
			equals(t, tc.want, outcomes(&e))
		})
	}
	t.Run("missing import", func(t *testing.T) {
		t.Parallel()
		e := arch.ExplainImport(&in, "m/app", "m/web")
		equals(t, (*arch.Span)(nil), e.Location)
		equals(t, 0, len(e.Rules))
	})
	t.Run("waived", func(t *testing.T) {
		t.Parallel()
		e := arch.ExplainImport(&in, "m/infra", "m/app")
		fp := e.Rules[0].Fingerprint
		e.Waive([]arch.Waiver{
			{Fingerprint: fp, Reason: "migrating", ApprovedBy: "lead", Expires: "2026-12-31"},
		}, "2026-11-01")
		equals(t, arch.OutcomeWaived, e.Rules[0].Outcome)
		late := arch.ExplainImport(&in, "m/infra", "m/app")
		late.Waive([]arch.Waiver{
			{Fingerprint: fp, Reason: "migrating", ApprovedBy: "lead", Expires: "2026-10-31"},
		}, "2026-11-01")
		equals(t, arch.OutcomeForbids, late.Rules[0].Outcome)
	})
}

// TestWithImport: the copy has the added import, and the input is
// unchanged, so asking about an import cannot leak into what is checked.
func TestWithImport(t *testing.T) {
	t.Parallel()
	in := arch.Input{Program: arch.Program{Imports: make([]arch.Import, 1, 2)}}
	in.Program.Imports[0] = arch.Import{From: "m/a", To: "m/b"}
	out := in.WithImport("m/b", "m/a", arch.Position{File: "case c", Line: 1})
	equals(t, []arch.Import{{From: "m/a", To: "m/b"}}, in.Program.Imports)
	equals(t, []arch.Import{
		{From: "m/a", To: "m/b"},
		{From: "m/b", To: "m/a", Pos: arch.Position{File: "case c", Line: 1}},
	}, out.Program.Imports)
}
