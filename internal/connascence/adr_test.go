package connascence_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
)

// docs/adr/0002: the gate ratchets the degree of meaning and stronger at a
// subsystem boundary or beyond, and lets name and type grow.
func TestADR_0002_DegreeRatchetSkipsNameAndType(t *testing.T) {
	t.Parallel()
	instance := func(fp string, kind arch.Strength, at arch.Locality) arch.Connascence {
		return arch.Connascence{Fingerprint: fp, Kind: kind, Locality: at, Degree: 3}
	}
	r := connascence.Report{Instances: []arch.Connascence{
		instance("name", arch.StrengthName, arch.LocalitySystem),
		instance("type", arch.StrengthType, arch.LocalitySubsystem),
		instance("meaning-in-package", arch.StrengthMeaning, arch.LocalityPackage),
		instance("meaning", arch.StrengthMeaning, arch.LocalitySubsystem),
		instance("algorithm", arch.StrengthAlgorithm, arch.LocalityExternal),
	}}
	var in arch.GateInput
	r.AddTo(&in)
	var ratcheted []string
	for _, m := range in.Metrics {
		ratcheted = append(ratcheted, m.Name)
	}
	equals(t, []string{"connascence.degree/algorithm", "connascence.degree/meaning"}, ratcheted)
}

// A candidate's slot shows a judge what the candidate is, and keeps the
// limits it exceeds, which say what a confirmation sets off, in the context
// (docs/adr/0008).
func TestCandidateSlotHidesTheLimits(t *testing.T) {
	t.Parallel()
	slot := analyze(model(true)).Judgements[0]
	for _, x := range slot.JudgeView().Exhibits {
		if strings.Contains(x.Text, "maxStrength") {
			t.Errorf("exhibit %s names a limit: %s", x.ID, x.Text)
		}
	}
	equals(t, 1, len(slot.Context))
	equals(t, true, strings.HasPrefix(slot.Context[0], "over the limits: above maxStrength."))
}

// docs/adr/0013: within a group of one body shape, copies in two or more
// packages are a fact with no slot, and the bodies that share only the shape
// stay a candidate; the part that keeps the group's first function keeps its
// fingerprint.
func TestADR_0013_CopiesAreFactsAndShapesCandidates(t *testing.T) {
	t.Parallel()
	fn := func(id, pkg, exact string) arch.Func {
		return arch.Func{
			ID: id, Package: pkg, BodyHash: "shape", ExactHash: exact,
			Span: arch.Span{File: pkg + ".go", Start: 1, End: 20},
		}
	}
	shapeOnly := func(fs ...arch.Func) []arch.Connascence {
		p := arch.Program{Funcs: fs, Packages: []arch.Package{
			{Path: "m/auth", Module: "m"},
			{Path: "m/billing", Module: "m"},
			{Path: "m/web", Module: "m"},
		}}
		var out []arch.Connascence
		for _, c := range connascence.Instances(&p, model(true)) {
			if c.Detector == "algorithm.clone" {
				out = append(out, c)
			}
		}
		return out
	}
	before := shapeOnly(fn("m/auth.A", "m/auth", "x"), fn("m/web.C", "m/web", "z"))
	after := shapeOnly(
		fn("m/auth.A", "m/auth", "x"), fn("m/billing.B", "m/billing", "x"),
		fn("m/web.C", "m/web", "z"), fn("m/billing.D", "m/billing", "y"),
	)
	if len(before) != 1 || len(after) != 2 {
		t.Fatalf(
			"want one group before and two parts after, got %d and %d",
			len(before),
			len(after),
		)
	}
	copies, shapes := after[0], after[1]
	if copies.Basis != arch.BasisFact {
		copies, shapes = shapes, copies
	}
	equals(t, arch.BasisFact, copies.Basis)
	equals(t, "m/auth.A", copies.Subject)
	equals(t, before[0].Fingerprint, copies.Fingerprint)
	equals(t, arch.BasisCandidate, shapes.Basis)
	equals(t, "m/billing.D", shapes.Subject)
}
