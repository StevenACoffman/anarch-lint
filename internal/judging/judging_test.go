package judging_test

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/judging"
)

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// slot is a pending slot of the connascence family with three exhibits.
func slot(subject string, evidence ...string) arch.JudgementSlot {
	if len(evidence) == 0 {
		evidence = []string{
			"the layout is written in 2 packages",
			"site: a/a.go:3",
			"site: b/b.go:4",
		}
	}
	return arch.NewSlot(&arch.SlotRequest{
		Criterion: "connascence/" + subject, Evidence: evidence, Context: []string{"MARKER"},
	}, nil)
}

func configs() []judging.Config {
	return []judging.Config{{Name: "a", Model: "m1"}, {Name: "b", Model: "m2"}}
}

// reply is a well-formed reply giving answer, citing E1.
func reply(answer, rationale string) string {
	return fmt.Sprintf("Here is my reply.\n\n```json\n{\"answer\": %q, \"confidence\": 0.8, "+
		"\"cites\": [\"E1\"], \"rationale\": %q}\n```\n", answer, rationale)
}

// TestEmit: each pending or stale slot, and each canary of a family asked,
// is asked of every configuration in two samples, the second with its
// exhibits reversed; a supplied slot and the canaries of other families are
// not; nothing hidden reaches a prompt; the budget is counted.
func TestEmit(t *testing.T) {
	t.Parallel()
	supplied := slot("done")
	supplied.Status = arch.SlotSupplied
	canaries, err := judging.Canaries()
	if err != nil {
		t.Fatal(err)
	}
	m := judging.Emit(&judging.EmitInput{
		Slots:    []arch.JudgementSlot{slot("x"), supplied},
		Canaries: canaries, Configs: configs(),
		Budget: judging.Budget{MaxExhibits: 2, MaxExhibitBytes: 10},
	})
	kinds := map[judging.Kind]int{}
	for _, it := range m.Items {
		kinds[it.Kind]++
		if it.Kind == judging.KindSlot && it.Family != "connascence" {
			t.Errorf("a %s item was asked", it.Family)
		}
		prompt := judging.Render(&it)
		if strings.Contains(prompt, "MARKER") || strings.Contains(prompt, "canary") {
			t.Fatalf("the prompt shows what a judge must not see:\n%s", prompt)
		}
	}
	equals(t, map[judging.Kind]int{judging.KindSlot: 4, judging.KindCanary: 12}, kinds)
	assertSorted(t, m.Items)

	var forward, reversed judging.Item
	for _, it := range m.Items {
		if it.View.ID == "connascence/x" && it.Config == "a" {
			if it.Sample == 0 {
				forward = it
			} else {
				reversed = it
			}
		}
	}
	equals(t, 1, forward.Omitted)
	equals(t, "the layout… (25 more bytes left out)", forward.View.Exhibits[0].Text)
	equals(t, "E3", reversed.View.Exhibits[0].ID)
	if forward.Key == reversed.Key {
		t.Fatal("the two samples share a key")
	}
}

