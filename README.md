# `anarch-lint`

anarch-lint measures the architecture of a Go module, gates its degradation,
and says how to fix what got worse. It follows *Software Architecture
Metrics* (Ciceri et al., O'Reilly, 2022) and Carola Lilienthal's 2008
dissertation on architectural complexity.

- **Deterministic.** The same source tree, build tags and model produce
  byte-identical output: no timestamps, no absolute paths, no map order.
- **Systemic and granular.** It gates the whole module against a baseline,
  and with `--new-from-rev` it gates only the lines a change touched.
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
anarch-lint fitness                  # report ArchUnit-style fitness rules, metrics and cycles
anarch-lint baseline                 # record the accepted state
anarch-lint check                    # gate against it
anarch-lint check --new-from-rev main
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
| `--model`      | The model file. The default is `anarch.json` in `--dir`, if present.                                                                    |
| `--judgements` | Verdicts for the judgement slots.                                                                                                       |
| `--codeowners` | The CODEOWNERS file, matched relative to `--dir`. The default is the first of `.github/CODEOWNERS`, `CODEOWNERS` and `docs/CODEOWNERS`. |
| `--tags`       | A build tag (repeatable).                                                                                                               |
| `--format`     | `text` or `json`.                                                                                                                       |

Exit status:

| Status | Meaning                                                                |
| ------ | ---------------------------------------------------------------------- |
| 0      | Healthy, or the command wrote its report.                              |
| 1      | The run failed: packages didn't load, or a file is missing or invalid. |
| 2      | The command line was wrong.                                            |
| 3      | The gate failed.                                                       |

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
  cyclomatic complexity, nesting depth, and LCOM4.
- **Package metrics**: Robert C. Martin's afferent and efferent coupling,
  instability, abstractness and distance from the main sequence.
- **Component Rank**: the types most depended upon, the ones to read first.
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
  finding.
- **Balanced Coupling** (`balance`): Vlad Khononov's model for every ordered
  pair of subsystems. Each pair has an integration strength (intrusive,
  functional, model or contract), a distance (module, CODEOWNERS or model
  owner, and deploy boundaries, and confirmed asynchronous integration), and
  the upstream's volatility, a judged subdomain with churn as evidence. Tight
  coupling on facts is a finding. The report also weighs each package's
  coupling by strength.
- **Fitness** (`fitness`): what ArchUnit tests in Java.
  - Every rule of the model's `fitness` section, with its reason, gate,
    matches, violations and status.
  - Lakos's metrics with NCCD, Dowalil's visibility metrics, and ArchUnit's
    component dependency metrics.
  - The elementary cycles of the type and subsystem graphs, in a fixed order
    where ArchUnit's depends on hash iteration.
  - Conformance to modelith domain models: every entity and enum should be a
    Go type, every relationship a reference between types, and every enum's
    values the type's constants. A rule without a reason is a warning.

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

Without a model, the criteria that need one report themselves *unmeasured*,
with the reason. They are never scored as zero.

Beyond subsystems, layers and forbidden imports, a model can declare the
following.

| Key                             | What it does                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `subsystemOrder`                | Orders subsystems from top to bottom, for criterion 2.1.2.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `roles`                         | Pattern roles, ordered from top to bottom. A type plays a role by an `//arch:role NAME` comment, by embedding a marker type (`embeds`), by implementing an interface (`implements`), by a name `suffix`, or by its `packages`.                                                                                                                                                                                                                                                                                                                                                                            |
| `isolations`                    | `{"from": "example.com/app/$*/...", "mayImport": ["example.com/app/$1/..."]}` keeps every top-level area to itself. `$*` captures one path segment, `$**` several, and `$1` to `$9` refer back to them.                                                                                                                                                                                                                                                                                                                                                                                                   |
| `forbiddenImports[].transitive` | Also forbids reaching the package through other packages. The finding shows the import path, and names the one package every path passes through when there is one.                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `forbiddenImports[].injected`   | Also forbids receiving a value of a forbidden package's type through an interface parameter: a dependency that only exists at run time.                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `gates`                         | `minCoverage` and `minCompliance` floors, in percent. `check` fails below either, whatever the baseline says.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `thresholds`                    | The size, complexity and nesting limits. Every one has a default, recorded among the report's assumptions.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `connascence`                   | `{"maxStrength": {"package": 9, "subsystem": 5, "system": 3, "external": 2}, "maxDegree": {"system": 20}}`: the strongest kind (1 name to 9 identity) and the largest degree each locality allows. Setting either opts the connascence findings into `baseline` and `check`.                                                                                                                                                                                                                                                                                                                              |
| `balance`                       | `{"gate": true, "messaging": ["github.com/nats-io/nats.go/..."]}`: `gate` opts the Balanced Coupling findings into `baseline` and `check`; `messaging` names message-bus clients, whose importers get an asynchrony slot.                                                                                                                                                                                                                                                                                                                                                                                 |
| `fitness`                       | ArchUnit-style rules, each with a `name`, a `reason`, a `gate` (`fail`, `warn` or `off`) and `optional`: `requireAssigned`, `strictLayers`, `layerAccess` (`mayOnlyBeAccessedBy`, `mayOnlyAccess`, `mayNotBeAccessedByAny`, `mayNotAccessAny`, `consideringAll`), `onion`, `slices` (`$*` groups, `freeOfCycles`, `notDependOnEachOther`), `dependents` (`onlyFrom`), `calls` (callees such as `time.Now` or `os.*`, with `only`), `interfacesOnly` and `domains` (see below). A rule that matches nothing fails unless optional, and so does a forbidden import, isolation or role that matches nothing. |
| `subsystems[].owner`            | The team that owns a subsystem, for the owner boundary. It takes precedence over CODEOWNERS.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |

Adding or changing any section changes the model's hash, so record a new
baseline afterwards. `anarch-lint schema` prints the model's JSON Schema,
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
names differ on purpose. The findings are `domain-entity-missing`,
`domain-entity-ambiguous`, `domain-relationship-missing`,
`domain-enum-missing`, `domain-enum-drift`, `domain-type-unmapped` and
`domain-contract-entity`. The path may not leave the repository, and nothing
is fetched over the network.

A domain model is also evidence for the judgement slots: the subsystem it
describes, and the models it imports, appear in the bounded-context and
integration-pattern slots. It never answers them
([ADR 0005](docs/adr/0005-domain-inferences-are-evidence.md)). This
repository checks itself against
[`anarch-lint.modelith.yaml`](anarch-lint.modelith.yaml).

## Rendering and Agent Skills

`anarch-lint render` writes `anarch.md` beside the model: the subsystems,
the layers, a Mermaid flowchart of the measured subsystem dependencies, the
import rules, the fitness rules with their reasons, and the domain models.
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

An expired waiver protects nothing, and `check` reports it. When findings
were fixed or metrics improved, `check` says the baseline can shrink, and
`baseline --shrink` rewrites it with only those changes: it never accepts
anything new.

`--today` sets the date waivers expire against; it defaults to the current
date.

## Judgement Slots

Five MMI criteria need a reviewer: 1.1.5, 1.1.6, 1.2.2, 3.3 and 3.4. Each
slot carries:

- the question;
- the deterministic evidence a reviewer should weigh;
- a hash of that evidence.

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

```json
{"version": 1, "verdicts": [
  {"criterion": "subdomain/billing", "answer": "supporting", "provenance": "human",
   "rationale": "an off-the-shelf invoicing flow", "evidenceHash": "…", "approved": true}
]}
```

anarch-lint applies a verdict, from a human or an LLM, only when someone
approved it and its hash matches the current evidence. When the code changes
the evidence, the verdict goes stale and needs a fresh review. A slot never
gates by itself:

- an MMI verdict narrows the MMI interval;
- a confirmed candidate becomes a finding, and a rejected one drops out;
- a supporting or generic subdomain suppresses tight coupling to it, as its
  low volatility balances the coupling;
- a shared kernel, partnership or conformist pattern suppresses a leaked
  model.

## Design

anarch-lint is a functional core with an imperative shell.

- `internal/arch` is pure: it does no I/O, reads no clock and uses no
  third-party code. It holds the vocabulary every measure shares.
- `internal/connascence`, `internal/balance` and `internal/fitness` are pure
  measures over
  `arch`'s facts. None imports another or any adapter; `cmd` passes the
  connascence instances to `balance`. The repository's own model enforces
  this.
- The adapters wrap one dependency each:
  - `internal/gopackages`: `golang.org/x/tools/go/packages`;
  - `internal/git`: the `git` command line;
  - `internal/codeowners`: GitHub's CODEOWNERS format;
  - `internal/modelith`: modelith's YAML, through `sigs.k8s.io/yaml`;
  - `internal/text`: `text/tabwriter`, and Markdown for `render`;
  - `internal/schema`: the model's JSON Schema, generated by reflection.
- `cmd` wires the adapters together.

Decisions a well-meant change might undo are recorded in
[`docs/adr/`](docs/adr), each pinned by a test named after it.

The definitions behind every measure, the survey of other tools, and the
implementation plan are in four companion documents: `mmi.md`,
`mmi-ref-plan.md`, `mmi-plan.md` and `connascence.md`.
