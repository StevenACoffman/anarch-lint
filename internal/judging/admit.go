package judging

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// The outcomes of a tally.
const (
	// TallyApplied: every configuration gave the same substantive answer.
	TallyApplied TallyOutcome = "applied"
	// TallySplit: the answers differ, which leaves the slot unresolved.
	TallySplit TallyOutcome = "split"
	// TallyAbstained: every answer was insufficient-evidence.
	TallyAbstained TallyOutcome = "abstained"
	// TallyTooFew: fewer than two configurations answered, so nothing is
	// recorded.
	TallyTooFew TallyOutcome = "too-few-judges"
)

// minJudges is the fewest independent configurations a tally needs.
const minJudges = 2

// TallyOutcome is what a slot's answers came to.
type TallyOutcome string

// AdmitInput is a judging run's manifest and replies, with the verdicts
// already recorded and the questions of known answer.
type AdmitInput struct {
	Manifest Manifest
	// Replies maps an item's key to its raw reply; a key with none was not
	// answered.
	Replies  map[string]string
	Verdicts []arch.Verdict
	Canaries []Labelled
	Gold     []Labelled
}

// Result is what a judging run admits.
type Result struct {
	// Verdicts are the judgements file to write: the recorded verdicts with
	// the run's tallies merged in, sorted by criterion.
	Verdicts []arch.Verdict
	Rejected []Rejection
	// Missing are the keys of items with no reply.
	Missing  []string
	Canaries []CanaryResult
	Tallies  []Tally
	Gold     []GoldOutcome
	// Notes report what deserves a reader's eye but decides nothing, such as
	// one set of exhibits cited for unrelated slots.
	Notes []string
}

// Rejection is a reply the parser refused, with every defect.
type Rejection struct {
	Key     string   `json:"key"`
	ID      string   `json:"id"`
	Config  string   `json:"config"`
	Defects []string `json:"defects"`
}

// CanaryResult is whether one configuration answered every canary of one
// family right; one that did not has its answers in that family refused.
type CanaryResult struct {
	Family string   `json:"family"`
	Config string   `json:"config"`
	Passed bool     `json:"passed"`
	Misses []string `json:"misses,omitempty"`
}

// Tally is what one slot's answers came to.
type Tally struct {
	Criterion string         `json:"criterion"`
	Counts    map[string]int `json:"counts"`
	Configs   []string       `json:"configs"`
	Outcome   TallyOutcome   `json:"outcome"`
}

// GoldOutcome is one configuration's answer to one gold item: whether it
// agreed with the human label, and the confidence it stated.
type GoldOutcome struct {
	Family     string  `json:"family"`
	Config     string  `json:"config"`
	HeldOut    bool    `json:"heldOut"`
	Correct    bool    `json:"correct"`
	Confidence float64 `json:"confidence"`
}

// answer is an accepted reply with the item it answers.
type answer struct {
	item  *Item
	reply Reply
}

// Untrustworthy reports whether a configuration missed a canary: the run's
// judges cannot all be trusted, which the command signals with its own exit
// status.
func (r *Result) Untrustworthy() bool {
	return slices.ContainsFunc(r.Canaries, func(c CanaryResult) bool { return !c.Passed })
}

// Admit parses every reply, refuses the answers of a configuration in a
// family whose canaries it missed, tallies the rest by slot, and merges the
// tallies into the recorded verdicts (mmi-plan.md §9.3 C–G). Every recorded
// event is a judge's: a verdict applies only once a human attests it.
func Admit(in *AdmitInput) Result {
	r := Result{Rejected: make([]Rejection, 0), Missing: make([]string, 0)}
	accepted := r.parse(in)
	refused := r.canaries(in, accepted)
	slots := map[string][]answer{}
	var challenges []answer
	for i := range accepted {
		switch a := &accepted[i]; {
		case a.item.Kind == KindGold:
			r.Gold = append(r.Gold, goldOutcome(a, in.Gold))
		case a.item.Kind == KindAdversary:
			challenges = append(challenges, *a)
		case a.item.Kind == KindSlot && !refused[[2]string{a.item.Family, a.item.Config}]:
			slots[a.item.View.ID] = append(slots[a.item.View.ID], *a)
		}
	}
	tallied := map[string]arch.Verdict{}
	for _, id := range slices.Sorted(maps.Keys(slots)) {
		t, v, ok := tally(id, slots[id], in.Manifest.Configs)
		r.Tallies = append(r.Tallies, t)
		if ok {
			tallied[id] = v
		}
	}
	var notes []string
	r.Verdicts, notes = challenge(merge(in.Verdicts, tallied), challenges, in.Manifest.Configs)
	r.Notes = append(r.Notes, notes...)
	return r
}

