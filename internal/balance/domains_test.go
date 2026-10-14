package balance_test

import (
	"slices"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// withDomains gives auth and billing domain models: billing's imports
// auth's, which is shared, and auth's declares Role.
func withDomains() *arch.Input {
	in := fixture()
	in.Model.Fitness.Domains = []arch.DomainRule{
		{Path: "models/auth.modelith.yaml", Subsystem: "auth"},
		{Path: "models/billing.modelith.yaml", Subsystem: "billing"},
	}
	in.Domains = []arch.DomainModel{
		{
			Path: "models/auth.modelith.yaml", Shared: true,
			Entities: []arch.DomainEntity{{Name: "Role"}},
		},
		{
			Path:    "models/billing.modelith.yaml",
			Imports: []arch.DomainImport{{Scope: "auth", Path: "auth.modelith.yaml"}},
		},
	}
	return in
}

func slotOf(slots []arch.JudgementSlot, criterion string) (arch.JudgementSlot, bool) {
	i := slices.IndexFunc(
		slots,
		func(s arch.JudgementSlot) bool { return s.Criterion == criterion },
	)
	if i < 0 {
		return arch.JudgementSlot{}, false
	}
	return slots[i], true
}

// A domain model's inferences are evidence, never answers: an entity's type
// is a model type without asking, and the models' descriptions and imports
// inform the bounded-context and integration-pattern slots.
func TestDomainEvidence(t *testing.T) {
	t.Parallel()
	_, asked := slotOf(analyze(fixture()).Judgements, "contract-type/m/auth.Role")
	equals(t, true, asked)

	in := withDomains()
	r := analyze(in)
	_, asked = slotOf(r.Judgements, "contract-type/m/auth.Role")
	equals(t, false, asked)
	equals(t, arch.BasisFact, pairOf(t, &r, "billing", "auth").Basis)

	var verdicts []arch.Verdict
	for _, sub := range []string{"auth", "billing"} {
		slot, _ := slotOf(r.Judgements, "bounded-context/"+sub)
		equals(t, true, slices.Contains(slot.Evidence,
			"described by domain model models/"+sub+".modelith.yaml"))
		verdicts = append(verdicts, arch.Verdict{
			Criterion:    slot.Criterion,
			Answer:       "yes",
			EvidenceHash: slot.EvidenceHash,
			Events:       []arch.VerificationEvent{{Role: arch.RoleHuman}},
		})
	}
	in.Verdicts = verdicts
	pattern, asked := slotOf(analyze(in).Judgements, "integration-pattern/billing->auth")
	equals(t, true, asked)
	equals(t, []string{
		"billing's domain model models/billing.modelith.yaml imports auth's " +
			"models/auth.modelith.yaml as auth",
		"auth's domain model models/auth.modelith.yaml is shared",
	}, pattern.Evidence[len(pattern.Evidence)-2:])
}
