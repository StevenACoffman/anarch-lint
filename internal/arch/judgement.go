package arch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Slot statuses.
const (
	// SlotPending is a slot no verdict addresses.
	SlotPending SlotStatus = "pending"
	// SlotSupplied is a slot filled by an approved verdict for the current
	// evidence.
	SlotSupplied SlotStatus = "supplied"
	// SlotStale is a slot whose verdict judged different evidence: the code
	// changed since, so the verdict no longer applies.
	SlotStale SlotStatus = "stale"
	// SlotUnapproved is a slot whose verdict nobody has approved yet, such as
	// a draft from an LLM.
	SlotUnapproved SlotStatus = "unapproved"
)

// Provenances of a verdict.
const (
	ProvenanceHuman Provenance = "human"
	ProvenanceLLM   Provenance = "llm"
)

// judgementsVersion is the only judgements-file version this build reads.
const judgementsVersion = 1

// SlotStatus says whether a judgement slot has a verdict that applies.
type SlotStatus string

// Provenance says who produced a verdict.
type Provenance string

// JudgementSlot is a question waiting for a reviewer: a human expert or an
// adversarial LLM (mmi.md §7.6). It carries the question, the deterministic
// evidence a reviewer should weigh, and a hash of that evidence. A verdict
// applies only to the evidence it was given, so a code change that alters the
// evidence makes the verdict stale instead of silently reusing it.
//
// A slot is either an MMI criterion, such as "1.1.5", answered with a score,
// or a member of a family, such as "subdomain/billing", answered with one of
// Answers (connascence.md §5, §9). A slot never gates by itself: an MMI
// verdict narrows the MMI interval, and an approved family verdict can turn a
// candidate into a finding or suppress one.
type JudgementSlot struct {
	// Criterion is the slot's ID: an MMI criterion, or "family/subject".
	Criterion string `json:"criterion"`
	Question  string `json:"question"`
	// Answers lists the valid answers of a family slot; an MMI slot takes a
	// score from 0 to 10 instead.
	Answers      []string   `json:"answers,omitempty"`
	Evidence     []string   `json:"evidence"`
	EvidenceHash string     `json:"evidenceHash"`
	Status       SlotStatus `json:"status"`
	Verdict      *Verdict   `json:"verdict,omitempty"`
}

// Verdict is one reviewer's answer to a judgement slot: a Score for an MMI
// criterion, an Answer for a family slot, never both.
type Verdict struct {
	Criterion  string     `json:"criterion"`
	Score      int        `json:"score"`
	Answer     string     `json:"answer,omitempty"`
	Provenance Provenance `json:"provenance"`
	Rationale  string     `json:"rationale"`
	// EvidenceHash must equal the slot's hash for the verdict to apply.
	EvidenceHash string `json:"evidenceHash"`
	// Approved records that a human accepted the verdict; an LLM's verdict
	// stays a draft until then.
	Approved bool `json:"approved"`
}

// question is what a slot asks, and the answers it accepts: nil for an MMI
// criterion, which takes a score.
type question struct {
	text    string
	answers []string
}

// judgementsFile is the on-disk form of a set of verdicts.
type judgementsFile struct {
	Version  int       `json:"version"`
	Verdicts []Verdict `json:"verdicts"`
}

// DecodeJudgements reads a judgements file: {"version": 1, "verdicts": [...]}.
// An unknown slot, two verdicts for one slot, an unknown provenance, a score
// outside 0–10, an answer on an MMI criterion, and a score or an invalid
// answer on a family slot are all EINVALID.
func DecodeJudgements(r io.Reader) ([]Verdict, error) {
	const op = "arch.DecodeJudgements"
	invalid := func(format string, args ...any) error {
		return &Error{
			Op:  op,
			Err: &Error{Code: EINVALID, Message: "judgements: " + fmt.Sprintf(format, args...)},
		}
	}
	var f judgementsFile
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, invalid("%v", err)
	}
	if f.Version != judgementsVersion {
		return nil, invalid("version %d is not supported; want %d", f.Version, judgementsVersion)
	}
	seen := map[string]bool{}
	for i := range f.Verdicts {
		v := &f.Verdicts[i]
		if seen[v.Criterion] {
			return nil, invalid("criterion %s has two verdicts", v.Criterion)
		}
		if msg := v.problem(); msg != "" {
			return nil, invalid("criterion %s: %s", v.Criterion, msg)
		}
		seen[v.Criterion] = true
	}
	return f.Verdicts, nil
}

// problem returns what makes v unusable, or "" when nothing does.
func (v *Verdict) problem() string {
	q, ok := questionOf(v.Criterion)
	switch {
	case !ok:
		return "not a judgement criterion or slot family"
	case v.Provenance != ProvenanceHuman && v.Provenance != ProvenanceLLM:
		return fmt.Sprintf("provenance %q is not human or llm", v.Provenance)
	case q.answers == nil:
		return v.scoreProblem()
	default:
		return v.answerProblem(q.answers)
	}
}

