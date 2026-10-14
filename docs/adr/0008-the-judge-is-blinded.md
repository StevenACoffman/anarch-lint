# The Judge Is Blinded

A judgement slot splits what it shows into evidence and context. A judge
sees only the question, the answers and the evidence, numbered as exhibits
`E1`, `E2`, … through `JudgeView`. The context holds what an answer would
set off: the limits a connascence candidate exceeds, and a Balanced
Coupling candidate's distance, volatility and score. A human reviewer sees
both, and both are hashed.

Pinned by `TestADR_0008_TheJudgeIsBlinded` and `TestJudgeViewShape` in
`internal/arch`.

## Context

A connascence candidate's evidence said "above maxStrength.system 3", and a
balance candidate's carried its balance score: both told a judge which
answer gates (`mmi-plan.md` §9.1, gap G2). A judge that can see the
consequence can be swayed by it. gnosis builds its critic's input as a type
with no field a warrant could travel in, and tests it with markers.

## Decision

**Make blinding structural.** `JudgeView` has no field for the context, the
status, an earlier verdict or a rationale, and a test pins its fields. A
marker test fills every hidden field and asserts that none reaches the
view. Each measure that opens slots puts its consequences in `Context`, and
tests in `internal/connascence` and `internal/balance` check that their
exhibits name no limit, distance or score.

## Consequences

A judge asked whether a candidate is real answers from the sites and the
message alone. Changing the context still makes verdicts stale, since a
change in what an answer sets off is a reason to judge again.
