package arch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

// JudgementSlot is a qualitative MMI criterion waiting for a reviewer: a human
// expert or an adversarial LLM (mmi.md §7.6). It carries the question, the
// deterministic evidence a reviewer should weigh, and a hash of that evidence.
// A verdict applies only to the evidence it was given, so a code change that
// alters the evidence makes the verdict stale instead of silently reusing it.
//
// Slots never feed a gate; they only narrow the MMI interval.
type JudgementSlot struct {
	Criterion    string     `json:"criterion"`
	Question     string     `json:"question"`
	Evidence     []string   `json:"evidence"`
	EvidenceHash string     `json:"evidenceHash"`
	Status       SlotStatus `json:"status"`
	Verdict      *Verdict   `json:"verdict,omitempty"`
}

// Verdict is one reviewer's answer to a judgement slot.
type Verdict struct {
	Criterion  string     `json:"criterion"`
	Score      int        `json:"score"`
	Provenance Provenance `json:"provenance"`
	Rationale  string     `json:"rationale"`
	// EvidenceHash must equal the slot's hash for the verdict to apply.
	EvidenceHash string `json:"evidenceHash"`
	// Approved records that a human accepted the verdict; an LLM's verdict
	// stays a draft until then.
	Approved bool `json:"approved"`
}

// judgementsFile is the on-disk form of a set of verdicts.
type judgementsFile struct {
	Version  int       `json:"version"`
	Verdicts []Verdict `json:"verdicts"`
}

// DecodeJudgements reads a judgements file: {"version": 1, "verdicts": [...]}.
// A score outside 0–10, an unknown provenance or criterion, or two verdicts
// for one criterion is EINVALID.
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
	for _, v := range f.Verdicts {
		switch {
		case questionOf(v.Criterion) == "":
			return nil, invalid("criterion %q is not a judgement criterion", v.Criterion)
		case seen[v.Criterion]:
			return nil, invalid("criterion %s has two verdicts", v.Criterion)
		case v.Score < 0 || v.Score > 10:
			return nil, invalid("criterion %s: score %d is outside 0–10", v.Criterion, v.Score)
		case v.Provenance != ProvenanceHuman && v.Provenance != ProvenanceLLM:
			return nil, invalid(
				"criterion %s: provenance %q is not human or llm",
				v.Criterion,
				v.Provenance,
			)
		}
		seen[v.Criterion] = true
	}
	return f.Verdicts, nil
}

// newSlot returns the slot of a criterion with its evidence and hash, filled
// by whichever verdict applies.
func newSlot(criterion string, evidence []string, verdicts []Verdict) JudgementSlot {
	sum := sha256.Sum256([]byte(criterion + "\n" + strings.Join(evidence, "\n")))
	slot := JudgementSlot{
		Criterion:    criterion,
		Question:     questionOf(criterion),
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

// questionOf returns the reviewer's question for a judgement criterion, from
// the MMI chapter and mmi.md §7.6, or "" for any other criterion.
func questionOf(criterion string) string {
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