// scoreProblem checks the verdict of an MMI criterion: a score, no answer.
func (v *Verdict) scoreProblem() string {
	switch {
	case v.Answer != "":
		return "an MMI criterion takes a score, not an answer"
	case v.Score < 0 || v.Score > 10:
		return fmt.Sprintf("score %d is outside 0–10", v.Score)
	}
	return ""
}

// answerProblem checks the verdict of a family slot: one of answers, no score.
func (v *Verdict) answerProblem(answers []string) string {
	switch {
	case v.Score != 0:
		return "a slot family takes an answer, not a score"
	case !slices.Contains(answers, v.Answer):
		return fmt.Sprintf("answer %q is not one of %s", v.Answer, strings.Join(answers, ", "))
	}
	return ""
}

// NewSlot returns the slot of a criterion with its evidence and hash, filled
// by whichever verdict applies.
//
// Requires: criterion is an MMI judgement criterion or "family/subject" of a
// known family.
func NewSlot(criterion string, evidence []string, verdicts []Verdict) JudgementSlot {
	sum := sha256.Sum256([]byte(criterion + "\n" + strings.Join(evidence, "\n")))
	q, _ := questionOf(criterion)
	slot := JudgementSlot{
		Criterion:    criterion,
		Question:     q.text,
		Answers:      q.answers,
		Evidence:     evidence,
		EvidenceHash: hex.EncodeToString(sum[:]),
		Status:       SlotPending,
	}
	for i := range verdicts {
		v := verdicts[i]
		if v.Criterion != criterion {
			continue
		}
		slot.Verdict = &v
		switch {
		case v.EvidenceHash != slot.EvidenceHash:
			slot.Status = SlotStale
		case !v.Approved:
			slot.Status = SlotUnapproved
		default:
			slot.Status = SlotSupplied
		}
	}
	return slot
}

// Answer returns the answer of an approved verdict that judged the slot's
// current evidence, and "" for a pending, stale or unapproved slot: the only
// verdicts a measure may act on.
func (slot *JudgementSlot) Answer() string {
	if slot.Status != SlotSupplied {
		return ""
	}
	return slot.Verdict.Answer
}

// apply scores a judgement criterion from its slot, when the slot is supplied.
func (slot *JudgementSlot) apply(c *Criterion) {
	switch slot.Status {
	case SlotSupplied:
		score := slot.Verdict.Score
		c.Score = &score
		c.Reason = fmt.Sprintf("scored by an approved %s verdict", slot.Verdict.Provenance)
	case SlotStale:
		c.Reason = "the verdict judged different evidence; re-review the slot"
	case SlotUnapproved:
		c.Reason = "the verdict awaits approval"
	case SlotPending:
		c.Reason = "needs a reviewer; see the judgement slot"
	}
}

// questionOf returns the question of a slot ID: an MMI judgement criterion,
// or "family/subject" for one of the families of connascence.md §5 and §9.
func questionOf(id string) (question, bool) {
	if family, subject, ok := strings.Cut(id, "/"); ok {
		q, known := familyQuestion(family)
		return q, known && subject != ""
	}
	text := mmiQuestion(id)
	return question{text: text}, text != ""
}

// familyQuestion returns the question a slot family asks of its subject.
func familyQuestion(family string) (question, bool) {
	confirm := []string{"confirmed", "rejected"}
	q, ok := map[string]question{
		"connascence": {
			"Must these places change together, so that the candidate is connascence?",
			confirm,
		},
		"subdomain": {
			"Is this subsystem a core, supporting or generic subdomain of the business?",
			[]string{"core", "generic", "supporting"},
		},
		"bounded-context": {
			"Is this subsystem a bounded context, with a model of its own?",
			[]string{"no", "yes"},
		},
		"integration-pattern": {
			"Which DDD integration pattern do these two subsystems follow?",
			[]string{
				"anti-corruption-layer", "conformist", "customer-supplier",
				"open-host-service", "partnership", "separate-ways", "shared-kernel",
			},
		},
		"contract-type": {
			"Is this exported type part of the domain model or an integration contract?",
			[]string{"contract", "model"},
		},
		"async": {
			"Does this package integrate asynchronously, through the messaging it imports?",
			[]string{"async", "sync"},
		},
		"balance": {
			"Is this Balanced Coupling candidate real?",
			confirm,
		},
	}[family]
	return q, ok
}

// mmiQuestion returns the reviewer's question for an MMI judgement criterion,
// from the MMI chapter and mmi.md §7.6, or "" for any other criterion.
func mmiQuestion(criterion string) string {
	return map[string]string{
		"1.1.5": "Does each subsystem, layer, package and type have one clear responsibility, " +
			"and does its name describe it?",
		"1.1.6": "Do the packages map the technical layers and the subsystems (domain modules) " +
			"of the target architecture, and is that the right mapping?",
		"1.2.2": "Is each subsystem's internal interface mapped through packages that other " +
			"subsystems can see and use?",
		"3.3": "Are the patterns mapped explicitly, through type names, embedding or " +
			"directives, rather than left for readers to infer?",
		"3.4": "Are domain and technical code separated, as DDD, Quasar or hexagonal " +
			"architecture separate them?",
	}[criterion]
}