func assertSorted(t *testing.T, items []judging.Item) {
	t.Helper()
	if !slices.IsSortedFunc(
		items,
		func(a, b judging.Item) int { return strings.Compare(a.Key, b.Key) },
	) {
		t.Fatal("the items are not sorted by key")
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	x, criterion := slot("x"), arch.NewSlot(&arch.SlotRequest{Criterion: "3.3"}, nil)
	it, mmi := judging.Item{View: x.JudgeView()}, judging.Item{View: criterion.JudgeView()}
	block := func(body string) string { return "```json\n" + body + "\n```" }
	cases := map[string]struct {
		item   *judging.Item
		raw    string
		defect string // "" when the reply is accepted
	}{
		"accepted": {item: &it, raw: reply("confirmed", "both validate one format")},
		"no block": {item: &it, raw: "confirmed", defect: "want exactly one fenced block, found 0"},
		"two blocks": {
			item: &it, raw: reply("confirmed", "a") + reply("rejected", "b"),
			defect: "want exactly one fenced block, found 2",
		},
		"an unknown field": {
			item:   &it,
			defect: "the block is not a reply",
			raw: block(
				`{"answer": "confirmed", "confidence": 1, "cites": ["E1"], "rationale": "r", "mood": "sure"}`,
			),
		},
		"an unknown answer": {
			item: &it, raw: reply("maybe", "r"),
			defect: `answer "maybe" is not one of confirmed, rejected, insufficient-evidence`,
		},
		"no confidence": {
			item: &it, defect: "confidence must be a number in [0, 1]",
			raw: block(`{"answer": "confirmed", "cites": ["E1"], "rationale": "r"}`),
		},
		"no citation": {
			item: &it, defect: "a substantive answer must cite an exhibit",
			raw: block(`{"answer": "confirmed", "confidence": 1, "cites": [], "rationale": "r"}`),
		},
		"an abstention needs no citation": {
			item: &it,
			raw: block(
				`{"answer": "insufficient-evidence", "confidence": 0.5, "cites": [], "rationale": "too little"}`,
			),
		},
		"a citation not shown": {
			item:   &it,
			defect: "cites E9, which the prompt did not show",
			raw: block(
				`{"answer": "confirmed", "confidence": 1, "cites": ["E9"], "rationale": "r"}`,
			),
		},
		"the question repeated": {
			item:   &it,
			defect: "the rationale repeats the question",
			raw: reply(
				"confirmed",
				"Must these places change together, so that the candidate is connascence?",
			),
		},
		"boilerplate": {
			item: &it, raw: reply("confirmed", "Based on the evidence provided, yes."),
			defect: `the rationale is boilerplate ("based on the evidence provided")`,
		},
		"a quoted name a cited exhibit shows": {
			item: &it,
			raw:  reply("confirmed", "both write the `layout`, a `connascence` of algorithm"),
		},
		"a quoted name only an uncited exhibit shows": {
			item: &it, raw: reply("confirmed", "`a/a.go` writes it"),
			defect: `the rationale quotes "a/a.go", which no cited exhibit shows`,
		},
		"a quoted name nothing shows": {
			item: &it, raw: reply("confirmed", "`ParseSKU` writes it"),
			defect: `the rationale quotes "ParseSKU", which no cited exhibit shows`,
		},
		"an unexamined aspect without a reason": {
			item:   &it,
			defect: `unexamined aspect "tests" gives no reason`,
			raw: block(`{"answer": "confirmed", "confidence": 1, "cites": ["E1"], "rationale": "r",
				"unexamined": [{"aspect": "tests", "reason": ""}]}`),
		},
		"an MMI score": {
			item: &mmi,
			raw: block(
				`{"score": 7, "confidence": 0.6, "cites": [], "rationale": "roles are clear"}`,
			),
			defect: "a substantive answer must cite an exhibit",
		},
		"an MMI score out of range": {
			item: &mmi, defect: "an MMI criterion takes a score from 0 to 10",
			raw: block(`{"score": 11, "confidence": 0.6, "cites": [], "rationale": "r"}`),
		},
		"an answer on an MMI criterion": {
			item:   &mmi,
			defect: "an MMI criterion takes a score, or the answer insufficient-evidence",
			raw: block(
				`{"answer": "confirmed", "confidence": 0.6, "cites": [], "rationale": "r"}`,
			),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, defects := judging.Parse(tc.raw, tc.item)
			if tc.defect == "" {
				equals(t, []string(nil), defects)
				return
			}
			if !slices.ContainsFunc(
				defects,
				func(d string) bool { return strings.HasPrefix(d, tc.defect) },
			) {
				t.Fatalf("want a defect %q, got %q", tc.defect, defects)
			}
		})
	}
}

// answerAll replies to every item: canaries as labelled unless wrong names a
// configuration that gets them wrong, and slots with the answer of a.
func answerAll(t *testing.T, m *judging.Manifest, canaries []judging.Labelled,
	answers map[string]string, wrong string,
) map[string]string {
	t.Helper()
	labels := map[string]string{}
	for _, c := range canaries {
		labels[c.Criterion] = c.Answer
	}
	out := map[string]string{}
	for i := range m.Items {
		it := &m.Items[i]
		answer := answers[it.Config]
		if it.Kind == judging.KindCanary {
			answer = labels[it.View.ID]
			if it.Config == wrong {
				answer = map[string]string{
					"confirmed": "rejected", "rejected": "confirmed",
					"contract": "model", "model": "contract",
				}[answer]
			}
		}
		out[it.Key] = reply(answer, "reason "+it.Key)
	}
	return out
}

