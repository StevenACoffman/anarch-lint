package arch_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// slotOf opens the slot of criterion over evidence.
func slotOf(criterion string, evidence []string, verdicts []arch.Verdict) arch.JudgementSlot {
	return arch.NewSlot(&arch.SlotRequest{Criterion: criterion, Evidence: evidence}, verdicts)
}

// TestQuestionEditionsArePinned fails when a question's words, answers or
// revision change. Update the table only after deciding that every verdict
// on the question should go stale, and turn the question's revision when
// its meaning changes without its words.
func TestQuestionEditionsArePinned(t *testing.T) {
	t.Parallel()
	pinned := map[string]string{
		"1.1.5":                 "74fd815348e4",
		"1.1.6":                 "7006f2a1eb43",
		"1.2.2":                 "e0a61ede6c68",
		"3.3":                   "bbead25ebeef",
		"3.4":                   "348152ace82b",
		"async/x":               "f9e22cdd80c3",
		"balance/x":             "9236d4c2a2f2",
		"bounded-context/x":     "9f4fe2d41b98",
		"connascence/x":         "5f293ed8552e",
		"contract-type/x":       "528e4391aab2",
		"integration-pattern/x": "acf3219be3a6",
		"subdomain/x":           "848cbe05244d",
	}
	for id, want := range pinned {
		if got := slotOf(id, nil, nil).Edition[:12]; got != want {
			t.Errorf("%s: edition %s, pinned %s", id, got, want)
		}
	}
}

// docs/adr/0007: the evidence hash covers the question's edition, so a
// verdict on an earlier edition is stale, and says that the question
// changed.
func TestADR_0007_EvidenceHashCoversTheQuestion(t *testing.T) {
	t.Parallel()
	evidence := []string{"site: a.go:1"}
	slot := slotOf("connascence/x", evidence, nil)
	verdict := func(edition, hash string) arch.Verdict {
		return arch.Verdict{
			Criterion:    "connascence/x",
			Answer:       "confirmed",
			EvidenceHash: hash,
			Edition:      edition,
			Events:       []arch.VerificationEvent{{Role: arch.RoleHuman}},
		}
	}
	cases := map[string]struct {
		v     arch.Verdict
		cause arch.StaleCause
	}{
		"current":               {verdict(slot.Edition, slot.EvidenceHash), ""},
		"an earlier question":   {verdict("0ld", "0ld"), arch.StaleQuestion},
		"other evidence":        {verdict(slot.Edition, "0ld"), arch.StaleEvidence},
		"other, edition unsaid": {verdict("", "0ld"), arch.StaleEvidence},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := slotOf("connascence/x", evidence, []arch.Verdict{tc.v})
			equals(t, tc.cause, got.StaleBecause)
		})
	}
	// Moving a boundary between evidence strings changes the hash.
	if slotOf("connascence/x", []string{"a", "b"}, nil).EvidenceHash ==
		slotOf("connascence/x", []string{"a\nb"}, nil).EvidenceHash {
		t.Error("the encoding is not injective")
	}
}

// docs/adr/0008: a judge sees the question, the answers and the evidence,
// and nothing that says what its answer would set off or how anyone
// answered before.
func TestADR_0008_TheJudgeIsBlinded(t *testing.T) {
	t.Parallel()
	const marker = "MARKER-"
	slot := arch.NewSlot(&arch.SlotRequest{
		Criterion: "connascence/x",
		Evidence:  []string{"site: a.go:1", "site: b.go:2"},
		Context:   []string{marker + "context: above maxStrength.system 3"},
	}, nil)
	slot.Status, slot.StaleBecause = marker+"status", marker+"cause"
	slot.EvidenceHash, slot.Edition = marker+"hash", marker+"edition"
	slot.Verdict = &arch.Verdict{
		Criterion: marker + "criterion",
		Answer:    marker + "answer",
		Rationale: marker + "rationale",
		Events: []arch.VerificationEvent{{
			Role: marker + "role", Actor: marker + "actor", Config: marker + "config",
		}},
		EvidenceHash: marker + "hash",
		Edition:      marker + "edition",
	}
	js, err := json.Marshal(slot.JudgeView())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(js), marker) {
		t.Errorf("a hidden field reached the judge: %s", js)
	}
	equals(t, []arch.Exhibit{{ID: "E1", Text: "site: a.go:1"}, {ID: "E2", Text: "site: b.go:2"}},
		slot.JudgeView().Exhibits)
}

