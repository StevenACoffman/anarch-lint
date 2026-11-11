# An Adversary Can Only Unsettle

`judge emit --adversary NAME=MODEL` asks an adversary about each slot whose
judges' draft awaits a human: unapproved, for the current evidence, with a
substantive answer. The prompt shows the question, the exhibits and what the
judges examined and could not, never their answers or rationales. `judge
admit` adds the adversary's event to the draft. An adversary that answers
otherwise turns the draft into `insufficient-evidence`; one that agrees only
adds its event. A verdict a human attests is never changed, and an
adversary disagreeing with it is a note.

Pinned by `TestADR_0020_AnAdversaryCanOnlyUnsettle` in `internal/judging`.

## Context

`mmi-plan.md` §9.3 G plans an adversary beside the judges: "only coverage
crosses", so it sees where the judges looked, not what they concluded, and
spends its effort where they did not look. The judges agree unanimously
before a draft is recorded, but agreement between models that share
the same gaps is weak evidence. Canaries qualify a configuration before its
answers may apply, and an adversary prompt, which carries the coverage,
cannot be mixed in with the canaries unseen.

## Decision

**An adversary decides nothing by itself.** It is asked only about a draft,
so there is always an answer to challenge, and its disagreement can only
move that draft to `insufficient-evidence`, which neither gates nor
suppresses (ADR 0009). Since it can never apply an answer, it needs no
canaries: a careless adversary costs a human a second look, never a wrong
verdict. A human's verdict outranks it.

**Coverage is recorded so it can cross.** Each judge event keeps the
judge's `examined` and `unexamined` lists, and the judgements file keeps
them, so an adversary run can follow a judging run. An adversary that has
challenged a draft is not asked about it again.

## Consequences

A human reviewing a draft sees whether an adversary challenged it, with the
adversary's event and rationale. Promoting a family to `automatic` judging
can later require an unchallenged draft; nothing requires it yet, since
only the `human` policy is built.