// TestAdmit: independent configurations that agree record a judge's verdict;
// a split records insufficient-evidence; a configuration that misses a
// canary has its answers in the family refused, which leaves too few judges.
func TestAdmit(t *testing.T) {
	t.Parallel()
	canaries, err := judging.Canaries()
	if err != nil {
		t.Fatal(err)
	}
	m := judging.Emit(&judging.EmitInput{
		Slots: []arch.JudgementSlot{slot("x")}, Canaries: canaries, Configs: configs(),
		Budget: judging.DefaultBudget(),
	})
	cases := map[string]struct {
		answers map[string]string
		wrong   string
		outcome judging.TallyOutcome
		answer  string
		events  int
	}{
		"agreement": {
			answers: map[string]string{"a": "confirmed", "b": "confirmed"},
			outcome: judging.TallyApplied, answer: "confirmed", events: 4,
		},
		"a split": {
			answers: map[string]string{"a": "confirmed", "b": "rejected"},
			outcome: judging.TallySplit, answer: arch.AnswerInsufficient, events: 4,
		},
		"a missed canary": {
			answers: map[string]string{"a": "confirmed", "b": "confirmed"}, wrong: "b",
			outcome: judging.TallyTooFew,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := judging.Admit(&judging.AdmitInput{
				Manifest: m, Replies: answerAll(t, &m, canaries, tc.answers, tc.wrong),
				Canaries: canaries,
			})
			equals(t, tc.wrong != "", r.Untrustworthy())
			equals(t, 1, len(r.Tallies))
			equals(t, tc.outcome, r.Tallies[0].Outcome)
			if tc.events == 0 {
				equals(t, 0, len(r.Verdicts))
				return
			}
			equals(t, 1, len(r.Verdicts))
			v := r.Verdicts[0]
			equals(t, tc.answer, v.Answer)
			equals(t, tc.events, len(v.Events))
			equals(t, arch.RoleJudge, v.Events[0].Role)
			// A judge's verdict is a draft until a human attests it.
			equals(t, arch.SlotUnapproved, arch.NewSlot(&arch.SlotRequest{
				Criterion: "connascence/x",
				Evidence:  slot("x").Evidence,
				Context:   []string{"MARKER"},
			}, r.Verdicts).Status)
		})
	}
}

// TestAdmitRefusesTemplates: a rationale given for one slot and repeated for
// another is a template, and the second reply is rejected; a missing reply
// is listed.
func TestAdmitRefusesTemplates(t *testing.T) {
	t.Parallel()
	m := judging.Emit(&judging.EmitInput{
		Slots: []arch.JudgementSlot{slot("x"), slot("y", "other evidence")}, Configs: configs()[:1],
		Budget: judging.DefaultBudget(),
	})
	replies := map[string]string{}
	for _, it := range m.Items[:len(m.Items)-1] {
		replies[it.Key] = reply("confirmed", "the same words every time")
	}
	r := judging.Admit(&judging.AdmitInput{Manifest: m, Replies: replies})
	equals(t, []string{m.Items[len(m.Items)-1].Key}, r.Missing)
	if len(r.Rejected) == 0 ||
		!strings.HasPrefix(r.Rejected[0].Defects[0], "the rationale repeats the one given for") {
		t.Fatalf("want the repeated rationale rejected, got %v", r.Rejected)
	}
}

// TestAdmitKeepsAHumanVerdict: a verdict a human attests for the same
// evidence is kept; agreeing judges add their events to it.
func TestAdmitKeepsAHumanVerdict(t *testing.T) {
	t.Parallel()
	s := slot("x")
	human := arch.Verdict{
		Criterion: s.Criterion, Answer: "rejected", EvidenceHash: s.EvidenceHash,
		Rationale: "seen", Events: []arch.VerificationEvent{{Role: arch.RoleHuman}},
	}
	m := judging.Emit(&judging.EmitInput{
		Slots: []arch.JudgementSlot{s}, Configs: configs(), Budget: judging.DefaultBudget(),
	})
	for answer, events := range map[string]int{"confirmed": 1, "rejected": 5} {
		replies := map[string]string{}
		for _, it := range m.Items {
			replies[it.Key] = reply(answer, "reason "+it.Key)
		}
		r := judging.Admit(&judging.AdmitInput{
			Manifest: m, Replies: replies, Verdicts: []arch.Verdict{human},
		})
		equals(t, "rejected", r.Verdicts[0].Answer)
		equals(t, events, len(r.Verdicts[0].Events))
	}
}

// TestCanaries: the corpus decodes, and each family it covers has a clear
// positive, a clear negative and a decoy.
func TestCanaries(t *testing.T) {
	t.Parallel()
	canaries, err := judging.Canaries()
	if err != nil {
		t.Fatal(err)
	}
	answers := map[string]map[string]bool{}
	for _, c := range canaries {
		family := judging.FamilyOf(c.Criterion)
		if answers[family] == nil {
			answers[family] = map[string]bool{}
		}
		answers[family][c.Answer] = true
	}
	for family, seen := range answers {
		if len(seen) < 2 {
			t.Errorf("family %s has canaries of one answer only", family)
		}
	}
}

