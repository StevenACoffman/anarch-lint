# Visibility Counts Exported Elements, Whatever the Package Path

Dowalil's visibility metrics count an element (a package-level type, or a
function that is not a method) as visible when it is exported. A package
under `internal/` does not lower its elements' visibility.

Pinned by `TestADR_0003_VisibilityCountsExportedElements` in
`internal/fitness`.

## Context

Go's `internal/` rule limits importers to the parent's tree. An analysis of
one module is mostly that tree, so to every package anarch-lint measures, an
exported element of an internal package is as visible as any other. Counting
`internal/` as hidden would reward moving code under `internal/` without
changing what the module's own packages can reach.

## Decision

**Visible means exported, nothing more.** The choice is listed among the
fitness report's assumptions as `visibility`.

## Consequences

Relative visibility falls only when a package stops exporting elements, which is
the change the metric is meant to reward.