// parse reads every item's reply, recording the missing and the rejected,
// and returns the accepted answers in key order. A rationale that repeats
// one given earlier in the run, for another slot, is rejected as a template.
func (r *Result) parse(in *AdmitInput) []answer {
	var out []answer
	rationales := map[string]string{} // normalized rationale -> the slot that gave it
	cited := map[string][]string{}    // cited exhibit texts -> the slots that cited them
	for i := range in.Manifest.Items {
		it := &in.Manifest.Items[i]
		raw, ok := in.Replies[it.Key]
		if !ok {
			r.Missing = append(r.Missing, it.Key)
			continue
		}
		reply, defects := Parse(raw, it)
		text := normalizedText(reply.Rationale)
		if first, seen := rationales[text]; seen && first != it.View.ID && len(defects) == 0 {
			defects = append(defects, "the rationale repeats the one given for "+first)
		}
		if len(defects) > 0 {
			r.Rejected = append(r.Rejected, Rejection{it.Key, it.View.ID, it.Config, defects})
			continue
		}
		rationales[text] = cmp.Or(rationales[text], it.View.ID)
		if set := citedTexts(&reply, &it.View); len(reply.Cites) > 1 {
			cited[set] = append(cited[set], it.View.ID)
		}
		out = append(out, answer{it, reply})
	}
	for _, set := range slices.Sorted(maps.Keys(cited)) {
		if ids := slices.Compact(slices.Sorted(slices.Values(cited[set]))); len(ids) > 1 {
			r.Notes = append(r.Notes, "one set of exhibits is cited for unrelated slots, a "+
				"possible fabrication: "+strings.Join(ids, ", "))
		}
	}
	return out
}

// citedTexts is the set of exhibit texts a reply cites, as one string.
func citedTexts(reply *Reply, v *arch.JudgeView) string {
	var texts []string
	for _, x := range v.Exhibits {
		if slices.Contains(reply.Cites, x.ID) {
			texts = append(texts, x.Text)
		}
	}
	slices.Sort(texts)
	return strings.Join(texts, "\x00")
}

// canaries scores each configuration's canaries by family and returns the
// (family, configuration) pairs whose answers are refused: those with a
// canary missed, unanswered, rejected or answered differently in its two
// samples. A family with no canaries cannot qualify anyone, and is not
// refused; its verdicts are drafts like every other.
func (r *Result) canaries(in *AdmitInput, accepted []answer) map[[2]string]bool {
	known := map[string]*Labelled{}
	for i := range in.Canaries {
		c := &in.Canaries[i]
		known[c.slot().EvidenceHash] = c
	}
	right := map[string]bool{} // keys of canary items answered right
	for i := range accepted {
		a := &accepted[i]
		if c, ok := known[a.item.EvidenceHash]; ok && a.item.Kind == KindCanary {
			right[a.item.Key] = c.agrees(&a.reply)
		}
	}
	misses := map[[2]string][]string{}
	asked := map[[2]string]bool{}
	for i := range in.Manifest.Items {
		it := &in.Manifest.Items[i]
		if it.Kind != KindCanary {
			continue
		}
		key := [2]string{it.Family, it.Config}
		asked[key] = true
		if !right[it.Key] {
			misses[key] = append(misses[key], fmt.Sprintf("%s (sample %d)", it.View.ID, it.Sample))
		}
	}
	refused := map[[2]string]bool{}
	for _, key := range slices.SortedFunc(maps.Keys(asked), comparePairs) {
		r.Canaries = append(r.Canaries, CanaryResult{
			Family: key[0], Config: key[1], Passed: len(misses[key]) == 0, Misses: misses[key],
		})
		refused[key] = len(misses[key]) > 0
	}
	return refused
}

// comparePairs orders (family, configuration) pairs.
func comparePairs(a, b [2]string) int {
	return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1]))
}

// agrees reports whether a reply gives the labelled answer: the same
// answer, or a score within the labelled range.
func (l *Labelled) agrees(reply *Reply) bool {
	if l.Answer != "" || reply.Score == nil {
		return reply.Answer == l.Answer && l.Answer != ""
	}
	return *reply.Score >= l.Score && *reply.Score <= max(l.Score, l.ScoreMax)
}

