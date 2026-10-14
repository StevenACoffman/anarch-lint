# The Evidence Hash Covers the Question

A judgement slot's evidence hash covers the question's edition, its
criterion and its evidence. The edition is a digest of the question's
words, its answers or score scale, and a revision turned by hand. A verdict
applies only while the hash matches, so rewording a question, changing its
answers or turning its revision makes every verdict on it stale.

Pinned by `TestADR_0007_EvidenceHashCoversTheQuestion` and
`TestQuestionEditionsArePinned` in `internal/arch`.

## Context

The hash covered only the criterion and the evidence, joined by newlines
(`mmi-plan.md` §9.1, gap G1). A verdict that answered "is this candidate
connascence?" stayed valid after the question changed to something else,
and two evidence lists that differed only in where a newline fell hashed
alike. skillsaw puts the first problem as "right text, wrong rules is as
stale as wrong text".

## Decision

**Hash the edition with the evidence, in an injective encoding.** Each
string is a netstring and each group is prefixed by its length, so no
evidence string can imitate a boundary. A test pins every edition, so a
reworded question fails a test until someone decides its verdicts should
go stale. A verdict may record the edition it answered, and a stale slot
then says whether the question or the evidence changed.

## Consequences

Every verdict recorded before this decision went stale once. The judge's
configuration (its model and prompt) is deliberately not in the hash: a
new model should qualify again on known answers, not void every past
verdict.
