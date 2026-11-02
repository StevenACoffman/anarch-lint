# Copies Are Facts; Shared Shapes Are Candidates

Within a group of function bodies of one shape, the bodies that are copies of
each other, in two or more packages, are a fact of connascence of algorithm:
it needs no judgement slot, and it is a finding when it exceeds the
connascence limits. The bodies that share only the shape stay a candidate,
with low confidence, for a reviewer to tell.

Pinned by `TestADR_0013_CopiesAreFactsAndShapesCandidates` in
`internal/connascence`.

## Context

`arch.Func.BodyHash` hashes a body's shape, dropping every identifier and
literal value, so `algorithm.clone` could not tell a copy from a
coincidence of shape. It asked a reviewer about every group, including
groups whose bodies were the same text, which a reviewer can only confirm
(`mmi-plan.md` §9.3 H: settle what code can settle before asking).

## Decision

**Hash the body twice.** `ExactHash` hashes the body as `go/format` prints
it, for the same bodies `BodyHash` covers. A block statement prints without
its comments, and the printer's blank lines are dropped, so neither a comment
nor a blank line tells two copies apart: a copy with a comment added is
still a copy.

**Split each shape group by the exact hash.** Copies in two or more packages
are one fact each; the rest, if they span two or more packages, are one
candidate. The detector and its kind stay the same. A fingerprint hashes the
rule, the subject (the part's first function) and the detector, so the part
that keeps the group's first function keeps the group's fingerprint, and the
other part gets a new one with no verdict yet.

## Consequences

A group that was wholly copies keeps its fingerprint and changes only its
basis, so a recorded verdict on it no longer matters. Two bodies that differ
only in a line break inside an expression, which `go/format` keeps, are not
copies; that errs toward asking.
