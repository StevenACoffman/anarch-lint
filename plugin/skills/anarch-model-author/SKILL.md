---
name: anarch-model-author
description: >
  Build or update an anarch-lint architecture model (anarch.json) by
  conversation. Use when the user wants to declare or change subsystems,
  interfaces, layers, forbidden imports, isolations, fitness rules or
  connascence limits for a Go module, or asks to "set up anarch-lint". Drafts
  the model in passes, asks one question at a time, and runs anarch-lint
  after every pass. To read the declared architecture before coding, use
  anarch-model-context instead.
---

# Authoring an Architecture Model

`anarch.json` declares the target architecture of a Go module: how packages
group into subsystems, which packages each subsystem exposes, the layer order,
and the rules every change must keep. anarch-lint measures the code against it
and gates on regressions. The model is the output of a conversation; your job
is to run that conversation and produce a model that loads and lints.

## Before You Write Anything

1. **Check that anarch-lint is installed:** `command -v anarch-lint`. If it is
   missing, stop and tell the user to run
   `go install github.com/StevenACoffman/anarch-lint@latest`. Never write a
   model you cannot lint: the tool is what tells you the model is valid and
   that its patterns match real packages.
2. **Read the schema:** `anarch-lint schema` prints the JSON Schema of the
   model, generated from the types anarch-lint reads. Put its `$id` in the
   model's `$schema` so editors complete and check it.
3. **Read what exists:** the current `anarch.json`, its rendering `anarch.md`,
   and any `*.modelith.yaml` domain models. New entries must agree with them.
4. **Look facts up; do not ask for them.** `go list ./...` gives the packages,
   `anarch-lint analyze` the measured dependencies, cycles and unassigned
   packages, and CODEOWNERS the owners. Ask the user only for what the code
   cannot say: intent.

## Build Order: Skeleton First

Work in passes across the whole model, each one a layer you could stop at.

1. **Subsystems and interfaces.** Group every package into a subsystem, each
   named for what it does, and give each subsystem the packages others may
   import. Run `anarch-lint analyze` and read the conformance and ordering
   sections: every package should be assigned, and every cross-subsystem
   import should go through an interface. This pass is the minimum useful
   model.
2. **Layers, forbidden imports and isolations.** Order the layers from the top
   (the layer that may import every other) down. Declare forbidden imports and
   isolations only for the boundaries that matter, each with a reason. Run
   `anarch-lint analyze` again.
3. **Fitness and connascence limits.** Add fitness rules (layer access, an
   onion, slices, dependents, calls, interface-only use, domain-model
   conformance) and connascence limits. Every rule takes a `reason`: a rule
   without one is a `rule-without-reason` warning, since nobody can tell
   whether to keep it. Run `anarch-lint fitness` and fix every
   `empty-selector` finding: a rule that matches nothing is usually a typo.

## The Conversation

- **Ask one question at a time, with your recommended answer and why.** "Should
  `internal/text` be its own subsystem? I recommend yes: four commands import
  it and it imports only the domain." A recommendation is cheaper to accept or
  reject than an open question.
- **A boundary is a decision.** When the user is unsure whether two packages
  belong together, ask what changes together and who changes it. Record the
  answer in the reason.
- **Do not legislate the current state.** A rule that only restates today's
  imports protects nothing. Ask what must stay true as the code grows.

## After Every Pass

1. `anarch-lint analyze` (or `fitness`) loads the model and rejects one it
   cannot read; fix what it reports before going on.
2. `anarch-lint render` regenerates `anarch.md`; commit it with the model.
   `anarch-lint render --check` fails when they drift.
3. Once the model is settled, `anarch-lint baseline` records the current
   findings and metrics, and `anarch-lint check` gates against them in CI.
   Refreshing the baseline accepts every current finding, so do it only when
   the user has reviewed what changed.
