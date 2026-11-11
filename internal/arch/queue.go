package arch

import (
	"cmp"
	"slices"
)

// Queue groups the open slots, pending or stale, that one reviewer's answer
// would settle together, largest group first, so a reviewer asks first the
// question that settles the most (Raghothaman et al., PLDI 2018;
// IMPROVEMENTS.md §10.8). It only orders the questions and answers none:
// each slot still takes its own verdict, so the gate stays as deterministic
// as before. Answers counts the groups: the answers that would clear the
// backlog if each group's slots share one.
type Queue struct {
	Answers int         `json:"answers"`
	Slots   int         `json:"slots"`
	Groups  []SlotGroup `json:"groups"`
}

// SlotGroup is the open slots of one group, by criterion.
type SlotGroup struct {
	Group    string   `json:"group"`
	Criteria []string `json:"criteria"`
}

// QueueOf groups the open slots by the group their measure gave them; a
// slot with none is a group of its own. Groups are ordered by size, then by
// name, and each lists its criteria sorted.
func QueueOf(slots []JudgementSlot) Queue {
	byGroup := map[string][]string{}
	q := Queue{Groups: make([]SlotGroup, 0)}
	for i := range slots {
		s := &slots[i]
		if s.Status != SlotPending && s.Status != SlotStale {
			continue
		}
		g := cmp.Or(s.Group, s.Criterion)
		byGroup[g] = append(byGroup[g], s.Criterion)
		q.Slots++
	}
	for g, criteria := range byGroup {
		q.Groups = append(
			q.Groups,
			SlotGroup{Group: g, Criteria: slices.Sorted(slices.Values(criteria))},
		)
	}
	slices.SortFunc(q.Groups, func(a, b SlotGroup) int {
		return cmp.Or(cmp.Compare(len(b.Criteria), len(a.Criteria)), cmp.Compare(a.Group, b.Group))
	})
	q.Answers = len(q.Groups)
	return q
}
