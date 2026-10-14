package balance

import (
	"cmp"
	"maps"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// slotBook creates each judgement slot once, with its evidence, and answers
// from the verdicts that judged that evidence.
type slotBook struct {
	verdicts []arch.Verdict
	byID     map[string]arch.JudgementSlot
}

// ask returns the approved answer to the slot req asks for, creating the
// slot the first time it is asked. Every request for one criterion carries
// the same evidence, since the evidence is a function of the program alone.
func (b *slotBook) ask(req *arch.SlotRequest) string {
	slot, ok := b.byID[req.Criterion]
	if !ok {
		slot = arch.NewSlot(req, b.verdicts)
		b.byID[req.Criterion] = slot
	}
	return slot.Answer()
}

// answered returns the approved answer of a slot already asked, and "" for
// one never asked: a measure that only reads a judgement must not create its
// slot without evidence.
func (b *slotBook) answered(id string) string {
	slot, ok := b.byID[id]
	if !ok {
		return ""
	}
	return slot.Answer()
}

// suppression records that the answer of the slot id lifted rule for the
// pair. The slot must have been asked and supplied.
func (b *slotBook) suppression(id, rule string, p *Pair) Suppression {
	slot := b.byID[id]
	return Suppression{
		Rule: rule, Subject: p.From, Object: p.To, Criterion: id,
		Answer: slot.Answer(), Events: slot.Verdict.Events,
	}
}

// all returns every slot asked, sorted by ID.
func (b *slotBook) all() []arch.JudgementSlot {
	out := make([]arch.JudgementSlot, 0, len(b.byID))
	for _, id := range slices.SortedFunc(maps.Keys(b.byID), cmp.Compare[string]) {
		out = append(out, b.byID[id])
	}
	return out
}
