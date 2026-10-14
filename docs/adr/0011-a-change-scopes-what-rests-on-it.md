# A Change Scopes What Rests on It

Under `check --new-from-rev`, a finding counts when its location, or any
place it rests on, is on a changed line. A subsystem cycle rests on every
import between two of its subsystems, and a type cycle on every reference
within its group.

Pinned by `TestADR_0011_AChangeScopesWhatRestsOnIt` in `internal/arch`.

## Context

A cycle finding's location is the first import or reference of its minimum
cut, which is where to fix it, not where it was closed. A change that closed
a subsystem cycle by adding an import elsewhere in the group was counted out
of scope, and only the `subsystems.inCycles` metric failed the gate, without
naming the cycle (`IMPROVEMENTS.md` 2.4).

## Decision

**A finding keeps every site it rests on, and the scoped gate asks whether a
change touches any of them.** The sites are unexported: only the gate reads
them, so neither the report nor the fingerprint changes. A finding with no
location and no site, such as one about the model, still belongs to no
change: a model change makes the baseline incomparable, so such findings are
re-baselined with the model that causes them.

## Consequences

A change that closes a cycle sees the cycle as its own new finding, with the
cut that breaks it. A change that only touches an unrelated edge of an
existing cycle changes nothing, since the cycle is in the baseline already.
