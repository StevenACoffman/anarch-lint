package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// The queue groups the open slots by the group their measure gave them, the
// largest group first; a slot with no group stands alone, and a supplied
// slot is no longer open.
func TestQueueOf(t *testing.T) {
	t.Parallel()
	slot := func(criterion, group string, status arch.SlotStatus) arch.JudgementSlot {
		return arch.JudgementSlot{Criterion: criterion, Group: group, Status: status}
	}
	q := arch.QueueOf([]arch.JudgementSlot{
		slot("connascence/3", "algorithm.clone in m/a, m/b", arch.SlotPending),
		slot("connascence/1", "algorithm.clone in m/a, m/b", arch.SlotStale),
		slot("subdomain/billing", "", arch.SlotPending),
		slot("connascence/2", "algorithm.clone in m/a, m/b", arch.SlotSupplied),
	})
	equals(t, arch.Queue{Answers: 2, Slots: 3, Groups: []arch.SlotGroup{
		{
			Group:    "algorithm.clone in m/a, m/b",
			Criteria: []string{"connascence/1", "connascence/3"},
		},
		{Group: "subdomain/billing", Criteria: []string{"subdomain/billing"}},
	}}, q)
}
