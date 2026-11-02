package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestRank: a finding's reach counts the packages that depend on its
// package, directly or not, found by its location, its subject, or the
// package its subject's ID starts with; a finding about the model has none.
func TestRank(t *testing.T) {
	t.Parallel()
	p := arch.Program{
		Packages: []arch.Package{{Path: "m/app"}, {Path: "m/core"}, {Path: "m/web"}},
		Files:    []arch.File{{Path: "core/c.go", Package: "m/core"}},
		Imports: []arch.Import{
			{From: "m/app", To: "m/core"}, {From: "m/web", To: "m/app"},
		},
	}
	finding := func(subject string, at *arch.Span) arch.Finding {
		f := arch.NewFinding(arch.FindingID{Rule: "r", Subject: subject}, arch.SeverityError)
		f.Location = at
		return f
	}
	in := arch.GateInput{Findings: []arch.Finding{
		finding("anything", &arch.Span{File: "core/c.go", Start: 1}),
		finding("m/app", nil),
		finding("m/core.Type.Method", nil),
		finding("layers", nil),
	}}
	in.Rank(&p)
	var reach []int
	for _, f := range in.Findings {
		reach = append(reach, f.Reach)
	}
	equals(t, []int{2, 1, 2, 0}, reach)
}
