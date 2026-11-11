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

// docs/adr/0014: no metric the gate ratchets reads a pair's score, so two
// reports whose pairs differ only in their scores gate alike.
func TestADR_0014_TheBalanceScoreNeverGates(t *testing.T) {
	t.Parallel()
	r := analyze(fixture(), clone())
	rescored := r
	rescored.Pairs = slices.Clone(r.Pairs)
	for i := range rescored.Pairs {
		rescored.Pairs[i].Score = 10 - rescored.Pairs[i].Score
	}
	var a, b arch.GateInput
	r.AddTo(&a)
	rescored.AddTo(&b)
	equals(t, a, b)
	for _, m := range a.Metrics {
		if strings.Contains(m.Name, "score") {
			t.Errorf("metric %s reads the score", m.Name)
		}
	}
}

// docs/adr/0016: invariants are asked about in slots of their own, so a
// domain model's invariants add to no other slot's evidence, and change no
// pair, finding or candidate.
func TestADR_0016_InvariantsAreNoOtherSlotsEvidence(t *testing.T) {
	t.Parallel()
	in := withDomains()
	in.Domains[0].Invariants = []arch.DomainInvariant{{
		ID: "one-admin", Statement: "At least one `Role` is admin.", Entity: "Role",
		PreservedBy: []string{"Role.revoke"},
	}}
	without, with := analyze(withDomains(), clone()), analyze(in, clone())
	equals(t, without.Pairs, with.Pairs)
	equals(t, without.Findings, with.Findings)
	equals(t, without.Candidates, with.Candidates)
	equals(t, without.Judgements, with.Judgements)
}

// docs/adr/0017: a hotspot above the model's limit over the history window
// is a finding, never a metric, and a change to any function it rests on
// scopes it, not only one at its location.
func TestADR_0017_HistoryGatesOverAWindowEndingAtHead(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.Model.Balance.Gate = true
	in.Model.Balance.History = arch.HistoryWindow{Commits: 10, MaxHotspot: 4}
	in.Program.Funcs = []arch.Func{
		{
			ID:      "m/billing.Bill",
			Package: "m/billing",
			Name:    "Bill",
			Span: arch.Span{
				File:  "billing/b.go",
				Start: 4,
				End:   9,
			},
			Complexity: arch.Complexity{Modified: 2},
		},
		{
			ID:      "m/billing.Total",
			Package: "m/billing",
			Name:    "Total",
			Span: arch.Span{
				File:  "billing/b.go",
				Start: 11,
				End:   12,
			},
			Complexity: arch.Complexity{Modified: 1},
		},
	}
	r := balance.Analyze(&balance.Input{Input: in, Churn: &arch.Churn{
		Since: "a", Head: "b", Commits: map[string][]string{"billing/b.go": {"c1", "c2"}},
	}})
	var gate arch.GateInput
	r.AddTo(&gate)
	for _, m := range gate.Metrics {
		equals(t, false, strings.Contains(m.Name, "hotspot"))
	}
	baseline := arch.NewBaseline(&arch.GateInput{ModelHash: gate.ModelHash})
	cases := map[string]struct {
		changed arch.ChangedLines
		isNew   bool
	}{
		"its most complex function": {
			changed: arch.ChangedLines{"billing/b.go": {{Start: 5, End: 5}}}, isNew: true,
		},
		"another function it rests on": {
			changed: arch.ChangedLines{"billing/b.go": {{Start: 12, End: 12}}}, isNew: true,
		},
		"a line outside its functions": {
			changed: arch.ChangedLines{"billing/b.go": {{Start: 20, End: 20}}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := arch.Gate(&gate, &baseline, &arch.GateOptions{Changed: tc.changed})
			found := false
			for i := range g.New {
				found = found || g.New[i].Rule == arch.RuleHotspot
			}
			equals(t, tc.isNew, found)
		})
	}
}
