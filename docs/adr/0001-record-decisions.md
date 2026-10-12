# Record Decisions Alongside the Code

anarch-lint records each decision that a reasonable contributor might undo as
a numbered record in `docs/adr/`. A record that a change could undo silently
is pinned by a test named `TestADR_NNNN_…`, which fails if the decision is
undone.

## Context

Several of anarch-lint's measures make choices that look like bugs to a
reader who has not seen the reasoning: a degree that is allowed to grow, a
visibility metric that ignores `internal/`, a rule that fails for matching
nothing. Each choice lives in a code comment and an assumption in the JSON
report, but a comment does not stop a well-meant fix from reversing it.
modelith pins its records the same way (its ADR-0001 and its `TestADR_`
tests).

## Decision

**A record states the decision first, then the context, then the
consequences.** Records are never edited to reverse a decision: a new record
supersedes the old one and says so.

**Every record after this one names the test that pins it,** and a test in
`main_test.go` fails when a record has no such test.

## Consequences

Undoing a decision means deleting or changing a test whose name says which
record it pins, so the reversal shows in review.
