# "Cannot Tell" Is an Answer That Decides Nothing

Every judgement slot accepts `insufficient-evidence` besides its own
answers, and an MMI verdict may give it instead of a score. An approved
verdict that gives it leaves the slot `unresolved`: it confirms and rejects
nothing, suppresses nothing, and leaves an MMI criterion without a score, so the
MMI interval stays as wide as for a pending slot.

Pinned by `TestADR_0009_InsufficientEvidenceDecidesNothing` in
`internal/arch`.

## Context

Every family's answers forced a choice, and an MMI slot forced a score
(`mmi-plan.md` §9.1, gap G4). A reviewer, or a judge, that cannot tell
would then guess, and the guess would gate. skillsaw states the cost of the
alternative: an unresolved result "is not a pass… or the gate quietly
becomes optional on exactly the small samples where it matters most".

## Decision

**Give the honest exit a name, and make it decide nothing.** It is one
constant, `arch.AnswerInsufficient`, added to every family's answers in one
place, so no family can lack it.

## Consequences

Adding the answer changed every family's edition, so verdicts recorded
before it went stale (ADR 0007). A slot left unresolved asks for more or
better evidence, which is a change to the measure, not to the verdict.
