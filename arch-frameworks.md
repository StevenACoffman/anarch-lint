# How the Architectural Frameworks of `anarch-lint` Fit Together

anarch-lint measures one Go codebase with several architectural frameworks.
Each asks a different question about the same code, and each covers a blind
spot of the others. This document says what each one asks, how they feed
each other in the code, where they disagree, and what none of them covers.
Every claim was checked against the source; file references are relative to
the repository root.

## What Each Framework Asks

| Framework                                                    | Question                                                                                                                            | Answer                                                                                                                                       | Basis                                                 | Gates                                                                                      |
| ------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| **MMI** (Lilienthal), `analyze`                              | Is the code allocated to subsystems, layers and pattern roles, and kept free of interface bypasses, layering violations and cycles? | 17 measured criteria and 5 reviewer criteria, combined into a score interval from 0 to 10                                                    | Measured, plus judgement slots                        | Always: `mmi.low` is ratcheted                                                             |
| **Architecture rules**, `analyze`                            | Does the code keep the model's subsystems, interfaces, layers, roles, forbidden imports and isolations?                             | Findings, such as `interface-bypass` and `subsystem-cycle`                                                                                   | Declared in `anarch.json`, checked against the code   | Always                                                                                     |
| **Coupling metrics** (von Zitzewitz, Martin), `analyze`      | How tangled is the graph, and how far does a change spread?                                                                         | CCD, ACD, propagation cost, relative cyclicity and SDI; Martin's Ca, Ce, instability, abstractness and distance per package                  | Measured                                              | Relative cyclicity, SDI and cycle counts are ratcheted; Martin's metrics are reported only |
| **ArchUnit-style fitness**, `fitness`                        | Does the code obey the fitness rules, and how does the graph compare with a balanced tree?                                          | Pass, fail, empty or off per rule; Lakos metrics with NCCD, Dowalil's visibility, ArchUnit's component dependency metrics; elementary cycles | Declared rules, and measured metrics                  | When the model declares fitness rules                                                      |
| **Connascence** (Page-Jones, connascence.io), `connascence`  | What knowledge do two places share, and how costly is it to change?                                                                 | Instances with a kind, strength, degree and locality                                                                                         | Facts, and candidates a reviewer confirms             | When the model sets connascence limits                                                     |
| **Balanced Coupling** (Khononov), `balance`                  | Is a coupling between two subsystems acceptable, given their distance and the upstream's volatility?                                | A level, distance, volatility, quadrant and score per subsystem pair                                                                         | Measured strength and distance, and judged volatility | When the model sets `balance.gate`                                                         |
| **Domain models** (modelith, DDD), `fitness` `domains` rules | Does the code still express the domain's vocabulary?                                                                                | Conformance findings, such as `domain-entity-missing` and `domain-enum-drift`                                                                | Declared vocabulary, checked against Go types         | As fitness rules                                                                           |
| **Judgement slots**, every measure                           | What can no computation decide?                                                                                                     | A question with evidence, answered by a verdict a human attests                                                                              | Judgement, bound to an evidence hash                  | Never by itself                                                                            |

The MMI does not penalise a dependency between subsystems as such. It
penalises one that bypasses the upstream's interface (criterion 1.2.1), one
against the layer or subsystem order (2.1.1, 2.1.2), and cycles (2.2.1–2.2.4;
`internal/arch/criteria.go`).

## How They Cover Each Other's Blind Spots

- **The MMI and the coupling metrics say that code is entangled, not whether
  it matters.** An interface bypass costs MMI points and gates as an error,
  whether it crosses a team boundary or stays inside one team's binary.
  Balanced Coupling adds the missing context: strong coupling over a short
  distance is cohesion, and strong coupling to a supporting or generic
  subdomain is balanced by that subdomain's low volatility.
