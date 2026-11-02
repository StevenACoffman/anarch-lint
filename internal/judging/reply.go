package judging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// fence matches a fenced code block; its info string may name json.
var fence = regexp.MustCompile("(?s)```[a-zA-Z]*\\n(.*?)```")

// Reply is a judge's answer to one item, as its one fenced JSON block holds
// it.
type Reply struct {
	Answer     string   `json:"answer,omitempty"`
	Score      *int     `json:"score,omitempty"`
	Confidence *float64 `json:"confidence"`
	Cites      []string `json:"cites"`
	Rationale  string   `json:"rationale"`
	// Examined and Unexamined say what the judge checked and could not;
	// they are advisory and never block (mmi-plan.md §9.3 C).
	Examined   []string `json:"examined,omitempty"`
	Unexamined []Gap    `json:"unexamined,omitempty"`
}

// Gap is an aspect a judge could not examine, and why.
type Gap struct {
	Aspect string `json:"aspect"`
	Reason string `json:"reason"`
}

// Parse reads a judge's reply to an item, or returns every defect that
// makes it unusable: not exactly one fenced block; a field the format lacks;
// an answer outside the item's, or a score outside 0–10 or on a family slot;
// no confidence, or one outside [0, 1]; no citation for a substantive
// answer, or a citation the prompt did not show; a rationale that is empty,
// repeats the question or is boilerplate; an unexamined aspect with no
// reason. A rejected reply counts as no answer, never as a default one
// (mmi-plan.md §9.3 C).
func Parse(raw string, it *Item) (Reply, []string) {
	blocks := fence.FindAllStringSubmatch(raw, -1)
	if len(blocks) != 1 {
		return Reply{}, []string{
			fmt.Sprintf("want exactly one fenced block, found %d", len(blocks)),
		}
	}
	var r Reply
	dec := json.NewDecoder(bytes.NewReader([]byte(blocks[0][1])))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return Reply{}, []string{"the block is not a reply: " + err.Error()}
	}
	defects := slices.Concat(r.answerDefects(&it.View), r.citeDefects(&it.View),
		r.rationaleDefects(&it.View))
	if r.Confidence == nil || *r.Confidence < 0 || *r.Confidence > 1 {
		defects = append(defects, "confidence must be a number in [0, 1]")
	}
	for _, g := range r.Unexamined {
		if strings.TrimSpace(g.Reason) == "" {
			defects = append(defects, fmt.Sprintf("unexamined aspect %q gives no reason", g.Aspect))
		}
	}
	return r, defects
}

// Abstains reports an answer that the evidence is insufficient.
func (r *Reply) Abstains() bool { return r.Answer == arch.AnswerInsufficient }

// answerDefects checks the answer against what the item accepts: a family
// slot's answers, or a score or an abstention for an MMI criterion.
func (r *Reply) answerDefects(v *arch.JudgeView) []string {
	if len(v.Answers) == 0 {
		return r.scoreDefects()
	}
	switch {
	case r.Score != nil:
		return []string{"a family slot takes an answer, not a score"}
	case !slices.Contains(v.Answers, r.Answer):
		return []string{fmt.Sprintf("answer %q is not one of %s", r.Answer,
			strings.Join(v.Answers, ", "))}
	}
	return nil
}

// scoreDefects checks the answer to an MMI criterion: a score from 0 to 10,
// or the answer that the evidence is insufficient, with no score.
func (r *Reply) scoreDefects() []string {
	switch {
	case r.Abstains() && r.Score != nil:
		return []string{"an abstaining reply takes no score"}
	case r.Abstains():
		return nil
	case r.Answer != "":
		return []string{"an MMI criterion takes a score, or the answer " + arch.AnswerInsufficient}
	case r.Score == nil || *r.Score < 0 || *r.Score > 10:
		return []string{"an MMI criterion takes a score from 0 to 10"}
	}
	return nil
}

// citeDefects checks the citations: a substantive answer cites at least one
// exhibit, and every citation is one the prompt showed (IMPROVEMENTS.md
// §7.4).
func (r *Reply) citeDefects(v *arch.JudgeView) []string {
	var out []string
	if len(r.Cites) == 0 && !r.Abstains() {
		out = append(out, "a substantive answer must cite an exhibit")
	}
	for _, c := range r.Cites {
		if !slices.ContainsFunc(v.Exhibits, func(x arch.Exhibit) bool { return x.ID == c }) {
			out = append(out, fmt.Sprintf("cites %s, which the prompt did not show", c))
		}
	}
	return out
}

// rationaleDefects refuses a rationale that says nothing of its own: empty,
// the question repeated, or a stock phrase (gnosis's rationale check).
func (r *Reply) rationaleDefects(v *arch.JudgeView) []string {
	text := normalizedText(r.Rationale)
	switch {
	case text == "":
		return []string{"the rationale is empty"}
	case strings.Contains(text, normalizedText(v.Question)):
		return []string{"the rationale repeats the question"}
	}
	for _, phrase := range boilerplate() {
		if strings.Contains(text, phrase) {
			return []string{fmt.Sprintf("the rationale is boilerplate (%q)", phrase)}
		}
	}
	return nil
}

// boilerplate lists phrases that stand in for a reason.
func boilerplate() []string {
	return []string{
		"as an ai", "based on the evidence provided", "based on the information provided",
		"it is important to note", "i cannot determine", "without more context",
	}
}

// normalizedText is text in lower case with its runs of space collapsed, so
// two rationales compare by their words.
func normalizedText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}
