# `anarch-lint`

anarch-lint measures the architecture of a Go module, gates its degradation,
and says how to fix what got worse. It follows *Software Architecture
Metrics* (Ciceri et al., O'Reilly, 2022) and Carola Lilienthal's 2008
dissertation on architectural complexity.

- **Deterministic.** The same source tree, build tags and model produce
  byte-identical output: no timestamps, no absolute paths, no map order.
- **Systemic and granular.** It gates the whole module against a baseline,
  and with `--new-from-rev` it gates only the findings resting on lines a
  change touched: a cycle counts when the change touched any of its edges
  ([ADR 0011](docs/adr/0011-a-change-scopes-what-rests-on-it.md)).
- **Actionable.** Every finding names its location and its evidence, and
  gives a fix. For a cycle, that's the smallest set of imports or references
  to cut.
- **Honest about judgement.** What needs a reviewer is a *judgement slot*,
  never a score. A slot never gates by itself; only an approved verdict can
  turn a candidate into a finding.

## Install and Run

```sh
go install github.com/StevenACoffman/anarch-lint@latest
anarch-lint analyze                  # report every measure, finding and slot
anarch-lint connascence              # report connascence: kinds, strength, degree, locality
anarch-lint balance --since-rev main # report Balanced Coupling for every subsystem pair
anarch-lint balance --coverprofile cover.out # rank widely imported, rarely tested packages
anarch-lint fitness                  # report ArchUnit-style fitness rules, metrics and cycles
anarch-lint baseline                 # record the accepted state
anarch-lint check                    # gate against it
anarch-lint check --new-from-rev main
anarch-lint check --baseline-rev main # gate against main's code, measured under today's model
anarch-lint explain example.com/app/a example.com/app/b # which rules examine an import, and why, or would if it existed
anarch-lint fmt --check              # is the model file in its canonical form
anarch-lint judge emit --config a=MODEL --config b=MODEL # ask judges the judgement slots
anarch-lint diagram components.puml  # draft a model from a PlantUML diagram
anarch-lint draft                    # draft a model from the import graph's communities
anarch-lint --format sarif check     # the gate's result as SARIF, for code scanning
anarch-lint baseline --shrink        # remove what was fixed; accept nothing new
anarch-lint render                   # write anarch.md, the model as Markdown
anarch-lint render --check           # exit 3 if anarch.md is stale
anarch-lint schema                   # print the JSON Schema of anarch.json
just corpus                          # the acceptance tests on cloned codebases
```

The shared flags come before the subcommand:

| Flag           | Meaning                                                                                                                                 |
| -------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `-C`, `--dir`  | The directory to analyze. Reported paths are relative to it.                                                                            |
| `--model`      | The model file, JSON or YAML. The default is `anarch.json`, `anarch.yaml` or `anarch.yml` in `--dir`, if present.                       |
| `--judgements` | Verdicts for the judgement slots.                                                                                                       |
| `--codeowners` | The CODEOWNERS file, matched relative to `--dir`. The default is the first of `.github/CODEOWNERS`, `CODEOWNERS` and `docs/CODEOWNERS`. |
| `--tags`       | A build tag (repeatable).                                                                                                               |
| `--format`     | `text` or `json`, or `sarif` for `check`.                                                                                               |

Exit status:

| Status | Meaning                                                                |
| ------ | ---------------------------------------------------------------------- |
| 0      | Healthy, or the command wrote its report.                              |
| 1      | The run failed: packages didn't load, or a file is missing or invalid. |
| 2      | The command line was wrong.                                            |
| 3      | The gate failed.                                                       |
| 4      | `judge admit`: a judge configuration missed a canary.                  |

## What It Measures

- **The Modularity Maturity Index**, with all 22 criteria and scores from
  Table 4-2. anarch-lint reports it as an interval: a criterion without a score
  counts as 0 toward the low bound and 10 toward the high bound.
- **The thirteen ordering measures of the dissertation**: cycle extent, cycle
  reach, entanglement and cycle size, at the type, package and subsystem
  levels.
- **von Zitzewitz's coupling metrics** at the same three levels:
  - ACD and propagation cost;
  - relative cyclicity;
  - the Structural Debt Index, with the edges to cut.
- **Size and complexity**: lines of code, statements, three variants of
  cyclomatic complexity, cognitive complexity (gocognit's SonarSource count),
  nesting depth, and LCOM4.
- **Package metrics**: Robert C. Martin's afferent and efferent coupling,
  instability, abstractness and distance from the main sequence.
- **Component Rank**: the types most depended upon, the ones to read first.
- **Communities**: the groups of packages the import graph suggests (gonum's
  Louvain, split into connected parts, with a fixed seed), and the packages
  whose community mostly lies in another subsystem. They are advice and
  never gate.
- **Conformance**: how much of the code the model's subsystems claim
  (coverage), and how many of its rules hold (compliance).
- **Architecture rules** declared in the model, among them layers,
  subsystems and their interfaces, pattern roles, forbidden imports and
  cycle limits.
- **Connascence** (`connascence`): every instance of the nine kinds of
  connascence.io, from name to identity, and of contranascence. Each has a
  strength, a degree (how many entities must change together) and a locality
  (the nearest container of every site, from a function to several modules).
  Facts gate; candidates, which may be coincidence, gate only once a reviewer
  confirms them. A fact stronger than the model allows at its locality is a
  finding. Partial clones, runs of code repeated in two or more packages, come
  from `golangci/dupl`.
- **Balanced Coupling** (`balance`): Vlad Khononov's model for every ordered
  pair of subsystems. Each pair has an integration strength (intrusive,
  functional, model or contract), a distance (module, CODEOWNERS or model
  owner, and deploy boundaries, and confirmed asynchronous integration), and
  the upstream's volatility, a judged subdomain with churn as evidence. Tight
  coupling on facts is a finding. A request to a route that a
  `fitness.openapi` document declares for its serving packages couples the
  two at the contract level, even when the code registers the route at run
  time. The report also weighs each package's
  coupling by strength. With `--since-rev`, it lists the subsystems that
  change in the same commits (co-change) and the packages that change often
  and are complex (hotspots); two subsystems that co-change without importing
  each other are a `hidden-coupling` candidate. With `--coverprofile`, a
  `go test -coverprofile` file, it ranks packages by afferent coupling times
  uncovered statements; one that three or more packages import and whose
  tests run less than half of it is an `untested-dependency` candidate.
  With both, a function the range adds or changes that calls into another
  subsystem and that no test runs is an `untested-crossing` candidate.
- **Fitness** (`fitness`): what ArchUnit tests in Java.
  - Every rule of the model's `fitness` section, with its reason, gate,
    matches, violations and status.
  - Lakos's metrics with NCCD, Dowalil's visibility metrics, and ArchUnit's
    component dependency metrics.
  - The elementary cycles of the type and subsystem graphs, in a fixed order
    where ArchUnit's depends on hash iteration.
  - Conformance to modelith domain models: every entity and enum should be a
    Go type, every relationship a reference between types, and every enum's
    values the type's constants; every invariant should have an enforcer,
    and its entities' state no exported fields. A rule without a reason is
    a warning.
  - A check of the import rules against each other, over classes of packages
    every rule reads alike: a rule another forbids everything of is a
    `redundant-rule` warning, and a layer or importer a rule allows by name,
    which other rules forbid entirely, is an `unusable-allowance` warning.
- **What it did not see**: the generated files, the files build constraints
  leave out, and the modules below the patterns that `./...` doesn't cross
  into (`other-module`) are listed with their reason, and `check` counts them
  under its verdict. The share of source files left out, generated or by the
  build, is ratcheted like the other metrics (`files.excludedShare`), so
  code cannot leave the measures by being marked generated. Test files are
  never measured; their imports are read only for the import rules that opt
  in with `tests`.
  - A file that imports C is measured as its source, not as the file cgo
    writes in its place.
  - The imports of the files build constraints leave out are read from their
    source. The import rules check them: forbidden imports, isolations, and
    the fitness rules on imports. A forbidden import in a `_windows.go` file
    fails on every platform. The measures read only the build.
  - The report records the platform (`go env GOOS GOARCH`) and the tags, and
    so does the baseline. `check` against a baseline of another build notes
    the difference instead of failing, and `baseline --shrink` leaves such a
    baseline as it is.

Every finding names the rule of the model it breaks (`modelRule`), such as
`forbiddenImports[2]` or `fitness.layerAccess web`. A pattern that matches
nothing (`stale-pattern`, `empty-selector`) suggests a correction when one
segment looks misspelled: `example.com/app/servce/...: did you mean
example.com/app/service/...?` A type cycle's fix says
where each cut's uses are (`embed`, `field`, `definition`, `signature` or
`body`), the hardest to remove first.

The definitions behind connascence and Balanced Coupling are in
`connascence.md`. The balance
score uses archfit's ordinals and formula, which nobody has checked against
the book, so the report shows the score and the gate ignores it.

## The Model File

`anarch.json` declares the target architecture. Patterns are import paths;
a pattern ending in `/...` includes everything beneath it. This repository
measures itself with its own [`anarch.json`](anarch.json), and `just
self-check` gates it against [`anarch-baseline.json`](anarch-baseline.json).

```json
{
  "version": 1,
  "subsystems": [
    {"name": "billing", "packages": ["example.com/app/billing/..."],
     "interface": ["example.com/app/billing"]}
  ],
  "layers": [
    {"name": "cmd", "packages": ["example.com/app/cmd/..."]},
    {"name": "domain", "packages": ["example.com/app/billing/..."]}
  ],
  "forbiddenImports": [
    {"from": ["example.com/app/billing/..."], "imports": ["database/sql"],
     "reason": "domain code stays free of persistence"}
  ]
}
```

The model may be YAML instead, read as the JSON it stands for, so both
spellings of one model hash alike.

Without a model, the criteria that need one report themselves *unmeasured*,
with the reason. They are never scored as zero.

Beyond subsystems, layers and forbidden imports, a model can declare the
following.

| Key                                              | What it does                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `subsystemOrder`                                 | Orders subsystems from top to bottom, for criterion 2.1.2.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `roles`                                          | Pattern roles, ordered from top to bottom. A type plays a role by an `//arch:role NAME` comment, by embedding a marker type (`embeds`), by implementing an interface (`implements`), by a name `suffix`, or by its `packages`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `isolations`                                     | `{"from": "example.com/app/$*/...", "mayImport": ["example.com/app/$1/..."]}` keeps every top-level area to itself. `$*` captures one path segment, `$**` several, and `$1` to `$9` refer back to them.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `forbiddenImports[].transitive`                  | Also forbids reaching the package through other packages. The finding shows the import path, and names the one package every path passes through when there is one.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `forbiddenImports[].injected`                    | Also forbids receiving a value of a forbidden package's type through an interface parameter: a dependency that only exists at run time.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `forbiddenImports[].tests`, `isolations[].tests` | Also checks the imports of the packages' test files, internal and external, read from their source, so a test cannot reach into another area's internals. The finding says the import is a test's.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `gates`                                          | `minCoverage` and `minCompliance` floors, in percent. `check` fails below either, whatever the baseline says.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `thresholds`                                     | The size, complexity and nesting limits. Every one has a default, recorded among the report's assumptions.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `connascence`                                    | `{"maxStrength": {"package": 9, "subsystem": 5, "system": 3, "external": 2}, "maxDegree": {"system": 20}}`: the strongest kind (1 name to 9 identity) and the largest degree each locality allows. Setting either opts the connascence findings into `baseline` and `check`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `balance`                                        | `{"gate": true, "messaging": ["github.com/nats-io/nats.go/..."]}`: `gate` opts the Balanced Coupling findings into `baseline` and `check`; `messaging` names message-bus clients, whose importers get an asynchrony slot.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `fitness`                                        | ArchUnit-style rules, each with a `name`, a `reason`, a `gate` (`fail`, `warn` or `off`) and `optional`: `requireAssigned`, `strictLayers`, `layerAccess` (`mayOnlyBeAccessedBy`, `mayOnlyAccess`, `mayNotBeAccessedByAny`, `mayNotAccessAny`, `consideringAll`), `onion`, `slices` (`$*` groups, `freeOfCycles`, `notDependOnEachOther`), `dependents` (`onlyFrom`), `calls` (callees such as `time.Now` or `os.*`, with `only`), `interfacesOnly`, `libraries` (`from`, `allow`: the only third-party packages the `from` packages may import; an approval nothing uses is a warning), `fanOut` (`packages`, `max`: how many packages under analysis each may import), `coChange` (`when`, `require`, `except`: doublestar file globs such as `api/**/*.proto`, checked only by `check --new-from-rev`) and `domains` (see below). A rule that matches nothing fails unless optional, and so does a forbidden import, isolation (`from` or `mayImport`), role, subsystem interface or balance messaging pattern that matches nothing. |
| `subsystems[].name` captures                     | A subsystem named `$1` whose `packages` capture with `$*` (`example.com/app/$*/...`) is a template: it becomes one subsystem per distinct capture, as ArchUnit's slices.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `findingRules`                                   | Gives a rule no model entry governs, such as `large-func` or `layer-order-violation`, a `gate`, a `reason` and a `qualityAttribute`. A forbidden import, isolation or fitness rule takes the same three keys and governs only its own findings.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `qualityAttribute`                               | One of ISO 25010's characteristics, `functional_suitability`, `performance_efficiency`, `compatibility`, `usability`, `reliability`, `security`, `maintainability` and `portability`, or `availability`, `scalability`, `compliance` and `safety`. Findings carry it into the JSON and SARIF reports, and `anarch.md` groups rules by it.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `thresholdOverrides`                             | `[{"packages": ["example.com/app/gen/..."], "thresholds": {"largeFuncLines": 200}}]` gives packages their own structural limits; the first match wins, and the MMI keeps the module's.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| `fitness.fields`                                 | `{"from": [...], "fields": ["example.com/app/order.Order.Status"], "writes": true, "only": true}` restricts reading or writing struct fields, as ArchUnit's `accessField`. A field may be `pkg.Type.*` or `pkg.*`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `fitness.unusedExports`                          | `{"packages": [...]}` reports exported types, functions and constants that no other package nor external test uses, counting what is reachable through used API. Methods and package `main` are never reported.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `fitness.openapi`                                | `{"path": "api/openapi.yaml", "servedBy": ["example.com/app/rest/..."]}` compares an OpenAPI 3 or Swagger 2.0 document with the routes the serving packages register: `openapi-operation-unserved` for an operation none registers, `openapi-route-undocumented` for a route the document leaves out. Parameters compare however they are spelled, the servers' base paths prefix every operation, and `fmt.Sprintf` of constants counts as a constant route. The document, and every file its `$ref`s name, must lie within the repository; nothing is fetched over the network.                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `fitness.graphql`                                | `{"schema": ["graph/*.graphqls"], "operations": ["client/*.graphql"], "servedBy": ["example.com/app/graph"]}` compares a GraphQL schema with its gqlgen resolvers and its clients' operations: `graphql-field-unresolved` for a query, mutation or subscription field with no resolver method in a serving package, named as gqlgen names it, and `graphql-operation-invalid` for an operation document that does not parse or validate against the schema. The globs are doublestar patterns under the analysis root, each must match a file, and no file may lie outside the repository.                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `fitness.protobuf`                               | `{"files": ["api/proto/*/v1/*.proto"], "importPaths": ["api/proto"], "servedBy": ["example.com/app/grpc"]}` compiles `.proto` files with `protocompile` and compares them with the gRPC servers the serving packages register: `proto-service-unregistered` for a service none registers, `proto-service-undeclared` for a registration from one of the files' Go packages that they do not declare, and `proto-constraint-invalid` for a message whose `buf.validate` constraints `protovalidate` cannot compile. Imports resolve against `importPaths` (the analysis root when empty), the standard imports and `buf/validate/validate.proto`; no file may lie outside the repository.                                                                                                                                                                                                                                                                                                                                                |
| `fitness.cases`                                  | `{"name": "billing-isolated", "from": "example.com/app/billing/internal", "imports": "example.com/app/shipping", "expect": "forbidden", "by": "isolations[0]"}` declares an import, which the code need not have, and what the rules should decide about it: `forbidden`, by the rule `by` names as findings do in `modelRule`, or by any; or `allowed`, by no rule forbidden. The rules run as `explain` runs them, on the program with the import added, so a model change that stops catching the case is a `model-case-contradicted` finding. Both packages must be under analysis.                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `fitness.boundaries`                             | `{"files": ["web/*.go"], "uses": ["example.com/app/legacy/..."]}` keeps a change to the files from importing or using anything of the packages: code that already does may stay, but no changed line may add more, so a dependency being retired only shrinks. Like `coChange`, it checks only a change set (`check --new-from-rev`); a `boundary-crossed` finding names the line.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `balance.packages`                               | `true` measures Balanced Coupling between packages as well as subsystems.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `subsystems[].owner`                             | The team that owns a subsystem, for the owner boundary. It takes precedence over CODEOWNERS.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |

Adding or changing any section changes the model's hash, so record a new
baseline afterwards. Reordering a list does not, except `layers`,
`subsystemOrder`, `roles` and `thresholdOverrides`, whose order means
something. The hash's canonical form is new in this release: every
existing baseline is orphaned once, and needs re-recording with
`anarch-lint baseline`.

A package comment can assign its package instead of the model, with
`//arch:subsystem NAME`, and mark it part of its subsystem's interface with
`//arch:exposes`. The directives apply on load and leave the hash
unchanged; two files of a package naming different subsystems fail the
load.

`check --new-from-rev` fails a change that loosens the bars it is measured
against while also changing non-test Go code: a raised threshold, a
lowered floor, a rule removed or its gate weakened, findings accepted into
the baseline, or a waiver added or extended. Loosen them in a change of
their own. Each finding carries its `reach`, how many packages depend on
where it rests, and `check` ranks new findings by it. `anarch-lint schema` prints the model's JSON Schema,
generated from the types anarch-lint reads; name its `$id` in the model's
`$schema` key for editor completion.

## Domain Models

A fitness `domains` rule compares a [modelith](https://modelith.sh) domain
model, a `*.modelith.yaml` file, with the code that should express it:

```json
{"name": "core-speaks-its-model", "path": "anarch-lint.modelith.yaml",
 "subsystem": "domain", "types": {"Finding": "example.com/app/finding.Diagnostic"},
 "reason": "the core keeps the vocabulary of its domain model"}
```

An entity or enum matches the type of its name in the rule's `subsystem`
(the whole program without one), or the type `types` maps it to where the
names differ on purpose. A type can also say which entity it expresses with
an `//arch:entity NAME` comment, which takes precedence over a match by
name; a directive
naming no entity of a covering model is an `unknown-entity-directive`
warning. The findings are `domain-entity-missing`,
`domain-entity-ambiguous`, `domain-relationship-missing`,
`domain-enum-missing`, `domain-enum-drift`, `domain-enum-order`,
`domain-attribute-missing`, `domain-action-missing`,
`domain-ownership-drift`, `domain-type-unmapped` and
`domain-contract-entity`. The path may not leave the repository, and nothing
is fetched over the network. anarch-lint decodes a model strictly, so a
field the format lacks is an error, but it does not lint the model: check
it with `modelith lint`, which the error for an invalid model also
suggests.

The rule also checks the model's invariants
([ADR 0016](docs/adr/0016-invariants-are-checked-through-their-enforcers.md)).
No rule can tell whether code keeps a sentence, so a person says once, in
the code, where an invariant is enforced: an `//arch:invariant ID` comment
on the function or test that keeps it, one comment per invariant. Test
files are read from source for the comment.

```go
// Close ends the visit, refusing a second open visit for the car.
//
//arch:invariant one-open-visit
func (v *Visit) Close() error {
```

The checks:

- `invariant-unenforced`: no function or test of the rule's packages
  carries a directive naming the invariant.
- `invariant-exposed`: an entity the invariant governs, the one that
  declares it or one its statement names in backticks, has a type with an
  exported field. Code outside the type's package can then change the state
  without the actions that preserve it. The evidence lists the writes from
  other packages; the type's own package may write its fields.
- `invariant-untested`, with `fitness --coverprofile FILE`: every enforcer
  is a function that no test runs. A test enforcer is enough. `check` reads
  no profile, so the gate never reports it.
- `unknown-invariant-directive`, a warning: a directive names no invariant
  of a covering model.

A domain model is also evidence for the judgement slots: the subsystem it
describes, and the models it imports, appear in the bounded-context and
integration-pattern slots. It never answers them
([ADR 0005](docs/adr/0005-domain-inferences-are-evidence.md)). This
repository checks itself against
[`anarch-lint.modelith.yaml`](anarch-lint.modelith.yaml).

## Inside `go test`

`anarchtest` runs `check` as a test, as ArchUnit's rules run under JUnit:

```go
func TestArchitecture(t *testing.T) {
	anarchtest.Check(t, nil)
}
```

The zero options check `./...` against the model and baseline in the
package's directory. Loading the packages takes seconds.

## Rendering and Agent Skills

`anarch-lint render` writes `anarch.md` beside the model: the subsystems,
the layers, a Mermaid flowchart of the measured subsystem dependencies, with
each one that lies on a cycle or runs up the subsystem order labelled and
drawn in red, the import rules, the fitness rules with their reasons, and the domain models.
The rendering is byte-identical for the same model and code, and model text
is escaped so it cannot inject HTML
([ADR 0006](docs/adr/0006-model-text-is-plain-text.md)). Commit it, and run
`anarch-lint render --check` in CI to catch drift.

The [`plugin/`](plugin) directory is a Claude Code plugin with two skills:
`anarch-model-author` builds a model by conversation, a pass at a time, and
`anarch-model-context` loads the rendered architecture before a coding task.

## Waivers

A waiver accepts one new finding until a date, with a reason and an approver.
Waivers live in `anarch-waivers.json`, or the file `--waivers` names, so that
adding one leaves the model, and with it the baseline, unchanged:

```json
{
  "version": 1,
  "waivers": [
    {"fingerprint": "0123456789abcdef", "reason": "migrating in Q3",
     "approvedBy": "a reviewer", "expires": "2027-01-01"}
  ]
}
```

An expired waiver protects nothing, and `check` reports it, as it reports
a waiver whose finding is gone. When findings
were fixed or metrics improved, `check` says the baseline can shrink, and
`baseline --shrink` rewrites it with only those changes: it never accepts
anything new.

`--today` sets the date waivers expire against; it defaults to the current
date.

## Judgement Slots

Five MMI criteria need a reviewer: 1.1.5, 1.1.6, 1.2.2, 3.3 and 3.4. Each
slot carries:

- the question and its `edition`, a digest of its words and answers;
- the evidence a reviewer should weigh;
- the `context`, what an answer would set off, which a human sees and an
  automated judge never does
  ([ADR 0008](docs/adr/0008-the-judge-is-blinded.md));
- `undecided`, why no deterministic measure settles it;
- a hash of all of these.

The connascence and balance commands add slots named `family/subject`,
answered with one of the slot's `answers` instead of a score:

| Family                | Asks                                                | Answers                                                                                                                          |
| --------------------- | --------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `connascence`         | Must the candidate's places change together?        | `confirmed`, `rejected`                                                                                                          |
| `balance`             | Is the Balanced Coupling candidate real?            | `confirmed`, `rejected`                                                                                                          |
| `subdomain`           | Is the subsystem core, supporting or generic?       | `core`, `supporting`, `generic`                                                                                                  |
| `bounded-context`     | Is the subsystem a bounded context?                 | `yes`, `no`                                                                                                                      |
| `integration-pattern` | Which DDD integration pattern does the pair follow? | `shared-kernel`, `partnership`, `conformist`, `customer-supplier`, `anti-corruption-layer`, `open-host-service`, `separate-ways` |
| `contract-type`       | Is the exported type a model type or a contract?    | `model`, `contract`                                                                                                              |
| `async`               | Does the package integrate asynchronously?          | `async`, `sync`                                                                                                                  |

Every slot also accepts `insufficient-evidence`, and an MMI verdict may give
it instead of a score. It leaves the slot `unresolved`, which decides
nothing ([ADR 0009](docs/adr/0009-insufficient-evidence-decides-nothing.md)).

```json
{"version": 1, "signer": "reviewer@example.com", "verdicts": [
  {"criterion": "subdomain/billing", "answer": "supporting",
   "rationale": "an off-the-shelf invoicing flow", "evidenceHash": "…",
   "edition": "…", "events": [{"role": "judge", "config": "model-v1"}, {"role": "human"}]}
]}
```

Each verdict's `events` say who attests it: a `judge`, with the hash of its
configuration, or a `human`. Every event's actor is the file's `signer`; an
event cannot name its own. A verdict applies only when a human event
attests it and its hash matches the current evidence. Otherwise the slot is:

- `pending`, with no verdict;
- `unapproved`, with only a judge's draft;
- `stale`, when the evidence or the question changed (`staleBecause` says
  which; record the slot's `edition` on the verdict to tell them apart);
- `unresolved`, when the verdict found the evidence insufficient.

The first format, with `provenance` and `approved` instead of `events`,
still reads: an approved verdict as a human event, and a draft as a judge's.

`anarch-lint judge` relays the slots to judges it does not run itself, such
as LLM agents. `judge emit` writes one prompt per pending or stale slot,
judge configuration (at least two) and sample under `.anarch/judging/run`,
mixing in canaries of known answer. `judge admit` parses the replies
strictly, refuses a configuration's answers in any family whose canaries it
missed (exit status 4), tallies the rest, and records them as judges'
events: drafts until a human attests them. `judge report` shows the
canaries, the gold set's agreement and calibration (`--gold`), the tallies
and the rejected replies.

A slot never gates by itself:

- an MMI verdict narrows the MMI interval;
- a confirmed candidate becomes a finding, and a rejected one drops out;
- a supporting or generic subdomain suppresses tight coupling to it, as its
  low volatility balances the coupling;
- a shared kernel, partnership or conformist pattern suppresses a leaked
  model.

Each suppression is listed in balance's `suppressed`, with the slot and its
events, and needs a human
([ADR 0010](docs/adr/0010-suppressing-a-fact-needs-a-human.md)). A slot whose
answer would change nothing is not opened: balance lists it in `skipped`,
with the reason.

## Design

anarch-lint is a functional core with an imperative shell.

- `internal/arch` is pure: it does no I/O, reads no clock and draws nothing
  from the global random source. It holds the vocabulary every measure
  shares. It may use only the third-party libraries the self-check's
  `core-approved-libraries` rule approves, each pure, deterministic and well
  tested: gonum for graph algorithms and communities, doublestar for file
  globs, and levenshtein for the corrections of misspelled patterns
  ([ADR 0012](docs/adr/0012-the-core-approves-its-libraries.md)).
- `internal/connascence`, `internal/balance`, `internal/fitness` and
  `internal/judging` are pure
  measures over
  `arch`'s facts. None imports another or any adapter; `cmd` passes the
  connascence instances to `balance`. The repository's own model enforces
  this.
- The adapters wrap one dependency each:
  - `internal/gopackages`: `golang.org/x/tools/go/packages`, with
    `gocognit` and `dupl` over the syntax trees;
  - `internal/git`: the `git` command line, with diffs parsed by
    `bluekeyes/go-gitdiff`;
  - `internal/codeowners`: GitHub's CODEOWNERS format, through
    `hmarr/codeowners`;
  - `internal/coverage`: coverage profiles, through `golang.org/x/tools/cover`;
  - `internal/sarif`: SARIF 2.1.0, through `owenrumney/go-sarif`;
  - `internal/modelith`: modelith's YAML, through
    `StevenACoffman/indomain`, the reader codetospec shares;
  - `internal/openapi`: OpenAPI and Swagger documents, through
    `getkin/kin-openapi`, reading only the files the shell allows;
  - `internal/graphql`: GraphQL schemas and operation documents, through
    `vektah/gqlparser/v2`;
  - `internal/protobuf`: `.proto` files, through `bufbuild/protocompile`,
    and their `buf.validate` constraints, through
    `buf.build/go/protovalidate`;
  - `internal/plantuml`: PlantUML component diagrams, by a small parser of
    its own, since no Go library reads them;
  - `internal/text`: `text/tabwriter`, and Markdown for `render`;
  - `internal/schema`: the model's JSON Schema, generated by reflection.
- `internal/pipeline` wires the adapters together, for `cmd` and
  `anarchtest` alike.

Decisions a well-meant change might undo are recorded in
[`docs/adr/`](docs/adr), each pinned by a test named after it.

The definitions behind every measure, the survey of other tools, and the
implementation plan are in four companion documents: `mmi.md`,
`mmi-ref-plan.md`, `mmi-plan.md` and `connascence.md`.