- **Balanced Coupling needs to know how strong a coupling is, and
  connascence tells it.** Connascence of algorithm (clones included) and the
  dynamic kinds (execution, timing, value and identity) count as functional
  coupling. Name, type, meaning and position only grade the contract or model
  level that references establish, and contranascence is left out
  (`internal/balance/level.go`). Imports past an interface, `go:linkname` and
  `unsafe.Pointer` conversions supply the intrusive level. Connascence also
  says how to rebalance: weaken the kind, for example by naming a shared
  magic value.
- **Metrics describe the code as it is; the model and the fitness rules say
  what it should be.** The model records intent (subsystems, interfaces,
  layers, rules), and each rule carries a reason. A rule whose selectors
  match nothing is an `empty-selector` finding unless it is optional
  (ADR 0004), so the declared architecture cannot quietly stop applying. A
  rule without a reason is a `rule-without-reason` warning.
- **NCCD makes coupling comparable across sizes.** CCD and ACD grow with the
  codebase. Lakos's NCCD divides CCD by that of a balanced binary tree of the
  same size, so it ratchets fairly as a module grows (`internal/fitness/metrics.go`).
- **Structure metrics cannot see meaning, and domain models fill that gap.**
  A package graph can be acyclic while the code calls the model's `Proof` a
  `Packet`. Conformance against a modelith model finds that drift: skillet's
  model has four drifts found by hand and more found mechanically
  (`mmi-plan.md` §10.3). The domain model also informs Balanced Coupling:
  an entity's type counts as a model type without a `contract-type` slot,
  and the model adds evidence to the `bounded-context` and
  `integration-pattern` slots, never an answer (ADR 0005).
- **Every framework reaches a question no computation answers, and the
  judgement slots hold it.** Examples: the MMI's five reviewer criteria;
  whether a candidate is really connascence; a subsystem's subdomain, which
  sets its volatility. A slot's hash covers its question (ADR 0007). A judge
  sees no consequence of its answer (ADR 0008), may answer
  `insufficient-evidence` (ADR 0009), and cannot apply a verdict without a
  human event. Every finding a verdict suppresses is listed with who attested
  it (ADR 0010).
- **The gate compares their findings and metrics with one baseline.** `check`
  always gates the MMI's findings and ratcheted metrics. It adds the fitness
  rules when the model declares them, and connascence and Balanced Coupling
  only when the model opts in (`cmd/root/gate.go`); otherwise their findings
  are warnings for review. A conscious exception is a waiver with a reason,
  an approver and an expiry date, kept outside the model so it does not
  change the model's hash.

## How They Feed Each Other in the Code

- **The model is shared.** Every measure reads the same subsystems, layers
  and roles from `anarch.json`, and `arch.BuildGraphs` builds one type,
  package and subsystem graph for all of them.
- **Connascence → Balanced Coupling.** `cmd` passes every connascence
  instance and candidate to `balance`, which turns those spanning
  subsystems into strength evidence.
- **Ownership and deployment → distance.** CODEOWNERS, or a subsystem's
  `owner`, gives the owner boundary; the binaries that contain each package
  give the deploy boundary; and approved asynchronous integration, through
  the model's `messaging` clients, adds distance.
- **Churn → volatility evidence.** `balance --since-rev` counts each
  subsystem's commits as evidence for its `subdomain` slot, never as the
  volatility itself. `check` passes no revision range, so churn never gates.
- **Domain models → fitness and balance.** Conformance runs as a fitness
  rule. The same entity matching (`arch.DomainTypes`) tells `balance` which
  types are model types.
- **Everything → `render`.** `anarch.md` renders the model, the measured
  subsystem dependencies and the domain models for people and coding agents,
  and `render --check` fails when the rendering drifts.

## Where They Disagree, and What `anarch-lint` Does

- **An interface bypass between neighbours.** The MMI (1.2.1) and the
  `interface-bypass` finding treat every bypass alike, as an error. Balanced
  Coupling classifies the same import as intrusive and, at a short distance,
  places it in the high-cohesion quadrant with no finding. anarch-lint keeps
  both: no Balanced Coupling verdict suppresses an `analyze` finding. The
  remedy is a model change, such as merging the two subsystems or widening
  the interface, or a waiver.
