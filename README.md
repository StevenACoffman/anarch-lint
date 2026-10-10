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
- **Honest about judgement.** The criteria that need a reviewer are
  *judgement slots*, never scores, and never gate.

## Install and Run

```sh
go install github.com/StevenACoffman/anarch-lint@latest
anarch-lint analyze                  # report every measure, finding and slot
anarch-lint baseline                 # record the accepted state
anarch-lint check                    # gate against it
anarch-lint check --new-from-rev main
just corpus                          # the acceptance tests on cloned codebases
```

The shared flags come before the subcommand:

| Flag           | Meaning                                                              |
| -------------- | -------------------------------------------------------------------- |
| `-C`, `--dir`  | The directory to analyze. Reported paths are relative to it.         |
| `--model`      | The model file. The default is `anarch.json` in `--dir`, if present. |
| `--judgements` | Verdicts for the judgement slots.                                    |
| `--tags`       | A build tag (repeatable).                                            |
| `--format`     | `text` or `json`.                                                    |

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

| Key                             | What it does                                                                                                                                                                                                                   |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `subsystemOrder`                | Orders subsystems from top to bottom, for criterion 2.1.2.                                                                                                                                                                     |
| `roles`                         | Pattern roles, ordered from top to bottom. A type plays a role by an `//arch:role NAME` comment, by embedding a marker type (`embeds`), by implementing an interface (`implements`), by a name `suffix`, or by its `packages`. |
| `isolations`                    | `{"from": "example.com/app/$*/...", "mayImport": ["example.com/app/$1/..."]}` keeps every top-level area to itself. `$*` captures one path segment, `$**` several, and `$1` to `$9` refer back to them.                        |
| `forbiddenImports[].transitive` | Also forbids reaching the package through other packages. The finding shows the import path, and names the one package every path passes through when there is one.                                                            |
| `forbiddenImports[].injected`   | Also forbids receiving a value of a forbidden package's type through an interface parameter: a dependency that only exists at run time.                                                                                        |
| `gates`                         | `minCoverage` and `minCompliance` floors, in percent. `check` fails below either, whatever the baseline says.                                                                                                                  |
| `thresholds`                    | The size, complexity and nesting limits. Every one has a default, recorded among the report's assumptions.                                                                                                                     |

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

An expired waiver protects nothing, and `check` reports it. `--today` sets the
date waivers expire against; it defaults to the current date.

## Judgement Slots

Five MMI criteria need a reviewer: 1.1.5, 1.1.6, 1.2.2, 3.3 and 3.4. Each
slot carries:

- the question;
- the deterministic evidence a reviewer should weigh;
- a hash of that evidence.

anarch-lint applies a verdict, from a human or an LLM, only when someone
approved it and its hash matches the current evidence. When the code changes
the evidence, the verdict goes stale and needs a fresh review. Slots never
gate.

## Design

anarch-lint is a functional core with an imperative shell.

- `internal/arch` is pure: it does no I/O, reads no clock and uses no
  third-party code. The repository's own model enforces that.
- The adapters wrap one dependency each:
  - `internal/gopackages`: `golang.org/x/tools/go/packages`;
  - `internal/git`: the `git` command line;
  - `internal/text`: `text/tabwriter`.
- `cmd` wires the adapters together.

The definitions behind every measure, the survey of other tools, and the
implementation plan are in three companion documents: `mmi.md`,
`mmi-ref-plan.md` and `mmi-plan.md`.
