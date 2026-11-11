package judging

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Labelled is a question whose answer is known: a canary, which qualifies a
// configuration, or a gold item, which a human labelled. Its question and
// answers come from its family, so they cannot drift from the real slots'.
type Labelled struct {
	// Criterion is a slot ID; a canary's subject is a name that does not
	// say it is a canary.
	Criterion string   `json:"criterion"`
	Evidence  []string `json:"evidence"`
	Undecided string   `json:"undecided,omitempty"`
	// Answer is the known answer of a family slot; Score that of an MMI
	// criterion, with ScoreMax its upper end when a range is acceptable.
	Answer   string `json:"answer,omitempty"`
	Score    int    `json:"score,omitempty"`
	ScoreMax int    `json:"scoreMax,omitempty"`
	// Note says why the answer is the right one, for the reader of the
	// corpus; no judge sees it.
	Note string `json:"note,omitempty"`
}

// question is one slot to ask, with why it is asked.
type question struct {
	kind Kind
	slot *arch.JudgementSlot
}

// EmitInput is what a judging run asks.
type EmitInput struct {
	// Slots are the run's judgement slots; only pending and stale ones are
	// asked.
	Slots    []arch.JudgementSlot
	Canaries []Labelled
	Gold     []Labelled
	Configs  []Config
	Budget   Budget
}

// Emit returns the run's manifest: every pending or stale slot, the
// canaries of each family asked, and the gold items, each for every
// configuration in two samples, sorted by key so that nothing about an
// item's position tells a canary from a slot.
func Emit(in *EmitInput) Manifest {
	m := Manifest{
		Version: manifestVersion, PromptVersion: PromptVersion,
		Configs: slices.Clone(in.Configs), Items: make([]Item, 0),
	}
	asked := map[string]bool{}
	var questions []question
	for i := range in.Slots {
		s := &in.Slots[i]
		if s.Status == arch.SlotPending || s.Status == arch.SlotStale {
			asked[FamilyOf(s.Criterion)] = true
			questions = append(questions, question{KindSlot, s})
		}
	}
	for i := range in.Canaries {
		if asked[FamilyOf(in.Canaries[i].Criterion)] {
			questions = append(questions, question{KindCanary, in.Canaries[i].slot()})
		}
	}
	for i := range in.Gold {
		questions = append(questions, question{KindGold, in.Gold[i].slot()})
	}
	for _, c := range in.Configs {
		for _, q := range questions {
			for sample := range 2 {
				m.Items = append(m.Items, q.item(&c, sample, in.Budget))
			}
		}
	}
	slices.SortFunc(m.Items, func(a, b Item) int { return cmp.Compare(a.Key, b.Key) })
	return m
}

// item is the question for one configuration and sample, within the budget.
func (q question) item(c *Config, sample int, budget Budget) Item {
	view := q.slot.JudgeView()
	exhibits := view.Exhibits
	if sample == 1 {
		exhibits = slices.Clone(exhibits)
		slices.Reverse(exhibits)
	}
	omitted := max(0, len(exhibits)-budget.MaxExhibits)
	exhibits = exhibits[:len(exhibits)-omitted]
	for i := range exhibits {
		exhibits[i].Text = clip(exhibits[i].Text, budget.MaxExhibitBytes)
	}
	view.Exhibits = exhibits
	return Item{
		Key: shortHash(
			q.slot.EvidenceHash,
			strconv.Itoa(PromptVersion),
			c.Hash(),
			strconv.Itoa(sample),
		),
		Kind:         q.kind,
		Family:       FamilyOf(q.slot.Criterion),
		Config:       c.Name,
		Sample:       sample,
		EvidenceHash: q.slot.EvidenceHash,
		Edition:      q.slot.Edition,
		View:         view,
		Omitted:      omitted,
	}
}

// slot is the labelled question as the slot its family asks.
func (l *Labelled) slot() *arch.JudgementSlot {
	s := arch.NewSlot(&arch.SlotRequest{
		Criterion: l.Criterion, Evidence: l.Evidence, Undecided: l.Undecided,
	}, nil)
	return &s
}

// clip shortens text to at most n bytes, at a rune boundary, saying how much
// it left out.
func clip(text string, n int) string {
	if n <= 0 || len(text) <= n {
		return text
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return fmt.Sprintf("%s… (%d more bytes left out)", text[:cut], len(text)-cut)
}

// Render writes an item as the prompt a judge answers: the question, the
// answers it accepts, why no measure could decide, the numbered exhibits,
// and the reply format. It shows nothing that says what kind of item it is.
func Render(it *Item) string {
	var b strings.Builder
	v := &it.View
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", v.ID, v.Question)
	if len(v.Answers) > 0 {
		fmt.Fprintf(&b, "Answer with one of: %s.\n\n", strings.Join(v.Answers, ", "))
	} else {
		fmt.Fprintf(&b, "Answer with a score from 0, the worst, to 10, the best, or with "+
			"the answer %s.\n\n", arch.AnswerInsufficient)
	}
	fmt.Fprintf(&b, "No measure could decide this: %s.\n\n## Evidence\n\n", v.Undecided)
	for _, x := range v.Exhibits {
		fmt.Fprintf(&b, "- %s: %s\n", x.ID, x.Text)
	}
	if it.Omitted > 0 {
		fmt.Fprintf(&b, "\n%d more exhibits were left out to keep this prompt short; "+
			"cite only those shown.\n", it.Omitted)
	}
	b.WriteString(replyFormat(len(v.Answers) == 0))
	return b.String()
}

// replyFormat says how to reply: one fenced JSON block, with a score for an
// MMI criterion and an answer otherwise.
func replyFormat(score bool) string {
	answer, abstain := `"answer": "<one of the answers>"`, ""
	if score {
		answer, abstain = `"score": <0 to 10>`, `, with no score`
	}
	return "\n## Reply\n\nReply with exactly one fenced JSON block and nothing else that is " +
		"fenced:\n\n```json\n{" + answer + `, "confidence": <0 to 1>, "cites": ["E1"],` +
		"\n" + ` "rationale": "<why, in your own words>", "examined": ["<what you checked>"],` +
		"\n" + ` "unexamined": [{"aspect": "<what you could not check>", "reason": "<why>"}]}` +
		"\n```\n\nCite the exhibits your answer rests on; a substantive answer needs at least " +
		"one. In the rationale, put each name, path or value you rely on in backticks, " +
		"spelled as the question or a cited exhibit spells it. Answer " +
		arch.AnswerInsufficient + " when the evidence cannot settle it" + abstain + ".\n"
}
