# An Empty Fitness Rule Fails Unless Optional

A fitness rule whose subject selectors match nothing is an `empty-selector`
finding, at the rule's gate, unless the rule sets `optional`. The same holds for a
forbidden import, isolation or role whose patterns match nothing.

Pinned by `TestADR_0004_EmptyRuleFailsUnlessOptional` in `internal/fitness`.

## Context

A rule that matches nothing passes forever: usually a typo in a pattern, or a
package that was renamed after the rule was written. ArchUnit fails such a
rule by default (`failOnEmptyShould`), for the same reason.

## Decision

**Fail by default, and let the model say when an empty match is intended,**
as a rule written ahead of the code it governs.

## Consequences

Renaming a package can fail the gate even though no import broke, and the
finding names the rule to correct.
