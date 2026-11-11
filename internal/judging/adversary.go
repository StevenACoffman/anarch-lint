package judging

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// adversaryItems returns, for each adversary configuration and sample, an
// item for each slot whose judges' draft awaits a human: unapproved, for
// the current evidence, with a substantive answer the adversary has not yet
// challenged. The item shows the judges' coverage, never their answers or
// rationales (mmi-plan.md §9.3 G).
func adversaryItems(slots []arch.JudgementSlot, adversaries []Config, budget Budget) []Item {
	var out []Item
	for i := range slots {
		s := &slots[i]
		if s.Status != arch.SlotUnapproved || s.Verdict == nil ||
			s.Verdict.Answer == arch.AnswerInsufficient {
			continue
		}
		coverage := coverageOf(s.Verdict)
		for j := range adversaries {
			c := &adversaries[j]
			if challenged(s.Verdict, c.Hash()) {
				continue
			}
			for sample := range 2 {
				it := question{KindAdversary, s}.item(c, sample, budget)
				it.Key = shortHash(it.Key, string(KindAdversary))
				it.Coverage = &coverage
				out = append(out, it)
			}
		}
	}
	return out
}

// coverageOf merges the coverage of a verdict's judge events.
func coverageOf(v *arch.Verdict) Coverage {
	c := Coverage{Examined: make([]string, 0), Unexamined: make([]string, 0)}
	for i := range v.Events {
		if e := &v.Events[i]; e.Role == arch.RoleJudge {
			c.Examined = append(c.Examined, e.Examined...)
			c.Unexamined = append(c.Unexamined, e.Unexamined...)
		}
	}
	slices.Sort(c.Examined)
	slices.Sort(c.Unexamined)
	c.Examined, c.Unexamined = slices.Compact(c.Examined), slices.Compact(c.Unexamined)
	return c
}

// challenged reports a verdict an adversary of the configuration already
// challenged.
func challenged(v *arch.Verdict, config string) bool {
	return slices.ContainsFunc(v.Events, func(e arch.VerificationEvent) bool {
		return e.Role == arch.RoleAdversary && e.Config == config
	})
}

// challenge records each adversary's answers on the judges' draft they
// challenge, returning the verdicts and a note per challenge. An adversary
// that answers otherwise, abstaining included, unsettles the draft into
// insufficient-evidence, which decides nothing (docs/adr/0009); one that
// agrees adds its event. A verdict a human attests is never changed, and a
// disagreement with it is only a note. A draft whose evidence changed since
// the run is left alone.
func challenge(
	verdicts []arch.Verdict,
	answers []answer,
	configs []Config,
) ([]arch.Verdict, []string) {
	hashes := map[string]string{}
	for i := range configs {
		hashes[configs[i].Name] = configs[i].Hash()
	}
	out := slices.Clone(verdicts)
	var notes []string
	for i := range answers {
		a := &answers[i]
		k := slices.IndexFunc(out, func(v arch.Verdict) bool {
			return v.Criterion == a.item.View.ID && v.EvidenceHash == a.item.EvidenceHash
		})
		if k < 0 {
			continue
		}
		v := &out[k]
		agrees := answerText(&a.reply) == verdictText(v)
		switch {
		case attested(v):
			if !agrees {
				notes = append(notes, fmt.Sprintf("adversary %s answers %s to %s, which a human "+
					"attested as %s", a.item.Config, answerText(&a.reply), v.Criterion, verdictText(v)))
			}
			continue
		case !agrees:
			notes = append(notes, fmt.Sprintf("adversary %s answers %s to %s, unsettling the "+
				"judges' %s", a.item.Config, answerText(&a.reply), v.Criterion, verdictText(v)))
			v.Rationale = fmt.Sprintf("challenged by adversary %s (%s), against the judges' %s: %s",
				a.item.Config, answerText(&a.reply), verdictText(v), a.reply.Rationale)
			v.Answer, v.Score = arch.AnswerInsufficient, 0
		}
		v.Events = append(slices.Clone(v.Events), arch.VerificationEvent{
			Role: arch.RoleAdversary, Config: hashes[a.item.Config],
			Confidence: a.reply.Confidence, Cites: a.reply.Cites,
		})
	}
	return out, notes
}

// verdictText is how a challenge compares a verdict: its answer, or its
// score, as answerText spells a reply's.
func verdictText(v *arch.Verdict) string {
	if v.Answer == "" {
		return "score " + strconv.Itoa(v.Score)
	}
	return v.Answer
}
