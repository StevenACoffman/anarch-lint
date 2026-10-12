package balance_test

import (
	"slices"
	"testing"
)

// docs/adr/0005: what a domain model implies adds evidence and answers no
// slot, so adding domain models changes no pair, finding, candidate or
// subsystem, only the evidence reviewers judge.
func TestADR_0005_DomainInferencesAreEvidence(t *testing.T) {
	t.Parallel()
	without, with := analyze(fixture(), clone()), analyze(withDomains(), clone())
	equals(t, without.Pairs, with.Pairs)
	equals(t, without.Findings, with.Findings)
	equals(t, without.Candidates, with.Candidates)
	equals(t, without.Subsystems, with.Subsystems)
	slot, _ := slotOf(with.Judgements, "bounded-context/auth")
	equals(t, true, slices.Contains(slot.Evidence,
		"described by domain model models/auth.modelith.yaml"))
	equals(t, "", slot.Answer())
}