// goldOutcome compares one answer to a gold item with its label. A gold
// item is held out when its hash is odd: a split fixed by the item, so
// tuning a prompt on one half leaves the other a fair test (ADH's SplitFor).
func goldOutcome(a *answer, gold []Labelled) GoldOutcome {
	out := GoldOutcome{Family: a.item.Family, Config: a.item.Config}
	if a.reply.Confidence != nil {
		out.Confidence = *a.reply.Confidence
	}
	for i := range gold {
		if gold[i].slot().EvidenceHash == a.item.EvidenceHash {
			out.Correct = gold[i].agrees(&a.reply)
			out.HeldOut = a.item.EvidenceHash[len(a.item.EvidenceHash)-1]%2 == 1
		}
	}
	return out
}

// tally counts one slot's answers across configurations and samples. Only
// an answer every configuration gives in every sample applies; a split, or
// every answer abstaining, is recorded as insufficient-evidence, which
// decides nothing (docs/adr/0009); fewer than two configurations record
// nothing.
func tally(id string, answers []answer, configs []Config) (Tally, arch.Verdict, bool) {
	t := Tally{Criterion: id, Counts: map[string]int{}}
	byConfig := map[string]bool{}
	for i := range answers {
		t.Counts[answerText(&answers[i].reply)]++
		byConfig[answers[i].item.Config] = true
	}
	t.Configs = slices.Sorted(maps.Keys(byConfig))
	switch {
	case len(t.Configs) < minJudges:
		t.Outcome = TallyTooFew
		return t, arch.Verdict{}, false
	case len(t.Counts) > 1:
		t.Outcome = TallySplit
	case answers[0].reply.Abstains():
		t.Outcome = TallyAbstained
	default:
		t.Outcome = TallyApplied
	}
	v := verdictOf(&t, answers, configs)
	return t, v, true
}

// answerText is how a tally counts a reply: its answer, or its score.
func answerText(r *Reply) string {
	if r.Score != nil {
		return fmt.Sprintf("score %d", *r.Score)
	}
	return r.Answer
}

// verdictOf records a tally as a verdict with one judge event per answer.
func verdictOf(t *Tally, answers []answer, configs []Config) arch.Verdict {
	first := answers[0]
	v := arch.Verdict{
		Criterion: t.Criterion, EvidenceHash: first.item.EvidenceHash,
		Edition: first.item.Edition,
		Rationale: fmt.Sprintf("judged %d/%d by %s: %s", len(answers), len(answers),
			strings.Join(t.Configs, ", "), first.reply.Rationale),
	}
	switch {
	case t.Outcome != TallyApplied:
		v.Answer = arch.AnswerInsufficient
		v.Rationale = fmt.Sprintf("%s: %s", t.Outcome, countsText(t.Counts))
	case first.reply.Score != nil:
		v.Score = *first.reply.Score
	default:
		v.Answer = first.reply.Answer
	}
	hashes := map[string]string{}
	for i := range configs {
		hashes[configs[i].Name] = configs[i].Hash()
	}
	for i := range answers {
		a := &answers[i]
		v.Events = append(v.Events, arch.VerificationEvent{
			Role: arch.RoleJudge, Config: hashes[a.item.Config],
			Confidence: a.reply.Confidence, Cites: a.reply.Cites,
			Examined: a.reply.Examined, Unexamined: a.reply.gaps(),
		})
	}
	return v
}

// countsText writes a tally's counts, sorted by answer.
func countsText(counts map[string]int) string {
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(counts)) {
		parts = append(parts, fmt.Sprintf("%s %d", k, counts[k]))
	}
	return strings.Join(parts, ", ")
}

// merge returns the recorded verdicts with the tallied ones merged in. A
// recorded verdict a human attests for the same evidence is kept, with the
// judges' events added when they agree with it; any other recorded verdict
// for a tallied slot, stale or a judge's draft, is replaced.
func merge(recorded []arch.Verdict, tallied map[string]arch.Verdict) []arch.Verdict {
	out := make([]arch.Verdict, 0, len(recorded)+len(tallied))
	for _, v := range recorded {
		t, ok := tallied[v.Criterion]
		switch {
		case !ok:
			out = append(out, v)
		case v.EvidenceHash == t.EvidenceHash && attested(&v):
			if v.Answer == t.Answer && v.Score == t.Score {
				v.Events = append(v.Events, t.Events...)
			}
			out = append(out, v)
		default:
			out = append(out, t)
		}
		delete(tallied, v.Criterion)
	}
	for _, id := range slices.Sorted(maps.Keys(tallied)) {
		out = append(out, tallied[id])
	}
	slices.SortFunc(
		out,
		func(a, b arch.Verdict) int { return cmp.Compare(a.Criterion, b.Criterion) },
	)
	return out
}

// attested reports a verdict a human attests.
func attested(v *arch.Verdict) bool {
	return slices.ContainsFunc(v.Events, func(e arch.VerificationEvent) bool {
		return e.Role == arch.RoleHuman
	})
}
