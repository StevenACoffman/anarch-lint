package balance_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
)

// docs/adr/0005: what a domain model implies adds evidence and answers no
// slot, so adding domain models changes no pair, finding, candidate or
// subsystem, only the evidence reviewers judge.
func TestADR_0005_DomainInferencesAreEvidence(t *testing.T) {
	t.Parallel()
	without, with := analyze(fixture(), clone()), analyze(withDomains(), clone())
	equals(t, without.Pairs, with.Pairs)
	equals(t, without.Findings, with.Findings)
	equals(t, without.Candidates, with.Candidates)
	equals(t, without.Subsystems, with.Subsystems)
	slot, _ := slotOf(with.Judgements, "bounded-context/auth")
	equals(t, true, slices.Contains(slot.Evidence,
		"described by domain model models/auth.modelith.yaml"))
	equals(t, "", slot.Answer())
}

// A balance candidate's slot shows the coupling, and keeps the distance,
// volatility and score that make it gate in the context (docs/adr/0008).
func TestBalanceSlotHidesWhatGates(t *testing.T) {
	t.Parallel()
	r := analyze(fixture(), clone())
	var asked bool
	for _, s := range r.Judgements {
		if !strings.HasPrefix(s.Criterion, "balance/") {
			continue
		}
		asked = true
		for _, x := range s.JudgeView().Exhibits {
			if strings.Contains(x.Text, "distance") || strings.Contains(x.Text, "balance ") {
				t.Errorf("%s: exhibit %s says what gates: %s", s.Criterion, x.ID, x.Text)
			}
		}
		equals(t, true, len(s.Context) > 0)
	}
	equals(t, true, asked)
}

// docs/adr/0010: only a verdict a human attests can lift a finding that
// rests on facts, and every lift is reported with who attested it.
func TestADR_0010_SuppressingAFactNeedsAHuman(t *testing.T) {
	t.Parallel()
	in := fixture()
	pending, _ := slotOf(analyze(in).Judgements, "subdomain/auth")
	supporting := func(events ...arch.VerificationEvent) balance.Report {
		judged := *in
		judged.Verdicts = []arch.Verdict{{
			Criterion: "subdomain/auth", Answer: "supporting",
			EvidenceHash: pending.EvidenceHash, Events: events,
		}}
		return analyze(&judged)
	}
	tight := func(r *balance.Report) bool {
		return slices.ContainsFunc(r.Findings, func(f arch.Finding) bool {
			return f.Rule == "tight-coupling-intrusive"
		})
	}

	byJudge := supporting(arch.VerificationEvent{Role: arch.RoleJudge, Config: "model-v1"})
	equals(t, true, tight(&byJudge))
	equals(t, 0, len(byJudge.Suppressed))

	human := arch.VerificationEvent{Role: arch.RoleHuman, Actor: "reviewer@example.com"}
	byHuman := supporting(human)
	equals(t, false, tight(&byHuman))
	equals(t, []balance.Suppression{{
		Rule: "tight-coupling-intrusive", Subject: "web", Object: "auth",
		Criterion: "subdomain/auth", Answer: "supporting",
		Events: []arch.VerificationEvent{human},
	}}, byHuman.Suppressed)
}