- **Strong coupling within a module.** connascence.io holds that stronger
  connascence is acceptable within a module, and Balanced Coupling calls
  strong, close coupling high cohesion. The two agree, and anarch-lint
  ratchets connascence degree only for meaning and stronger at a subsystem
  boundary or beyond (ADR 0002).
- **The same words for different things.** Martin's *instability*
  (Ce ÷ (Ca + Ce)) is structural: how freely a package can change without
  breaking importers. Khononov's *volatility* is how often the business need
  behind it changes. Martin's *distance* is from the main sequence;
  Khononov's is how far apart two components are. Reports keep them under
  separate names, and only volatility enters a balance score.
- **Overlapping metrics with different definitions.** `analyze`'s Martin
  metrics count all named types toward abstractness and give an isolated
  package instability 0. ArchUnit's component dependency metrics count
  exported types only and give it 1. Cycles are counted three ways: the
  MMI's cycle groups, von Zitzewitz's relative cyclicity, and the fitness
  report's elementary cycles. Each definition is recorded among its report's
  assumptions, and none is merged into another.
- **One number or several.** The MMI folds its criteria into one weighted
  score; Balanced Coupling reports a score per pair but never gates on it,
  because its formula is archfit's and unchecked against the book. Nothing
  combines the frameworks into a single number.

## What None of Them Covers

These are the notable gaps. Several are deferred in `mmi-plan.md` §1, §9 or
§10.

- **Temporal coupling.** With `--since-rev`, `balance` lists the subsystems
  that change in the same commits and the hotspots (commits × complexity),
  and two subsystems that co-change without importing each other are a
  `hidden-coupling` candidate; a `coChange` fitness rule gates files that must
  change together. Co-change across files within one subsystem, and authors,
  are not measured, and churn is otherwise read only as evidence of
  volatility.
- **Run-time and dynamic coupling.** Dependencies resolved at run time
  through interfaces are seen only through injected interface parameters;
  a VTA call graph is deferred. Dynamic connascence (execution, timing,
  value, identity) is found only as candidates from static shapes.
- **Coupling outside the Go code.** Shared database schemas, wire contracts
  (HTTP, gRPC, events) and services in other repositories are invisible.
  anarch-lint infers asynchrony only from imports of declared messaging clients,
  and a deploy boundary only from the binaries in the analysed module.
- **Test code.** Test files are not loaded, so tests that reach into
  another subsystem's internals, and test-only cycles, go unmeasured.
- **Generated code.** Generated files, and files build constraints leave
  out, are listed but left out of every measure, so a cycle through
  generated code goes unseen.
- **Team structure beyond ownership.** Ownership is a CODEOWNERS boundary
  for distance. Team cognitive load, communication paths and other
  Team Topologies concerns are not modelled.
- **Quality attributes.** Performance, security and availability
  trade-offs, as ATAM weighs them, are out of scope.
- **The rest of a domain model.** Conformance checks entities, enums and
  relationships. Attributes against struct fields, ownership against
  containment, and invariants are not checked, and an `iota` enum's
  constants rarely match its value names.
- **Diagrams.** No C4, PlantUML or D2 import keeps a drawn architecture and
  `anarch.json` in step; `render` only goes from the model to Markdown.
- **An automated judge.** Every judgement slot waits for a human: the judge,
  its canaries, gold set and calibration are `mmi-plan.md` §9 steps 7–11.
  Until then, criteria that need judgement leave the MMI interval wide.

## Corrections from Verification

An earlier, unverified version of this comparison said two things the code
contradicts:

- **"The MMI penalises cross-subsystem dependencies uniformly."** It
  penalises interface bypasses, order violations and cycles, not
  dependencies as such. The real disagreement with Balanced Coupling is the
  interface bypass between neighbours, described above.
- **"Every framework's output becomes findings or metrics in one
  baseline."** Only the MMI's findings and metrics always gate. Fitness
  gates when rules are declared, and connascence and Balanced Coupling only
  when the model opts in.
