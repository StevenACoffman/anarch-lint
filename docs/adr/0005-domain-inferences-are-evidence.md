# Domain-Model Inferences Are Evidence, Never Answers

What anarch-lint infers from a modelith domain model is added to a judgement
slot's evidence and never answers the slot. A subsystem a domain model
describes is not thereby a bounded context, and a model that imports another
does not thereby name an integration pattern.

Pinned by `TestADR_0005_DomainInferencesAreEvidence` in `internal/balance`.

## Context

A domain model says what a subsystem is about, written by people who know.
It is tempting to read answers off it: a model per subsystem looks like one
bounded context each, and an import of a shared model looks like a shared
kernel. Neither follows. A team may write one model for two contexts, and a
model may import another only for its vocabulary while the code integrates
through a published contract (`mmi-plan.md` §10.0). Answering from the model
would turn a guess into a gate.

## Decision

**Inferences only add evidence lines.** The one deterministic use is that a
Go type matched to an entity is a model type, so balance asks no reviewer
whether it is a contract; that is a fact about the model, not a judgement.

## Consequences

Adding a domain model changes the evidence of the bounded-context and
integration-pattern slots, so verdicts on them go stale once and must be
judged again.