// An adversary is asked about a slot whose judges' draft awaits a human,
// shown what the judges examined and never what they answered. One that
// disagrees unsettles the draft into insufficient-evidence; one that agrees
// adds its event; once it has challenged a draft it is not asked again.
func TestAdversary(t *testing.T) {
	t.Parallel()
	judges := judging.Emit(&judging.EmitInput{
		Slots: []arch.JudgementSlot{slot("x")}, Configs: configs(), Budget: judging.DefaultBudget(),
	})
	replies := map[string]string{}
	for _, it := range judges.Items {
		replies[it.Key] = "```json\n{\"answer\": \"confirmed\", \"confidence\": 0.9, " +
			"\"cites\": [\"E1\"], \"rationale\": \"JUDGES-SECRET " + it.Key + "\", " +
			"\"examined\": [\"the layout\"], \"unexamined\": [{\"aspect\": \"b/b.go\", " +
			"\"reason\": \"not shown\"}]}\n```\n"
	}
	drafts := judging.Admit(&judging.AdmitInput{Manifest: judges, Replies: replies}).Verdicts
	s := arch.NewSlot(&arch.SlotRequest{
		Criterion: "connascence/x", Evidence: slot("x").Evidence, Context: []string{"MARKER"},
	}, drafts)
	equals(t, arch.SlotUnapproved, s.Status)

	withAdversary := append(configs(), judging.Config{Name: "adv", Model: "m3", Adversary: true})
	m := judging.Emit(&judging.EmitInput{
		Slots: []arch.JudgementSlot{s}, Configs: withAdversary, Budget: judging.DefaultBudget(),
	})
	equals(t, 2, len(m.Items))
	prompt := judging.Render(&m.Items[0])
	equals(t, judging.KindAdversary, m.Items[0].Kind)
	equals(t, true, strings.Contains(prompt, "They examined: the layout"))
	equals(t, true, strings.Contains(prompt, "They could not examine: b/b.go: not shown"))
	equals(t, false, strings.Contains(prompt, "JUDGES-SECRET"))

	for answer, expected := range map[string]string{
		"rejected": arch.AnswerInsufficient, "confirmed": "confirmed",
	} {
		challenges := map[string]string{}
		for _, it := range m.Items {
			challenges[it.Key] = reply(answer, "adversary reason "+it.Key)
		}
		r := judging.Admit(&judging.AdmitInput{Manifest: m, Replies: challenges, Verdicts: drafts})
		equals(t, expected, r.Verdicts[0].Answer)
		equals(t, arch.RoleAdversary, r.Verdicts[0].Events[len(r.Verdicts[0].Events)-1].Role)
		again := arch.NewSlot(&arch.SlotRequest{
			Criterion: "connascence/x", Evidence: slot("x").Evidence, Context: []string{"MARKER"},
		}, r.Verdicts)
		if again.Status == arch.SlotUnapproved {
			m2 := judging.Emit(&judging.EmitInput{
				Slots: []arch.JudgementSlot{again}, Configs: withAdversary,
				Budget: judging.DefaultBudget(),
			})
			equals(t, 0, len(m2.Items))
		}
	}
}

// TestADR_0020_AnAdversaryCanOnlyUnsettle pins docs/adr/0020: an adversary
// is asked nothing about a slot without a judges' draft, so its answer can
// never be the first; and its disagreement with a verdict a human attests
// changes nothing but a note.
func TestADR_0020_AnAdversaryCanOnlyUnsettle(t *testing.T) {
	t.Parallel()
	adversary := append(configs(), judging.Config{Name: "adv", Model: "m3", Adversary: true})
	m := judging.Emit(&judging.EmitInput{
		Slots: []arch.JudgementSlot{slot("x")}, Configs: adversary, Budget: judging.DefaultBudget(),
	})
	for _, it := range m.Items {
		equals(t, judging.KindSlot, it.Kind)
	}

	s := slot("x")
	human := arch.Verdict{
		Criterion: s.Criterion, Answer: "rejected", EvidenceHash: s.EvidenceHash,
		Rationale: "a person looked", Events: []arch.VerificationEvent{
			{Role: arch.RoleJudge, Config: "c1"}, {Role: arch.RoleHuman},
		},
	}
	unapproved := arch.NewSlot(&arch.SlotRequest{
		Criterion: s.Criterion, Evidence: s.Evidence, Context: []string{"MARKER"},
	}, []arch.Verdict{{
		Criterion: s.Criterion, Answer: "rejected", EvidenceHash: s.EvidenceHash,
		Rationale: "judges", Events: []arch.VerificationEvent{{Role: arch.RoleJudge}},
	}})
	m = judging.Emit(&judging.EmitInput{
		Slots: []arch.JudgementSlot{
			unapproved,
		},
		Configs: adversary,
		Budget:  judging.DefaultBudget(),
	})
	replies := map[string]string{}
	for _, it := range m.Items {
		replies[it.Key] = reply("confirmed", "adversary "+it.Key)
	}
	r := judging.Admit(&judging.AdmitInput{
		Manifest: m, Replies: replies, Verdicts: []arch.Verdict{human},
	})
	equals(t, []arch.Verdict{human}, r.Verdicts)
	equals(t, 2, len(r.Notes))
}
