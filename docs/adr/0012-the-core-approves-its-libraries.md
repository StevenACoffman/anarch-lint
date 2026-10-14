# The Core Approves Its Libraries

The functional core, `internal/arch` and the measures, may import
third-party code that is on an allow-list in `anarch.json`, the fitness
`libraries` rule `core-approved-libraries`. Each approval is a decision; the
list replaces the earlier ban on every import under `github.com/...`,
`golang.org/x/...` and `pgregory.net/...`.

Pinned by `TestADR_0012_TheCoreApprovesItsLibraries` in `main_test.go`.

## Context

The core was kept free of third-party code so that it stays pure: no I/O, no
clock, no randomness, hence deterministic output (`mmi-plan.md` §2;
`summary_rules.md` §2 keeps the domain free of third-party imports for the
same reason). The ban also kept out pure libraries, so the core grew its own
Tarjan, Johnson, dominator and PageRank code, and the plan for communities
and file globs would have added more. Bespoke algorithms are code to test
and maintain that well-tested libraries already provide.

Purity is a property of the code imported, not of who wrote it. The calls
rule `no-clock-or-os-in-core` and the forbidden imports of `os`, `time` and
`math/rand` still apply to the core's own code; a library is approved only
when it keeps those properties itself. The core may use `math/rand/v2` only
through a source seeded with constants, which gonum's Louvain needs to order
its moves: the calls rule forbids every function that reads the global
source.

## Decision

**A library may enter the core when it is pure** (no I/O, no clock, no
global randomness, no cgo), **deterministic for a fixed input and version,
permissively licensed, widely used and tested.** Each approval is an `allow`
pattern of `core-approved-libraries`, pinned in `go.mod`; anything else
outside the standard library is an `unapproved-library` finding, and an
approval nothing imports is an `unused-approval` warning.

Approved now:

- `gonum.org/v1/gonum/...` (BSD-3-Clause): graph algorithms and community
  detection.
- `github.com/bmatcuk/doublestar/v4` (MIT): file globs.

Adapters need no approval: they may import what they translate, such as
`golang.org/x/tools` in the loader.

## Consequences

Adopting a library in the core is a reviewed change to `anarch.json` and
this list. A library's upgrade can change results, such as a community
partition, so its version is pinned and golden files are re-recorded with
the upgrade.
