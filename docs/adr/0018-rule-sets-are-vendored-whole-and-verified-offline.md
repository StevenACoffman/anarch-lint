# Rule Sets Are Vendored Whole and Verified Offline

A model may include rule sets, files of rules that repositories share, by
path in `ruleSets`. A copy vendored from another repository carries
modelith's provenance header, and anarch-lint refuses the model when the
copy no longer hashes to the digest its header records.

Pinned by `TestADR_0018_RuleSetsAreVendoredWholeAndVerifiedOffline` in
`internal/pipeline`.

## Context

`TODO.md` asked to vendor shared rule sets with modelith's provenance header
and digest (modelith ADR-0015 to ADR-0017). modelith vendors a whole file,
stamped with a header that names its origin, ref and commit, and a SHA-256
over its bytes without the header lines. `lint` re-checks that digest
offline, so an edit to the copy shows as drift. anarch-lint already reads
modelith's code through `indomain`, whose `pkg/provenance` parses and
verifies the header.

## Decision

**A rule set is a whole file, included by path, merged after the model's own
rules.** It holds only rules that stand on their own patterns: forbidden
imports, isolations, and the fitness rules over packages, callees and files.
A rule over the including model's layers, subsystems or contract files would
tie the shared file to one model, so the decoder refuses it. Every fitness
rule in a set has a name, and the sets merge in path order, the order the
hash reads them in, so including a set never renames a rule.

**A vendored copy is verified offline, the way modelith verifies its
models.** A set with no header is the repository's own. One with a header
must have a sound header and match its digest; otherwise anarch-lint refuses
the model as `bad-provenance`, and the message names the digest it computed
and the origin to restore from. Nothing fetches: modelith's `deps import`
copies models only, and fetching would bring the network into a tool that
never touches it.

**The merged rules are the model.** The hash covers them, and the check for
loosened bars reads the sets at the base revision too, so removing a rule
from a shared set loosens the bars like removing one from the model.

## Consequences

Only a YAML rule set can be vendored, since the header is a comment. Someone
writes a vendored copy by hand or with a tool outside anarch-lint, and a
digest mismatch prints the digest to record. A change upstream reaches a
repository only when someone copies it again.
