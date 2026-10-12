# The Degree Ratchet Skips Name and Type

`check` ratchets the degree of each connascence fact of meaning or stronger at
a subsystem boundary or beyond. The degree of connascence of name and of type
may grow without failing the gate.

Pinned by `TestADR_0002_DegreeRatchetSkipsNameAndType` in
`internal/connascence`.

## Context

The degree of a connascence instance counts the entities that share the
knowledge. For name and type, the weakest kinds, each new caller of an
exported function adds to the degree: that is ordinary use of an interface,
not degradation (connascence.md §2.1, §2.2). For meaning, position, algorithm
and the dynamic kinds, each new site spreads knowledge that should have one
home (§6).

## Decision

**Ratchet only meaning and stronger, and only at subsystem locality or
beyond.** Inside a package or a type, strong connascence is acceptable, as
connascence.io has it, so its degree is not gated either.

## Consequences

A change that adds the hundredth caller of an exported function passes the
gate. A change that adds a site of a shared magic value across subsystems
fails it, until the value gets a name.