// TestJudgeViewShape pins the fields a judge can see, so adding one is a
// decision, reviewed against docs/adr/0008, not an accident.
func TestJudgeViewShape(t *testing.T) {
	t.Parallel()
	fields := func(v any) []string {
		var out []string
		for f := range reflect.TypeOf(v).Fields() {
			out = append(out, f.Name)
		}
		return out
	}
	equals(
		t,
		[]string{"ID", "Question", "Answers", "Undecided", "Exhibits"},
		fields(arch.JudgeView{}),
	)
	equals(t, []string{"ID", "Text"}, fields(arch.Exhibit{}))
}

// docs/adr/0009: "cannot tell" is an answer every slot accepts, and it
// decides nothing: the slot is unresolved, gives no answer, and leaves an
// MMI criterion unscored.
func TestADR_0009_InsufficientEvidenceDecidesNothing(t *testing.T) {
	t.Parallel()
	abstain := func(slot arch.JudgementSlot) []arch.Verdict {
		return []arch.Verdict{
			{
				Criterion:    slot.Criterion,
				Answer:       arch.AnswerInsufficient,
				EvidenceHash: slot.EvidenceHash,
				Events:       []arch.VerificationEvent{{Role: arch.RoleHuman}},
			},
		}
	}
	for _, id := range []string{"connascence/x", "subdomain/x", "1.1.5"} {
		pending := slotOf(id, []string{"e"}, nil)
		if pending.Answers != nil {
			equals(t, arch.AnswerInsufficient, pending.Answers[len(pending.Answers)-1])
		}
		judged := slotOf(id, []string{"e"}, abstain(pending))
		equals(t, arch.SlotUnresolved, judged.Status)
		equals(t, "", judged.Answer())
	}

	// An abstaining MMI verdict decodes, and leaves the criterion unscored.
	vs, err := arch.DecodeJudgements(strings.NewReader(`{"version": 1, "verdicts": [
		{"criterion": "1.1.5", "answer": "insufficient-evidence", "provenance": "llm",
		 "rationale": "the names say nothing", "evidenceHash": "h", "approved": true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	equals(t, arch.AnswerInsufficient, vs[0].Answer)
	_, err = arch.DecodeJudgements(strings.NewReader(`{"version": 1, "verdicts": [
		{"criterion": "1.1.5", "answer": "insufficient-evidence", "score": 4, "provenance": "llm",
		 "rationale": "", "evidenceHash": "h", "approved": true}]}`))
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}

func TestDecodeVerificationEvents(t *testing.T) {
	t.Parallel()
	decode := func(t *testing.T, verdict string) ([]arch.Verdict, error) {
		t.Helper()
		return arch.DecodeJudgements(strings.NewReader(
			`{"version": 1, "signer": "reviewer@example.com", "verdicts": [` + verdict + `]}`))
	}
	t.Run("events take their actor from the signer", func(t *testing.T) {
		t.Parallel()
		vs, err := decode(t, `{"criterion": "subdomain/billing", "answer": "core",
			"rationale": "r", "evidenceHash": "h",
			"events": [{"role": "judge", "config": "model-v1"}, {"role": "human"}]}`)
		equals(t, nil, err)
		equals(t, []arch.VerificationEvent{
			{Role: arch.RoleJudge, Actor: "reviewer@example.com", Config: "model-v1"},
			{Role: arch.RoleHuman, Actor: "reviewer@example.com"},
		}, vs[0].Events)
	})
	for name, tc := range map[string]struct{ verdict, message string }{
		"an event naming its actor": {
			`{"criterion": "3.3", "events": [{"role": "human", "actor": "me"}]}`,
			`judgements: json: unknown field "actor"`,
		},
		"no events": {
			`{"criterion": "3.3", "score": 1}`,
			"judgements: criterion 3.3: no events attest the verdict",
		},
		"events and provenance": {
			`{"criterion": "3.3", "provenance": "human", "events": [{"role": "human"}]}`,
			"judgements: criterion 3.3: events and the first format's provenance and approved " +
				"cannot be mixed",
		},
		"an unknown role": {
			`{"criterion": "3.3", "events": [{"role": "oracle"}]}`,
			`judgements: criterion 3.3: role "oracle" is not judge, adversary or human`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := decode(t, tc.verdict)
			equals(t, arch.EINVALID, arch.ErrorCode(err))
			equals(t, tc.message, arch.ErrorMessage(err))
		})
	}
}

// A file of the first format still works: approved reads as a human event,
// and a draft as a judge's, so each gives the slot the status it gave
// before.
func TestFirstFormatReadsAsEvents(t *testing.T) {
	t.Parallel()
	slot := slotOf("subdomain/billing", []string{"e"}, nil)
	statusOf := func(attestation string) arch.SlotStatus {
		vs, err := arch.DecodeJudgements(strings.NewReader(`{"version": 1, "verdicts": [
			{"criterion": "subdomain/billing", "answer": "core", "rationale": "r",
			 "evidenceHash": "` + slot.EvidenceHash + `", ` + attestation + `}]}`))
		if err != nil {
			t.Fatal(err)
		}
		return slotOf("subdomain/billing", []string{"e"}, vs).Status
	}
	equals(t, arch.SlotSupplied, statusOf(`"provenance": "llm", "approved": true`))
	equals(t, arch.SlotSupplied, statusOf(`"events": [{"role": "human"}]`))
	equals(t, arch.SlotUnapproved, statusOf(`"provenance": "llm", "approved": false`))
	equals(t, arch.SlotUnapproved, statusOf(`"provenance": "human"`))
	equals(t, arch.SlotUnapproved, statusOf(`"events": [{"role": "judge", "config": "m"}]`))
}

// Every slot says why no measure settled it, and a measure may say it more
// precisely for one slot.
func TestEverySlotSaysWhyItIsUndecided(t *testing.T) {
	t.Parallel()
	for _, id := range []string{
		"1.1.5", "1.1.6", "1.2.2", "3.3", "3.4", "async/x", "balance/x", "bounded-context/x",
		"connascence/x", "contract-type/x", "integration-pattern/x", "subdomain/x",
	} {
		if slotOf(id, nil, nil).Undecided == "" {
			t.Errorf("%s gives no reason it is undecided", id)
		}
	}
	precise := arch.NewSlot(&arch.SlotRequest{
		Criterion: "connascence/x", Undecided: "low confidence",
	}, nil)
	equals(t, "low confidence", precise.Undecided)
	equals(t, "low confidence", precise.JudgeView().Undecided)
	if precise.EvidenceHash == slotOf("connascence/x", nil, nil).EvidenceHash {
		t.Error("the reason a slot is undecided is not hashed")
	}
}

// A judge's coverage and an adversary's event survive the judgements file,
// so a later run can show the coverage to an adversary and skip a draft it
// already challenged.
func TestJudgementsKeepCoverage(t *testing.T) {
	t.Parallel()
	confidence := 0.7
	in := []arch.Verdict{{
		Criterion: "3.3", Score: 6, Rationale: "r", EvidenceHash: "h",
		Events: []arch.VerificationEvent{
			{
				Role: arch.RoleJudge, Config: "c1", Examined: []string{"the layout"},
				Unexamined: []string{"b/b.go: not shown"},
			},
			{
				Role:       arch.RoleAdversary,
				Config:     "c2",
				Confidence: &confidence,
				Cites:      []string{"E2"},
			},
		},
	}}
	var buf strings.Builder
	if err := arch.EncodeJudgements(&buf, "", in); err != nil {
		t.Fatal(err)
	}
	out, err := arch.DecodeJudgements(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	equals(t, in, out)
}
