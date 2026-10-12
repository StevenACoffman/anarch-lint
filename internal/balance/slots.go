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

// ask returns the approved answer to the slot id, creating the slot with
// evidence the first time it is asked. Every caller of one ID passes the
// same evidence, since the evidence is a function of the program alone.
func (b *slotBook) ask(id string, evidence []string) string {
	slot, ok := b.byID[id]
	if !ok {
		slot = arch.NewSlot(id, evidence, b.verdicts)
		b.byID[id] = slot
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

// all returns every slot asked, sorted by ID.
func (b *slotBook) all() []arch.JudgementSlot {
	out := make([]arch.JudgementSlot, 0, len(b.byID))
	for _, id := range slices.SortedFunc(maps.Keys(b.byID), cmp.Compare[string]) {
		out = append(out, b.byID[id])
	}
	return out
}
