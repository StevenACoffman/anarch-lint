# Improvements from Neighbouring Projects

This file collects ideas for anarch-lint from nine projects in
`~/Documents/git`. Each was read at the revision below, and every claim was
checked a second time against that source by an independent pass; techne's,
read later, were checked by reproducing each on a small module. Paths are
relative to the project's root.

| Project        | Revision   |
| -------------- | ---------- |
| codegrapher    | `b5e264f5` |
| code-graph     | `2a7a52c0` |
| gortex         | `d1e0cf82` |
| codegraph-go   | `65a2b9a9` |
| codetospec     | `2ca222d2` |
| zen-go         | `91f6a7e4` |
| feelc          | `21a1887b` |
| go-rule-engine | `11cd0a9a` |
| techne         | `10397bf`  |

The ideas are grouped by the anarch-lint gap they close, and ranked within each
group. The gaps are the ones in `arch-frameworks.md` ("What None of Them
Covers") and `TODO.md`. Every idea must keep anarch-lint deterministic: no
`time.Now`, no map order, no LLM in the gate.

## Summary of the Projects

| Project        | What it is                                    | Go analysis                                                         | License    | Worth taking                                                     |
| -------------- | --------------------------------------------- | ------------------------------------------------------------------- | ---------- | ---------------------------------------------------------------- |
| gortex         | Code-intelligence engine for agents           | tree-sitter, optional LSP                                           | Apache-2.0 | Co-change mining, contract model, edge kinds, guards, clones     |
| feelc          | DMN/FEEL compiled to Go, with proofs          | n/a                                                                 | Apache-2.0 | Model verification, explanation traces, claims, diagnostic codes |
| codegraph-go   | Code graph for agents                         | cgo tree-sitter; go/types only in its own architecture tests        | MIT        | Gate honesty: locked bars, witness asserts, exclusion ledger     |
| codetospec     | LLM extraction of business rules, with checks | cgo tree-sitter and SCIP; go/packages for routes; go/ast for webapp | MIT        | Judge design: canaries, grounding, cache keys, citations         |
| go-rule-engine | Deterministic typed rule engine               | n/a                                                                 | MIT        | Canonical form, limits, three-valued outcomes, test practice     |
| codegrapher    | SQLite code graph                             | go/parser without go/types, heuristic resolution                    | Apache-2.0 | Edge provenance, annotations, coverage attribution               |
| code-graph     | FalkorDB graph visualiser (Python)            | none: no Go analyzer                                                | MIT        | Small ideas: file-level aggregation, bounded impact              |
| zen-go         | CGO bindings to the Rust ZEN engine           | n/a                                                                 | MIT        | Only the decision-table idea; don't depend on it                 |
| techne         | IDE navigation and refactoring for agents     | go/packages and go/types in process, gopls, cgo tree-sitter         | MIT        | Honest coverage, cgo mapping, excluded files, nested modules     |

Every one of the code graphs resolves Go more weakly than anarch-lint's
`go/packages` and `go/types` loading. What carries over is the analyses built
on a graph and the discipline around gates, not the extraction.

## What `anarch-lint` Already Has

Several ideas below would otherwise repeat work that exists. The ideas build on
these instead:

- **Unused and expired waivers.** `arch.GateResult.UnusedWaivers` lists
  waivers whose fingerprint matches no current finding, and `ExpiredWaivers`
  those past their date (`internal/arch/gate.go`). Waivers are per
  fingerprint, not per rule.
- **Model checks.** `arch.Model.validate` rejects duplicate names, a `$N`
  past an isolation's captures, an unknown subsystem in `subsystemOrder`, and
  fitness rules naming unknown layers. `ambiguous-package` is an error when two
  subsystems or layers match one package equally specifically, and
  `stale-pattern` a warning when a subsystem or layer pattern matches nothing.
- **Empty selectors.** A fitness rule matching nothing is `empty-selector`
  unless `optional` (ADR 0004). `forbiddenImports[].from` and `.imports`,
  `isolations[].from` and the roles' `packages`, `embeds` and `implements` are
  checked too (`internal/fitness/checks.go`), as errors when the model has
  fitness rules and warnings otherwise.
- **A canonical model hash.** `Model.Hash` is the SHA-256 of the *decoded*
  model re-encoded, so whitespace and key order don't change it; reordering a
  list does. The model has a required `version`, rejects unknown fields, and
  its JSON Schema is a golden file.
- **Kinds on facts.** `arch.Ref` has no kind, but `Facts.Uses` carry an
  `ObjectKind` (type, func, method, var, const, field), and `Facts` hold
  `Signatures`, `Calls`, `FieldWrites` (to the receiver's own fields) and
  more. `Type.Embeds` and `Type.Implements` exist; `Implements` is used only
  for roles.
- **Pinned history.** `git.Churn` resolves the revision and HEAD to full
  hashes, records them in the report, and maps each file to its sorted
  commits. Renames are off on purpose, "so the result does not depend on
  git's similarity guesses".
- **Determinism tests.** rapid properties shuffle the input to cycle groups,
  SDI and cuts (`internal/arch/graph_test.go`), elementary cycles
  (`cycles_test.go`), and connascence and fitness (`TestAnalyzeIgnoresInputOrder`).
  Component Rank, LCOM4 and the whole pipeline aren't covered.
- **Test practice.** `FuzzDecodeModel`, golden files for every report and the
  schema, ADR tests (`TestEveryDecisionRecordIsPinned`), a mutation floor of
  95 on `internal/arch`, and diff-scoped mutation of `./internal/...` and
  `./cmd/...` in CI.
- **Counts per fitness rule.** Each fitness result reports `matched` and
  `violations`.
- **Dangling directives.** `//arch:role` naming an unknown role is
  `unknown-role-directive`.
- **A clone detector.** `Func.BodyHash` hashes a body's shape with names and
  literals replaced, for connascence of algorithm.

## Status

Built since this file was written, with where the result departs from the
idea:

| Idea | Built                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1.1  | In `fitness`: `redundant-rule` and `unusable-allowance` warnings over classes of the packages there are, not declared inputs; the report's `verification` says whether the rules were checked. Isolations are left out.                                                                                                                                                                                                                                                                                                                                                                         |
| 1.2  | As `empty-selector` findings, beside the other model patterns, rather than `stale-pattern`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 1.3  | `fitness.cases`: declared imports with the decision expected, checked by explain's evaluator on the program with the import added; `explain` answers for an import the code does not have the same way. The self-check declares four.                                                                                                                                                                                                                                                                                                                                                           |
| 1.4  | The hash reads a canonical model: every list sorted except `layers`, `subsystemOrder`, `roles` and `thresholdOverrides`. Every baseline was orphaned once. `anarch-lint fmt [--check]` writes the model with its sets sorted, keeping every list of rules in order so index names such as `isolations[2]` hold; it hashes the same. A protobuf rule's `importPaths` is ordered.                                                                                                                                                                                                                 |
| 1.5  | Every cap is among the assumptions, and a result that hits one says so: a slices rule is `truncated` at 100 cycles, capped evidence ends with how many it left out, a subsystem cycle's cut says when it is heuristic, and balance's `capped` names each list it cut. Balance's `untested-dependency` candidates now come from every package, not the 20 listed.                                                                                                                                                                                                                                |
| 1.6  | `thresholdOverrides`, first match wins, for the structural rules; the MMI keeps the module's limits.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| 1.7  | Decided against: no rule asks for a predicate language yet, the idea's own condition.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| 2.1  | Every finding names its `modelRule`, and `anarch-lint explain IMPORTER IMPORTED` lists every rule that examines an import, with why it allows or forbids it.                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| 2.2  | A rule catalog, one name and summary per rule, and model problem codes (`model: CODE: text`).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 2.3  | `excluded` lists generated and build-constrained files, `sizes` counts them, and the gate ratchets their share of the source files, `files.excludedShare`.                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 2.4  | A cycle finding is in scope when a change touches any of its edges (ADR 0011), and each cut in its fix names the shortest cycle it closes, ties broken by sorted order (`Graph.ClosingCycle`).                                                                                                                                                                                                                                                                                                                                                                                                  |
| 2.5  | Each cut between subsystems in a cycle finding says what it carries: its imports and their files, and the type uses behind it by kind.                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 3.1  | `arch.Ref.Kinds` classifies body references by syntax (call, composite literal, conversion, type argument, field access, …) besides `embed`, `field`, `definition` and `signature`.                                                                                                                                                                                                                                                                                                                                                                                                             |
| 3.2  | Findings carry a typed `provenance`: `types`, `source`, `injected` or `dispatch`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| 3.3  | Decided against: the VTA call graph (`arch.Dispatch`) gives the run-time wiring this over-approximation stood in for.                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| 4.1  | In `balance`, with `--since-rev`: `coChange`, `hotspots`, and `hidden-coupling` candidates.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 4.2  | The `coChange` fitness rule, checked by `check --new-from-rev`, over doublestar globs, and the boundary guard as `fitness.boundaries`: a changed line of the files may not import or use the packages.                                                                                                                                                                                                                                                                                                                                                                                          |
| 4.3  | Decided against for now: `check --baseline-rev` compares two revisions, and a history of every commit needs a full analysis per commit, which a CI job that keeps its reports gives without new code.                                                                                                                                                                                                                                                                                                                                                                                           |
| 5.1  | In part: gRPC registrations and clients, and constant HTTP routes, from the Go facts, as balance evidence; `fitness.openapi` compares an OpenAPI document with the routes, through kin-openapi; `fitness.graphql` compares a GraphQL schema with gqlgen's resolvers and validates operation documents, through gqlparser; `fitness.protobuf` compares `.proto` services with gRPC registrations and compiles their `buf.validate` constraints, through protocompile and protovalidate. In `balance`, a request to a route a document declares is contract evidence toward its serving packages. |
| 5.2  | Waiting on a SQL parser to choose, as for the contract files, and a codebase under analysis with queries or datastore kinds.                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| 6.1  | `check --new-from-rev` fails a change that loosens its bars (model, baseline, waivers) and changes non-test Go code.                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| 6.2  | `Finding.Reach`, the packages depending on the finding's package; the text report ranks new findings by it.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 6.3  | Decided against: findings already rank by reach, and a random walk would add an order nobody reads differently.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| 6.4  | In `balance`, with `--coverprofile`: `coverage` by package and `untested-dependency` candidates; with `--since-rev` too, `untested-crossing` candidates for changed functions that call into another subsystem and that no test runs.                                                                                                                                                                                                                                                                                                                                                           |
| 7.3  | `judging.Parse` refuses a reply whose rationale quotes, in backticks, a name that neither the question nor a cited exhibit contains; the prompt asks for backticks, so a name written without them goes unchecked. Prompt version 2.                                                                                                                                                                                                                                                                                                                                                            |
| 8.1  | The corpus tests fail unless each clone is at its pinned commit, and compare whole reports with committed goldens.                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 8.2  | A catalog-wide test that every rule has a failing case; `testdata/gx` pins call resolution through aliased, dot and shadowed names.                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| 8.3  | rapid properties: `Analyze` is byte-identical across runs, and the graph measures ignore reference order.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 9.1  | With `golangci/dupl`, not MinHash: `algorithm.partial-clone` candidates in `connascence`, for runs of code repeated in two or more packages. Bodies identical as `go/format` prints them are facts, not candidates (ADR 0013).                                                                                                                                                                                                                                                                                                                                                                  |
| 9.2  | `fitness.unusedExports`, counting what used API exposes as used.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| 9.3  | In `analyze`: `communities`, gonum's Louvain split into connected parts, and `misplaced` packages; `anarch-lint draft` prints a starter model from them, with a subsystem order when they form no cycle.                                                                                                                                                                                                                                                                                                                                                                                        |
| 9.4  | `//arch:entity NAME` on a type maps it to a domain-model entity, beating a match by name; one naming no entity of a covering model is an `unknown-entity-directive` warning.                                                                                                                                                                                                                                                                                                                                                                                                                    |
| 9.5  | `arch.ErrorGuard`: an `if` at the top of a body, with no init statement, that returns a non-nil error or panics. Guards of the functions that handle an invariant's governed types are evidence on `invariant-unenforced`, never an enforcer (ADR 0016).                                                                                                                                                                                                                                                                                                                                        |
| 9.6  | With `uudashr/gocognit`: `Func.Cognitive` and `cognitive-complexity` warnings above `highCognitive` (15).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 9.7  | In part: test files' imports are read from source into `testImports`, and a forbidden import or isolation checks them with `tests: true`; the self-check opts its isolations in. Their calls of package-level functions are read into `testCalls`, which a `calls` rule checks with `tests: true`; the self-check forbids `time.Sleep` and the environment in tests. The measures still read the build only.                                                                                                                                                                                    |
| 9.8  | The `fanOut` fitness rule.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 9.9  | `render` labels a subsystem dependency that lies on a cycle or runs up the subsystem order, and draws it in red.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |

## Dependency Decisions

Undifferentiated work goes to popular, well-tested libraries rather than
bespoke code. The core, `internal/arch` and the measures, may import only
the pure libraries the self-check's `core-approved-libraries` rule approves
([ADR 0012](docs/adr/0012-the-core-approves-its-libraries.md)); adapters
need no approval.

| Library                                                          | Licence                | Where                 | Use                                                                                                                                                                            |
| ---------------------------------------------------------------- | ---------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `gonum.org/v1/gonum`                                             | BSD-3                  | core (approved)       | Tarjan's components, dominators, Louvain communities                                                                                                                           |
| `github.com/bmatcuk/doublestar/v4`                               | MIT                    | core (approved)       | `coChange` file globs                                                                                                                                                          |
| `github.com/hmarr/codeowners`                                    | MIT                    | `internal/codeowners` | CODEOWNERS parsing and matching                                                                                                                                                |
| `github.com/bluekeyes/go-gitdiff`                                | MIT                    | `internal/git`        | unified diffs, quoted paths included                                                                                                                                           |
| `github.com/uudashr/gocognit`                                    | MIT                    | `internal/gopackages` | cognitive complexity                                                                                                                                                           |
| `github.com/golangci/dupl`                                       | MIT                    | `internal/gopackages` | partial clones                                                                                                                                                                 |
| `github.com/owenrumney/go-sarif/v2`, `github.com/google/uuid`    | Unlicense, BSD-3       | `internal/sarif`      | SARIF 2.1.0 for `check`; v2, since v3 links `net/http` through its schema validator                                                                                            |
| `golang.org/x/tools/cover`                                       | BSD-3                  | `internal/coverage`   | coverage profiles                                                                                                                                                              |
| `github.com/agnivade/levenshtein`                                | MIT                    | core (approved)       | corrections of misspelled patterns                                                                                                                                             |
| `golang.org/x/mod/modfile`                                       | BSD-3                  | `internal/gopackages` | the module path of a nested module                                                                                                                                             |
| `github.com/google/renameio/v2/maybe`                            | Apache-2.0             | `cmd`                 | atomic writes of the baseline and `anarch.md`; in place on Windows                                                                                                             |
| `golang.org/x/tools/go/callgraph/vta`                            | BSD-3                  | `internal/gopackages` | run-time wiring: `Dispatch`es for `injected` forbidden imports                                                                                                                 |
| `github.com/StevenACoffman/indomain`                             | Apache-2.0             | `internal/modelith`   | modelith domain models, the reader codetospec shares                                                                                                                           |
| `sigs.k8s.io/yaml`                                               | MIT, BSD-3             | `internal/pipeline`   | YAML models, read as the JSON they stand for                                                                                                                                   |
| `github.com/StevenACoffman/skillet`                              | MIT                    | `cmd/judge`           | Wilson intervals and calibration of the judges; too little used for the core                                                                                                   |
| `github.com/getkin/kin-openapi`                                  | MIT                    | `internal/openapi`    | OpenAPI 3 documents, and Swagger 2.0 through `openapi2conv`; it links `net/http`, but every file is read through the shell's confined reader                                   |
| `github.com/vektah/gqlparser/v2`                                 | MIT                    | `internal/graphql`    | GraphQL schemas, `extend type` included, and operation documents validated against them; the parser gqlgen and genqlient use                                                   |
| `github.com/bufbuild/protocompile`, `buf.build/go/protovalidate` | Apache-2.0, Apache-2.0 | `internal/protobuf`   | `.proto` files compiled with their imports, and their `buf.validate` constraints compiled; protovalidate brings cel-go and antlr, and the binary grows from 22.2 MB to 33.5 MB |

Kept bespoke, with the reason in the code:

- Johnson's elementary cycles: gonum's `topo.DirectedCyclesIn` has no limit,
  and the count can be exponential.
- Component Rank: gonum's `network.PageRank` stops at a tolerance and sums in
  map order, so its last digits can vary.
- The SDI minimum cut and its Eades–Lin–Smyth ordering: no library offers it.
- The PlantUML component-diagram parser: no Go library reads PlantUML.
- The Plain and Modified cyclomatic variants: gocyclo computes only the
  Extended one, which remains its test oracle.

## From Techne

techne (`~/Documents/git/techne`, MIT) gives agents an IDE's navigation in
ten languages. Every answer states how its names were bound (syntactic,
indexed or resolved) and how much of its scope it read, and an empty answer
proves absence only when both are total (its ADR-0003). Its Go engine
type-checks in process with go/packages, as anarch-lint does. Comparing the
two loaders found three bugs in anarch-lint and a gap, now fixed:

| Found                                                                                                                                                                           | Now                                                                                                                           |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| A file that imports C was measured as nothing: the file cgo writes in its place says "Code generated", under a build-cache path that changes between runs, and imports `unsafe` | Measured as its source, at its own path and lines, importing `C` (techne's `placed` and `sourceOf`)                           |
| A module below the patterns was dropped without a word: `./...` never crosses a `go.mod`, even under `go.work`                                                                  | Listed as excluded, `other-module`, by its `go.mod` (techne's walk and load plans)                                            |
| A forbidden import in a file of another platform passed the gate                                                                                                                | The import rules read excluded files' imports from source (techne's `importsIn`); the measures still read only the build      |
| The report did not say which platform it read, nor the baseline which build it recorded                                                                                         | Both record the platform and the tags; `check` notes a baseline of another build, and `baseline --shrink` leaves one as it is |

Verifying them found more, also fixed: a failed baseline write truncated the
old file; an untracked file whose name has a space became two paths; a
method of a generic type was named with its type parameter, which a `calls`
rule had to spell. Adopted besides: `judge emit` caps each prompt's exhibits by count and
bytes, and counts what it left out (techne's output budgets);
`check` says what it did not read, by
reason, so a pass reads as "nothing new in what was read" (techne's
completeness axis; anarch-lint needs no fidelity axis, since it refuses code
that does not load), and a pattern that matches nothing suggests the
correction of a misspelled segment (techne's `address.go`).

Deferred:

- Loading each module below the root as a program of its own, as techne's
  load plans do; the model, the baseline and the measures assume one program.
- Test variants (`p [p.test]` and `p_test`), should tests ever count.

## 1. Verify the Model Itself

What remains open is the rule set as a whole: two rules that contradict each
other, or one that a stricter rule makes redundant.

### 1.1 Find Conflicts and Redundant Rules by Enumerating Cells

- **Source:** `internal/verify/verify.go`, `Verify` and `verifyTable`. It
  collects cut points per input column and enumerates the product of witness
  points (`buildDim`, `eachPoint`, capped by `gridBudget = 500_000`). It counts
  the rules matching each point with `ir.MatchCell`, the function the VM calls
  too (`internal/vm/vm.go`), so the proof and the execution can't disagree.
  Its kinds are `gap`, `conflict`, `dead-rule`, `unreachable-default`,
  `subsumed`, `priority-gap` and `not-verifiable`. Only `gap` and `conflict`
  carry a concrete `Witness`; `dead-rule`, `subsumed` and `priority-gap` name
  the rules. Gaps and subsumptions are capped at 5 (`maxWitnessesPerKind`);
  conflicts are de-duplicated and sorted (`sortedConflicts`) but not capped.
  Subsumption (`reportSubsumption`, a `uint64` bitset) runs only for 2 to 64
  rules and *silently skips* tables above 64.
- **For anarch-lint:** the cells are pairs of (subsystem, layer) classes of
  the importing and imported package, a finite set because subsystems and
  layers are declared. Evaluate every import rule on each cell, with the same
  matcher the analysis uses, and report:
  - a *conflict*: one rule allows a cell another forbids, such as a
    `layerAccess.mayOnlyAccess` that `strictLayers` contradicts;
  - a *subsumed* rule: every cell it forbids is forbidden by another rule
    already;
  - a *dead* rule: it forbids no cell any declared pattern can reach.

  Waivers aren't cells: they are per fingerprint, and unused waivers are
  already reported.
- **Determinism and honesty:** sort the findings and cap the witnesses. Where
  anarch-lint can't enumerate, it says `not-verifiable` instead of skipping, as
  feelc fails to above 64 rules: an isolation's `$1` back-references, and
  `forbiddenImports` naming packages outside the module, which the cells don't
  cover.
- **Gating:** these findings are about the model and have no location, so
  under `--new-from-rev` they would always be out of scope (see 2.4). They gate
  like the coverage and compliance floors, whatever the change.
- **Effort:** medium.

### 1.2 Check the Remaining Patterns for Matching Nothing

`isolations[].mayImport`, subsystem `interface` patterns and
`balance.messaging` are never checked for matching nothing, so a typo there
passes silently: an `interface` pattern that matches nothing makes every
import of the subsystem an `interface-bypass`, and a stale `messaging`
pattern hides an asynchrony slot. Report them as `stale-pattern`, as the other
model patterns are. **Effort:** low.

### 1.3 Test the Rules with Declared Cases

- **Source:** `internal/check/check.go`. A `Claim{Decision, Input, Expect,
  Desc}` gets `supported`, `contradicted` or `error`; `Blockers()` counts every
  verdict that isn't `supported`.
- **For anarch-lint:** a model test file of cases such as "an import from
  `billing/internal` to `shipping` fails by `isolate-areas`", run against the
  real evaluator on synthetic edges, so a model change that silently stops
  catching a case fails. It is the same idea as `TODO.md`'s
  `anarchtest.Check(t, model)`, which would run the model inside `go test`.
- **Effort:** low to medium.

### 1.4 Hash the Model Without the Order of Its Sets

- **Sources:** go-rule-engine `serialization.go` `marshalCanonical` compiles,
  then encodes, sorting tags, derived facts and rules while keeping the order
  of logical children; `CanonicalHash` hashes that. `cache.go` `CompileCached`
  accepts a hit only when the hash and the limits match. feelc
  `internal/fmtrules` is a canonical, idempotent `feelc fmt` whose tests list
  what it loses (comments).
- **For anarch-lint:** the hash already ignores formatting. What still
  changes it without changing the meaning: reordering a set (a subsystem's
  `packages`, a forbidden import's `imports`, `roles[].embeds`), the editor-only
  `$schema` field, and an explicit value equal to its default. Sort the set
  fields and drop `$schema` before hashing, and keep the order of `layers`,
  `subsystemOrder` and `roles`, which mean something. A `fmt [--check]` command
  can write the same form.
- **Cost:** changing the hash orphans every baseline once, so do it with a
  reason in the release notes, or with a baseline schema `v2` that `baseline
  --shrink` rewrites.

### 1.5 Limits and Three-Valued Outcomes

- **Source:** `limits.go` caps AST depth, rule count, explanation length,
  `MaxIterations`, `MaxDiagnostics` and `EvaluationTimeout`. Hitting
  `MaxIterations` makes a result `Indeterminate` (`plan.go`), not true or
  false. `compiler.go` `Compile` sorts rules by priority, then ID, and copies
  only the `Tags` and `Derive` slices; predicates stay the caller's.
- **For anarch-lint:** fitness caps cycles at 100 (`maxCycles`) and evidence
  at 20 per edge, and SDI cuts switch method above 12 elements. Record every cap
  among the assumptions, and when one is hit say so in the result (`Truncated`
  exists for cycles), never silently.
- **Warning:** `findDerivationCycle` (`compiler.go`) ranges over a map and
  returns one rule ID, not the cycle, so which rule it blames varies between
  runs. Don't copy it.

### 1.6 Limits That Vary by Package

- **Source:** zen-go's JDM decision tables (`test-data/table.json`,
  `hitPolicy: "first"`). feelc checks gaps only for single-hit policies and
  conflicts only for `UNIQUE` and `ANY`.
- **For anarch-lint:** `thresholds` holds one value per module, and
  `connascence.maxStrength` and `maxDegree` one per locality; none varies by
  package. An ordered table of (package pattern) → limits, first match wins,
  lets a legacy subsystem have looser limits than a new one without waivers.
  It is `TODO.md`'s "model knobs for the fixed constants". 1.1 can then report
  rows that a row above shadows.
- **Effort:** low to medium. Use the idea only; see "Not Worth Adopting" on
  zen-go.

### 1.7 Write Fitness Rules in a Closed Predicate Language

- **Source:** `model.go` has `True`, `False`, `All`, `Any`, `Not`, `Exists`
  and `Compare`. `Predicate` is an open interface, and `PredicateFunc` and
  custom operators admit callbacks; only the versioned JSON grammar
  (`docs/json-ast.md`, enforced in `serialization.go` with
  `DisallowUnknownFields` and a trailing-data check) is closed.
- **For anarch-lint:** fitness rules are a fixed set of kinds. A closed JSON
  language over edge facts (`from.layer`, `to.subsystem`, `edge.kind`, `role`)
  would let users write rules without new code, with today's kinds as sugar.
  It must stay closed, with no callbacks, for 1.1 to verify it. `TODO.md`'s
  "severity and reason on the non-fitness rules" is a step toward one rule
  shape.
- **Effort:** high. Wait for a rule users ask for that no kind expresses.

## 2. Explain Findings, and What Was Not Seen

### 2.1 Explain Every Finding and Every Pass

- **Source:** `internal/vm/trace.go` `DecisionTrace{Matched, Fallback,
  RuleIndex, RuleLine, Cells, Contributors, Output, HitPolicy, NotGeometric}`.
  `TraceFull` walks the upstream decisions that were reached, in dependency
  order, skipping those required but not reached. Stability comes from
  replaying on one memoized evaluator; `internal/explain` `NormalizeJSON` only
  writes decimals in fixed notation.
- **For anarch-lint:** an `explain` field, or command, giving the declared
  rule that fired with its JSON pointer in `anarch.json`, the values that
  matched, and the witness path. It also explains a *pass*: "allowed by
  `layerAccess` rule X", "excused by waiver W until 2027-01-01".
- **Warning:** go-rule-engine's `Explanation` "records attempts up to
  MaxExplanation; truncation is silent" (`plan.go`). Mark truncation.
- **Effort:** low to medium. SARIF output (`TODO.md`) can carry the same
  paths as `codeFlows`.

### 2.2 Stable Diagnostic Codes with a Suggestion

- **Source:** `internal/diag` gives every error a stable code (`DSL001`,
  `CMP001`, …) with file, line, column and a `Suggestion`.
- **For anarch-lint:** findings already have stable rule names and a `Fix`.
  Model *errors* from `DecodeModel` are plain messages; a code per problem
  ("model: isolation refers to $2 but captures 1") lets editors and CI link to
  documentation. **Effort:** low.

### 2.3 List What Was Left Out

- **Source:** `internal/indexer/discoverexclusion.go`. One walk yields the
  included and the excluded files (`newExcludedFile`, `dirExclusionReason`),
  each excluded file with one of five reasons (`DIR_VENDOR`, `DIR_DOTPREFIX`,
  `UNSUPPORTED_EXTENSION`, `BUILD_TAG`, `SIZE_LIMIT`;
  `internal/schema/graph.proto`). Only a build-tag exclusion records its
  context, as `GOOS/GOARCH`. `internal/indexer/sync.go` (`diffExclusions`)
  treats a change to exclusions alone as a change. `internal/query/coverage.go`
  checks `Discovered == Indexed + ExtractionFailed + Excluded`.
- **For anarch-lint:** the report counts generated files
  (`Package.Generated`) but doesn't list them, and doesn't mention test files
  or files a build tag excludes. List every excluded file with its reason,
  check the same accounting identity, and let the gate fail when the excluded
  share grows, so code can't leave the measures by gaining a `// Code
  generated` header. Listing doesn't measure generated code; a cycle through it
  still goes unseen.
- **Effort:** low.

### 2.4 Cycles a Change Closes, with the Path

- **Sources:** gortex `internal/analysis/cycles.go` `WouldCreateCycle`, a DFS
  from the new edge's target over imports *and* calls, returning the first path
  found in edge order, not the shortest. feelc `internal/project/link.go`
  `checkAcyclic` reports the whole path of a cross-module cycle.
  codegraph-go `internal/query/filegraph_cycles.go` is an iterative Tarjan over
  sorted keys and successors.
- **The gap in anarch-lint:** Go forbids import cycles between packages, so a
  change can only close a cycle between subsystems or types.
  `subsystem-cycle` gives a minimum cut and the imports behind it, not a path,
  and its location is the first cut import. Under `--new-from-rev` a finding
  gates only when its location is on a changed line (`arch/changed.go`), so a
  cycle closed by an import elsewhere is counted `OutOfScope`; only the
  `subsystems.inCycles` metric catches it.
- **For anarch-lint:** a cycle finding is in scope when *any* of its edges'
  positions is on a changed line, and its evidence gives the shortest closing
  path, ties broken lexically, so the fix reads "remove A→B, or invert it".
  Fitness already lists elementary cycles; this is about gating.
- **Effort:** low.

### 2.5 Aggregate a Coupling by File and Kind

- **Source:** `api/mcp/tools/structural.py` `get_file_neighbors` counts edges
  per neighbouring file and per `relation:direction`, aggregates before
  limiting, and sorts by `(-count, path)`.
- **For anarch-lint:** an edge to cut is more actionable as "7 calls and 2
  type references, from these 3 files" than as one weighted edge. Needs 3.1.

## 3. Dependency Kinds and Provenance

### 3.1 Give Every Reference a Kind

- **Taxonomy:** gortex `internal/graph/edge.go` names, among others,
  `reads`, `writes`, `accesses_field`, `typed_as`, `param_of`, `returns`,
  `instantiates`, `composes`, `spawns`, `sends`, `recvs`, `reads_env`,
  `reads_config`, `queries`, `emits`, `listens_on` and `handles_route`. gortex
  infers them from tree-sitter; anarch-lint can compute them exactly from
  go/types.
- **For anarch-lint:** `arch.Ref` counts referring identifiers with no kind,
  and the kinds that exist live in `Facts`, which load only for the
  connascence and balance measures. Give each `Ref` its kinds: embedding,
  field type, parameter or result type, method call, composite literal, type
  argument, conversion, field read, field write. Then:
  - field-access rules: "no subsystem writes another's struct fields"
    (`Facts.FieldWrites` sees only a method's own receiver);
  - "layer X may only reference layer Y's interfaces", generalising
    `fitness.interfacesOnly`;
  - SDI weights by how hard a kind is to break: an embedding is harder than a
    parameter type;
  - evidence for connascence and Balanced Coupling strength, which today infer
    intrusiveness from shapes.
- **Effort:** medium. It changes the program's JSON and the golden files.

### 3.2 Provenance on Every Edge

- **Sources:** codegrapher `model.Edge.Provenance` declares `tree-sitter`,
  `scip` and `heuristic`, and the code also writes `semantic`,
  `explicit_annotation`, `modfile`, `package.json` and `specscore`.
  codegraph-go `graph.proto` field 6 is `ast` or `heuristic`, a heuristic edge
  naming its source in `Metadata["synthesizedBy"]` (`declared-implements`,
  `structural-override`, a route detector). gortex has six origin tiers,
  `lsp_resolved`, `lsp_dispatch`, `ast_resolved`, `ast_inferred`,
  `text_matched` and `speculative` (`edge.go`), with severities `error`, `warn`
  and `info` per guard (`guards.go`, `rule_family.go`).
- **For anarch-lint:** tag each dependency as `types` (exact), `implements`
  (interface satisfaction), `injected` (`arch.Injection`), `annotation` or
  `contract` (§5). A rule can then count only exact edges, or gate inferred ones
  at `warn`, and a finding says how its edge was known, as connascence already
  separates facts from candidates. Statically known reflection
  (`Facts.ReflectByNames`) fits here too.
- **Effort:** low once 3.1 exists.

### 3.3 Implementation Edges in Impact Analysis

- **Sources:** codegraph-go `internal/query/traverse.go`, `Engine.Impact` and
  `Engine.Affected`, whose reverse BFS also visits `dispatchSiblingIDs`, the
  other implementations of an interface method. (code-graph's
  `derive_overrides` is about class inheritance, not interfaces.)
- **For anarch-lint:** `Type.Implements` is computed exactly but used only
  for roles. Using it in impact (6.2) stands in for the deferred VTA call graph
  and over-approximates, so tag those edges `implements` (3.2).

## 4. Temporal Coupling and Hotspots

Churn is read today only as evidence of volatility.

### 4.1 Co-Change, from the Churn Already Read

- **Source:** gortex `internal/cochange/cochange.go` `Mine` parses
  `git log --no-merges -n N --name-only` (N defaults to 1500), drops commits
  touching more than 40 files from the pairs (they still count toward each
  file's changes), keeps pairs that changed together at least twice, and
  scores them `count / sqrt(changesA × changesB)`, at most 1, sorted by score
  and then the two paths.
- **For anarch-lint:** no new git code is needed. `arch.Churn` already maps
  each file to its commits over a resolved range, so co-change is a pure
  function in the core: invert the map, count pairs per commit, and roll files
  up to packages and subsystems. Then:
  - *Hidden coupling:* subsystem pairs that co-change above a threshold but
    have no static dependency between them. That is coupling the import graph
    can't see: a shared schema, a wire format, a duplicated algorithm. The fix:
    make it explicit, or merge.
  - *Balanced Coupling:* co-change as evidence for volatility next to churn,
    and for strength.
  - *Hotspots:* commits × complexity per package and function.
- **Determinism:** keep the existing range from `--since-rev` to a resolved
  HEAD and `--no-renames`, and record the commit cap among the assumptions.
  Don't add `-M`: it would make results depend on git's similarity guesses,
  which `git.Churn` avoids on purpose. Merges are in the range; leave them out
  of pairs, as gortex does.
- **Effort:** low.

### 4.2 Change Rules: Paired Files and Boundaries

- **Source:** `internal/analysis/guards.go`. `evaluateCoChange`: when the
  changed symbols include one under the `Source` path prefix, one under
  `Target` must change too, except for paths matching an `Except` glob; the
  violation is only a message. Its sibling, the boundary guard
  (`guards.go`), forbids a changed symbol under `Source` to call or reference
  anything under `Target`.
- **For anarch-lint:** a `coChange` fitness rule under `check
  --new-from-rev`, on changed files: "a change to `api/*.proto` must also
  change `gen/`", "a change to the domain types must update
  `anarch-lint.modelith.yaml`". The finding names the patterns nothing matched.
  It needs the changed files, which `git.ChangedLines` gives.
- **Effort:** low.

### 4.3 Keep a Metric History by Commit

- **Source:** `api/git_utils/git_utils.py` `build_commit_graph` walks the
  first parent and re-analyses the files each commit changed, storing replay
  queries; `switch_commit` picks a direction by comparing commit dates.
- **For anarch-lint:** a history of MMI, NCCD and cycle counts per commit,
  for trend lines, as snapshots keyed by tree hash. Avoid both flaws: the
  first-parent walk skips side branches, and dates mislead under clock skew. It
  depends on `TODO.md`'s comparison against a base revision checked out in a
  worktree.

## 5. Coupling Outside the Go Code

### 5.1 Contracts for HTTP, gRPC, GraphQL, OpenAPI and Events

- **Source:** `internal/contracts/`, about 20,600 lines without tests. The
  README lists HTTP routes, gRPC, GraphQL, OpenAPI, message topics, WebSocket
  events, environment variables and Temporal workflows. Each becomes a
  canonical ID such as `http::GET::/api/users/{id}`, matched within and across
  repositories by (workspace, project, ID, role):
  - `contract.go`: `Contract{ID, Type, Role, WorkspaceID, ProjectID}` and
    `NormalizeHTTPPath`;
  - `matcher.go` `Match`: matched provider–consumer pairs and orphans, and
    `joinRPCCanonical` for gRPC and Thrift;
  - `validate.go`, `Validate`, `diffPair`, `canAssign`: a direction-aware
    shape diff, where a field the reader needs going missing is breaking and an
    extra field is not, with wire-type equivalences (time, UUID, bytes);
  - `internal/analysis/event_boundary.go` `EventBoundaryFamily.Evaluate`:
    producer and consumer globs, `Forbid`, `RequireConsumer`.
- **Detection is shallow.** The protobuf, GraphQL, OpenAPI, environment and
  gRPC-client extractors are regular expressions over raw text; Go routes and
  endpoint topics use tree-sitter (`route_ast_go.go`, `topics_struct_go.go`,
  `bodyfacts_go.go`). None uses go/types:
  - *Protobuf* (`grpc.go`, `shape_proto.go`): `service\s+(\w+)\s*\{`,
    `rpc\s+(\w+)\s*\(` and a regex per message field. It doesn't parse the
    files or follow their imports.
  - *gRPC clients in Go* (`goGRPCNewClientAssignRe`, `goGRPCCallRe`): it
    remembers a variable assigned from `New<X>Client(` and counts any
    `var.Method(` as a call, so it misses a client passed in as a parameter or
    held in a field, and a variable of the same name in another scope matches
    falsely.
  - *GraphQL* (`graphql.go`): the schema regex is
    `type\s+(Query|Mutation|Subscription)\s*\{([^}]*)\}`. Consumers come from
    ``gql`…` `` literals and bare operation blocks, where `[^}]*` stops at the
    first nested `}`. Nothing is specific to gqlgen or genqlient.
  - *OpenAPI* (`openapi.go`): regexes tied to indentation, such as
    `^  (\/\S+)\s*:` for paths at exactly two spaces, instead of a YAML or JSON
    parser; `$ref` across files is lost.
- **For anarch-lint:** take the model, the matcher and the diff semantics,
  and detect precisely with pure-Go, deterministic parsers:
  - *Go code*, with go/types: calls to the generated `Register<X>Server`
    and `New<X>Client`, resolved by type so the client is found wherever its
    value travels; `http.HandleFunc` and router methods with constant-folded
    paths; publish and subscribe calls on the clients in `balance.messaging`.
  - *Protobuf*, with `github.com/bufbuild/protocompile` (Apache-2.0): full
    descriptors with imports resolved, linked to Go through `go_package` and
    the generated types.
  - *GraphQL*, with `github.com/vektah/gqlparser/v2` (MIT), the parser
    gqlgen and genqlient are built on: schemas, `extend type` across files, and
    operation documents validated against the schema. A gqlgen resolver is the
    provider and a genqlient operation the consumer, down to its fields.
  - *OpenAPI*, with `github.com/getkin/kin-openapi` (MIT): specs loaded with
    `$ref` resolved, linked to the routes found in Go.

  That gives contract edges (provenance `contract`) for Balanced Coupling
  distance, better evidence than the messaging-import slot; a contracts fitness
  family (orphan providers, events with no consumer, forbidden producers);
  breaking-shape diffs between the baseline and `--new-from-rev`, such as a
  removed proto field a known consumer selects; and cross-repo coupling, by
  joining the contract lists of several runs keyed by a workspace name.
- **Related:** gortex `internal/analysis/contracts.go` `VerifyChanges` and
  `checkInterfaceViolations` check a signature change against its callers and
  implementers, a Go-API version of the same breaking-change idea.
- **Effort:** high: three new dependencies, each needing the vetting
  codegraph-go gives gonum (`check:gonum`: govulncheck, SBOM, cgo closure).

### 5.2 Route, SQL and Datastore Seeds

- **Sources:** codegrapher `internal/extract/walk_go.go` `ExtractGoRoutes`
  (regex) and `walk_sql.go`, which parses `.sql` files with tree-sitter and so
  can't see SQL in Go strings. codetospec `extractors/go/routes.go`
  `ExtractRoutes` (matches router methods by name, resolves handlers with type
  information), `extractors/go/sql.go`, and `extractors/webapp/entities.go`,
  which finds `datastore.RegisterKind` calls by selector name with go/ast.
- **For anarch-lint:** a table or datastore kind touched by more than one
  subsystem is shared-schema coupling, which Khononov ranks strongest and
  anarch-lint can't yet see. Detect it with go/types, from query constants and
  registration calls, so aliased imports aren't missed.

## 6. Change-Scoped Gating and Prioritisation

### 6.1 Lock Bars Before Measuring Against Them

- **Source:** `corpora/graph-cluster-threshold.json` is committed before any
  measurement, and says what to do when the bar is missed; loosening it isn't
  one of the options. The observation file records the threshold file's
  digest (`thresholdDigest`), which
  `TestClusterThresholdDigestMatchesCommittedObservation` checks, and
  `tools/graphcluster/ancestry_test.go`
  (`TestClusterThresholdCommitIsAncestorOfEveryMeasurement`) uses `git
  merge-base --is-ancestor` to show the threshold commit precedes every commit
  touching the observations or the harness.
- **For anarch-lint:** `check --new-from-rev` could refuse a change that both
  loosens the baseline, a threshold, a gate or a waiver *and* changes the code
  measured against it, unless the loosening is an earlier commit. `baseline
  --shrink` only tightens, so it's always allowed. It needs the base
  revision's model and baseline, read with `git show REV:anarch.json`. The same
  check suits the judge's gold set (§7).
- **Effort:** low to medium. In a shallow clone it reports that it was
  skipped.

### 6.2 Rank Findings by Their Reach

- **Sources:** codegrapher `internal/cli/affected.go` `findAffectedTests`, a
  bounded reverse BFS over files; code-graph `structural.py` `impact_analysis`,
  depth capped at 10; codegraph-go `traverse.go` (3.3); gortex
  `internal/analysis/diffmap.go` `MapGitDiff`, which maps a diff to symbols.
- **For anarch-lint:** give each finding the number of packages that depend
  on its subject within k hops, and rank the text report by it. Component Rank
  reports only the top 10 types, and most findings are about packages, so this
  is counted on the package graph. Under `--new-from-rev`, also name the
  changed declarations a finding's location falls in.
- **Effort:** low.

### 6.3 Personalised PageRank from the Change

- **Sources:** codegrapher `mcp/explore.go` and codegraph-go
  `internal/query/rwr.go`, both `computeGraphRelevance`: a random walk with
  restart, alpha 0.25, 25 fixed iterations; codegraph-go rounds scores to 1e-9
  (`roundRWRScore`).
- **For anarch-lint:** Component Rank seeded at the changed packages, to
  order a change's findings. Sort the nodes and round, for stable output.
  Advisory only.

### 6.4 Coverage Attributed to Functions

- **Sources:** codegrapher `coverage/profile.go` (via
  `golang.org/x/tools/cover`) and `coverage/attribute.go`, whose
  `attributeLines` gives each line to its innermost function and
  `attributeBlocks` counts statements per block; `attributeLines` returns its
  result in map order. code-graph `api/code_coverage/lcov/lcov.py`.
- **For anarch-lint:** with a `--coverprofile` input, hashed into the
  report's inputs and never collected implicitly, flag packages many depend on
  (high Ca) that tests rarely run, and new cross-subsystem functions with no
  coverage. It covers part of the "test code not loaded" gap. Don't copy
  code-graph's formula, which divides by the line span, off by one, rather than
  the instrumented statements.

## 7. The Automated Judge

`mmi-plan.md` §9 steps 7–11 plan a judge with canaries, a gold set and
calibration. codetospec has solved parts of this for its own LLM checks.

### 7.1 Planted Faults, with Known Blind Spots Locked In

- **Source:** `internal/grounding/planted_test.go` `TestPlantedFaults`
  always runs: a control case, three faults that must be caught, and a
  negated condition, a known blind spot, that must stay `grounded`, so the list
  of blind spots can't go stale. `internal/crosscheck/eval_test.go`
  `TestEvalPlantedFaults` runs only with `CODETOSPEC_EVAL=1` and a configured
  model, with an empty cache in `t.TempDir()`: a control and four faults
  (changed threshold, negated condition, invented message, wrong actor),
  logging the catch rate without asserting it.
- **For anarch-lint:** the canaries. A deterministic suite for the
  mechanical checks below, and a live suite to re-run before adopting a new
  prompt or model. The control case's false-alarm rate starts the calibration.

### 7.2 Put the Judge and Its Sampling in the Key

- **Source:** `internal/llm/promptkey.go` `PromptKey(model, parts...)` hashes
  the model and the prompts, NUL-separated. The cross-check key covers both
  prompts, both repair prompts and the repair flag; `crosscheck.inputHash` adds
  the rule ID, title, body and the excerpt shown, joined by newlines. The
  temperature (fixed at 0.1) and `--max-tokens` are *not* in the key.
- **For anarch-lint:** ADR 0007 puts the question in the evidence hash. Put
  the judge's model, prompt, sampling settings and blinding options in the
  verdict's key too, so changing any of them makes earlier verdicts `stale`.

### 7.3 Check a Verdict Mechanically Before Trusting It

- **Source:** `internal/grounding/grounding.go` `Check` requires every number
  and quoted string in a rule's requirement text to appear in the cited lines,
  reading numbers with the output language's decimal separator.
  `Constants()` indexes numeric constants that extractors supply (webapp's
  reads `http.Status*` from GOROOT), and `elsewhere()` says where a missing
  value is. A claim with nothing checkable is `unchecked`, a third status
  counted apart; nothing gates on it.
- **For anarch-lint:** a verdict whose rationale names an identifier, path
  or value absent from its exhibits is `insufficient-evidence` (ADR 0009).

### 7.4 Numbered Evidence, Cited by ID

- **Sources:** `mapper.numberedContent` numbers every line and marks context
  lines as not citable. `crosscheck.citedExcerpt` marks cited lines `>` and up
  to 10 context lines on each side `·`, within one 400-line budget.
  Citations must fall inside the given chunk (`validateMapReply`), among the
  candidates (`validateReduceReply`, which also repairs slugs itself), or, for
  a repair, in a file the rule already cited and on a real symbol
  (`validateRepairCitations`). `ChatJSON` retries with corrections a number of
  times each caller sets: 2 for map and cross-check, 1 for repair, 0 for
  reduce except at its smallest batch. `docs/03-failure-modes.md` §3.1 explains
  the numbering: without it, citations drift the further the model reads.
- **For anarch-lint:** `JudgeView` already numbers its `Exhibit`s. Accept
  only verdicts that cite exhibit IDs; a citation outside the set makes the
  verdict `insufficient-evidence`.

### 7.5 Measure a Judged Field Before Relying on It

- **Source:** `docs/03-failure-modes.md` §3.5: self-reported confidence was
  about 0.9 for every rule, and an explicit/implicit label marked about 99%
  explicit; neither told cases apart. It names better signals: an adversarial
  verdict, citations overlapping the symbol, and agreement across candidates.
- **For anarch-lint:** `TODO.md` plans `confidence` on verification events.
  Record it if the judge gives it, but don't gate or rank on it until the gold
  set shows it separates right from wrong. That applies first to the subdomain
  classification behind volatility.

### 7.6 Keep Repair Out of Judgement

codetospec's repair step runs on a `partial` or `unsupported` verdict, sees
the reviewer's reason for rejecting the citation, and its `repaired` result
*replaces* the verdict with no second review, only a mechanical overlap check.
A gating judge must not see earlier verdicts (ADR 0008), and a changed answer
needs a new review. Don't copy it.

### 7.7 Settle What Code Can Settle Before Asking

- **Source:** `crosscheck.go` keeps `unchecked` (never judged) apart from
  `mechanical` (decided by code, never sent to the LLM). Structured output is an
  inline JSON schema in the prompt, validated by hand, with the error fed back
  (`client.go`). Token use is recorded per phase (`internal/state`).
- **For anarch-lint:** a slot whose answer the measures decide (ADR 0005's
  domain inferences, connascence facts) should be answered `mechanical` and
  never reach the judge; the balance report's `Skipped` already lists the slots it did not open.

## 8. Testing `anarch-lint`

### 8.1 Pin the Corpus and Check It for Drift

- **Sources:** codegraph-go `internal/corpora/manifest.go` validates each
  repository's SHA and allows only MIT or Apache-2.0; `coverage.go`
  `CheckCoverage` derives its counts from the manifest and observations rather
  than a stored summary; `task corpora:drift` re-measures and requires an empty
  `git diff`. gortex `internal/eval/parity/baselines.go` `Baseline.Check`,
  whose repositories (`repos.go`) pin a branch or tag, or nothing.
- **For anarch-lint:** `just corpus` runs `TestCorpus` against clones made
  beforehand (default `~/Documents/git`); nothing clones them, no SHA is
  pinned, and only the models are committed (`testdata/corpus/`). Pin each
  repository's SHA in a manifest, commit its full JSON output, and fail on any
  difference. It catches regressions and nondeterminism at scale, and will hold
  the judge's gold set.
- **Effort:** medium.

### 8.2 Prove Every Rule and Allowlist Can Fail

- **Source:** the `archtest` packages (`internal/{query,graphstore,cli,mcp}/archtest/`):
  - `stdout_detection_selftest_test.go` runs each predicate on a fixture
    that must be fully flagged and one that must be flagged zero times,
    including a local `os` shadowing the package;
  - `stdout_closure_selftest_test.go` checks that the closure walk reaches
    dependencies, and `stdout_transport_allowlist_selftest_test.go` that an
    allowlist covers exactly one function;
  - `import_direction_test.go` asserts *positive witnesses*: the test fails if
    a package it expects in the dependency set leaves it, so a broken walk
    can't pass vacuously.
- **For anarch-lint:** a must-flag and a must-not-flag fixture for each rule
  kind, including shadowing, aliased and dot imports and generic
  instantiations; a self-check that asserts its own witnesses (the self-check
  model's rules each match something already, per ADR 0004).

### 8.3 Determinism Tests for the Rest of the Pipeline

- **Source:** `internal/indexer/determinism_test.go` `TestDeterministicRebuild`
  indexes twice under `GOMAXPROCS(8)`, with the one volatile field zeroed, and
  reports the first differing frame. `internal/query/community.go` has a
  test-only `reverseInsertion` control that feeds a graph backwards.
- **For anarch-lint:** properties for what isn't covered: Component Rank,
  LCOM4, and `arch.Analyze` on a shuffled program (packages, files, refs and
  imports reordered) giving byte-identical JSON.

### 8.4 Truth Tables and Boundary Tests

- **Sources:** `operator_truth_table_test.go`;
  `mutation_boundaries_internal_test.go`; `CONTRIBUTING.md`, whose full and
  release mutation gates require zero surviving viable mutants; `fuzz_test.go`;
  `api/baseline.txt` and `api/v2.txt`, binary apidiff export data.
- **For anarch-lint:** truth tables for each pattern and capture form
  (`/...`, `$*`, `$**`, `$1`), and fuzzers for the waivers and judgements
  files beside `FuzzDecodeModel`.

## 9. Further Analyses

### 9.1 Near-Clone Detection

- **Source:** `internal/clones/`: `minhash.go` (64 slots, an xorshift64*
  generator seeded `0x9E3779B97F4A7C15`, 16 bands of 4 rows), `lsh.go`, and
  `stratified.go` (five length classes). Candidates are filtered by exact
  Jaccard similarity after LSH.
- **For anarch-lint:** the partial-clone gap, filled by `golangci/dupl`
  instead of a MinHash port: its suffix tree finds exact repeats of the
  syntax tree with names and values ignored, so no estimate needs deciding.
  Clones across packages are connascence of algorithm.
- **Built:** see "Status". dupl's threshold applies to each statement of a
  run, not to the run; at 60 every clone on anarch-lint itself spans at least
  four lines.

### 9.2 Unused Exported API

- **Source:** `internal/analysis/deadcode.go`: `incomingUsageKinds` sets, per
  node kind, which edges count as a use; `buildIfaceRequiredMethods` keeps
  methods an interface requires; `isPackagePrivateByConvention` handles
  `internal/`.
- **For anarch-lint:** exported identifiers no other package uses inflate the
  visibility metrics; the fix is to unexport or delete them. It needs test
  packages loaded, or tests become the only users. Don't copy code-graph's
  `unreachable_entities`, which calls anything without an incoming edge dead,
  `main` and `init` included.

### 9.3 Declared Subsystems Against Detected Communities

- **Sources:** codegraph-go `internal/query/community.go` `AssignCommunities`:
  gonum `Modularize` (Louvain) with a fixed PCG seed, sorted paths and
  `canonicalCommunityLabels`. codetospec `internal/communities` has built
  Louvain plus a connected-components split, only to check directory-based
  domains, never to define them; anarch-lint's is adapted from it.
- **Louvain, not Leiden.** Leiden adds connected communities, which the split
  restores, and better partitions, which matter little for advice. No Go
  Leiden is both permissively licensed and widely used: `andreswebs/meso` is
  GPL-3.0. Two of gonum's orders vary between runs: `Communities()` and
  `ConnectedComponents`, so communities are sorted by their first member.
  Weights are whole import counts, so their sums are exact in any order.
- **For anarch-lint:** report packages whose community disagrees with their
  declared subsystem, and offer a starter model for a module with none. It
  could seed `TODO.md`'s subsystems defined by a `$*` capture. Communities
  shift with small edits, so it's advisory and never gates.
- **Warning:** gortex's Leiden (`internal/analysis/leiden.go`) is
  nondeterministic: `for c, wc := range commWeights` with a strict `>` sends
  ties to whichever entry the map yields first, and map order also orders the
  queue and the refinement; `incremental_communities.go` seeds from a cached
  partition. Don't reuse it.

### 9.4 Code Annotations That Link to a Domain Model

- **Source:** `semantic/build.go` `annotations()` reads
  `// modelspec: implements X` and `// meaninggraph: references Y`, attaches
  each to the smallest enclosing symbol or one starting within 3 lines below,
  and reports `unresolved-annotation` for a missing or ambiguous target; a
  directive with no symbol nearby is skipped silently. `trace/trace.go`
  attaches to the next symbol and records a missing target as an `unresolved`
  node.
- **For anarch-lint:** an `//arch:entity NAME` directive for modelith
  conformance, which helps where an `iota` enum's constants don't match its
  value names, reported like `unknown-role-directive` when dangling,
  including when no declaration follows it.

### 9.5 Invariants from Guard Shapes

- **Source:** `extractors/webapp/authz.go` and `render.go`. A guard is an
  `if` without an init statement whose body ends in `return` and whose
  condition is an access check or that returns an authorization error, or
  nested one level inside one; calls marking open access count without an `if`.
  Unknown predicates are quoted verbatim and unrecognised shapes produce
  nothing.
- **For anarch-lint:** a template for the domain-model invariants gap:
  find validation guards on entity types and check each modelith invariant has
  one, with go/types (codetospec's `IMPROVEMENTS.md` item 4 notes that matching
  package names misses aliased imports).

### 9.6 Cognitive Complexity

gortex's version (`internal/parser/languages/helpers_complexity.go`)
deliberately doesn't count sequences of `&&` and `||`, so it isn't
SonarSource's definition. `uudashr/gocognit`, which golangci-lint runs,
implements the specification; anarch-lint uses it.

### 9.7 a Test Scope for Each Rule

- **Source:** `internal/query/archtest/import_direction_test.go` loads once
  with `Tests: true`, and its production-only rule skips test variants by
  package ID.
- **For anarch-lint:** load test packages once, and let each rule say whether
  test files count. Test-only cycles, and tests reaching into another
  subsystem's internals, become visible.

### 9.8 Fan-Out Limits and Caller Restrictions

- **Source:** gortex's architecture rules (`internal/analysis/architecture.go`,
  `config/config.go`): layers with glob `Paths` and `Allow`/`Deny` lists, and
  rules with `MaxFanOut`, `DenyCallersOutside`, `Except` and a severity.
- **For anarch-lint:** layers, access lists and `dependents` already cover
  most of it. `MaxFanOut`, a cap on a package's efferent coupling, is new and
  cheap; `fitness` already computes Ce.

### 9.9 Findings Drawn on the Architecture

- **Source:** `internal/graph` renders the decision graph as DOT, Mermaid or
  JSON with verification findings overlaid.
- **For anarch-lint:** `render` writes the model as Markdown. A diagram of
  subsystems and layers with the current findings on their edges is a first
  step toward the diagrams gap, though none of the projects imports a drawn
  architecture back.

## Not Worth Adopting

- **Any of the Go resolution.** codegraph-go and gortex resolve with
  tree-sitter, codegrapher with go/parser and no types (`resolve/resolve.go`
  `pickBestGoNode`: same directory first, then the first candidate);
  codegraph-go matches interfaces by method name and arity
  (`dispatch.SynthesizeImplements`). code-graph has no Go analyzer
  (`api/analyzers/source_analyzer.py`). go/types is strictly better.
- **Node IDs containing line numbers** (codegrapher `model.GenerateNodeID`).
  They change on unrelated edits. anarch-lint's fingerprints already exclude
  lines, as codegraph-go's node IDs do (`internal/indexer/nodeid`).
- **Incremental re-indexing.** codegrapher re-extracts only changed files,
  confirmed by content hash; codegraph-go adds their direct call-graph
  dependents. Both suit a search index, but propagation cost and cycles are
  whole-program. If anarch-lint caches, key the cache on content hashes, the
  tool version and `go.sum`, and keep a version stamp that forces a full run,
  as codegrapher's hand-bumped `ExtractionVersion` does. Never trust mtime.
- **Health, recency and PR-risk scores** (gortex
  `internal/mcp/tools_analyze_hotspot_modes.go` `rerankHotspots` and
  `tools_analyze_health_score.go`, which read `time.Now()`, and `prrisk.go`,
  with hand-tuned weights and keyword matching). gortex's `FindHotspots`
  itself is deterministic, but its ranking is heuristic.
- **Regex contract extractors** (gortex `grpc.go`, `graphql.go`,
  `openapi.go`, `shape_proto.go`, `topics.go`, `envvar.go`). Take the contract
  model of §5.1 and detect with go/types and the parsers it names.
  codegraph-go's route detectors (`internal/indexer/routes/`) match
  tree-sitter call nodes, which is better, but still untyped.
- **LLM-written queries and unblinded judges** (code-graph `api/llm.py`;
  gortex `internal/eval/recall/judge.go`). Weaker than the judgement slots.
- **Line-span digests for staleness** (codetospec `internal/drift/drift.go`
  hashes `path|lines` and the content there: a shift goes `stale` only if the
  content now at those lines differs, and `broken` past the end of the file).
  Hash the resolved declaration instead. A `broken` status, for evidence that
  no longer resolves, is worth having.
- **Caching a verdict by the cited code alone.** codetospec's own docs (§3.8)
  reject it, since two claims about the same code deserve different verdicts;
  ADR 0007 agrees.
- **Daemons, watchers and file locks** (codegrapher `watch/`, `daemon/`,
  `freshness/`; codegraph-go). A CI gate doesn't run long enough to need them.
- **feelc's Z3 backend** (`verify_smt.go`, build tag `smt`), which runs an
  external `z3` and reports `not-verifiable` without it.
- **zen-go as a dependency.** `cgo.go` blank-imports a prebuilt Rust static
  library for each of five platforms, 38.5 to 80.8 MiB each and about 293 MB in
  all, so every consumer downloads every platform's. That breaks pure-Go and
  reproducible builds, and the binary can't be audited from Go. Its Go API is
  `Evaluate`, a batch form, expressions and templates, with an optional raw
  JSON trace; it exposes no validation or analysis.
- **Copying go-rule-engine's code directly.** Its `go.mod` asks for Go
  1.27.0; port ideas.

## Gaps No Project Addresses

- **Team structure.** gortex reads CODEOWNERS into `owns` edges, as
  anarch-lint does for distance; none models team cognitive load or
  communication paths.
- **Importing a drawn architecture** (C4, D2, PlantUML), to keep it and
  `anarch.json` in step. feelc draws; nothing reads a drawing.
- **Generated code in the graph.** 2.3 lists it; measuring it needs a model
  choice about whether generated code may break the rules it serves.
- **A base revision in a worktree.** `--new-from-rev` filters findings by
  line; comparing whole reports across revisions (4.3, 6.1) needs `TODO.md`'s
  worktree checkout.

## Suggested Order

Of what remains, after the work in "Status":

1. The judge's promotion to `automatic`, once gold labels exist (§7.5), and
   the adversary role.
2. Invariants from guard shapes (9.5), once invariants are decided to be
   evidence or advisory.

## Licensing

Ideas carry over freely. To copy code: gortex, feelc and codegrapher are
Apache-2.0, so keep their notices (gortex's `NOTICE` names its vendored
`renameio` and `go-pointer`); codegraph-go, codetospec, go-rule-engine,
code-graph and zen-go are MIT, so keep the copyright line, and codegraph-go's
`NOTICE` file.
