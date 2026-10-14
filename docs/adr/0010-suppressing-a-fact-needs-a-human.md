# Suppressing a Fact Needs a Human

A verdict can lift a Balanced Coupling finding that rests on facts: a
supporting or generic subdomain lifts tight coupling to it, and a shared
kernel, partnership or conformist pattern lifts a leaked model. Such a
verdict applies only when a human event attests it, and every lift is
listed in the report's `suppressed`, with the slot, the answer and the
events.

Pinned by `TestADR_0010_SuppressingAFactNeedsAHuman` in `internal/balance`.

## Context

One verdict could remove a finding the code proves, and the report kept no
trace of it (`mmi-plan.md` §9.1, gap G7). Confirming a candidate and
suppressing a fact are not symmetric: a wrong confirmation adds a finding
someone will look at, while a wrong suppression hides one nobody will.
agentic-dev-harness keeps the same asymmetry the other way round: "a
finding never blocks on the critic's text alone".

## Decision

**Require a human event to suppress, whatever a family's policy, and show
every suppression.** Today every family needs a human event to apply at
all. When a family is promoted to automatic judging (`mmi-plan.md` §9.3 D,
F), the promotion must not reach the suppressing slots, and this test fails
if it does.

## Consequences

A reviewer can audit every lifted finding from the report alone. Automated
judging, when it comes, can confirm candidates but cannot clear facts.
