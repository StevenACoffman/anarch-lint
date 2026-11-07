# Invariants Are Checked Through Their Enforcers

Whether the code keeps a domain model's invariant is settled in two parts.
The code settles what it can as findings: whether anything changes the
state an invariant governs outside the actions that preserve it, whether
someone has declared where the invariant is enforced, and whether tests run
that place. A reviewer is asked only what is left, whether the declared
enforcer expresses the statement, and only once the findings show how much
that is. An invariant is never evidence for another family's slots. This
refines ADR 0005 for invariants.

Pinned by `TestADR_0016_InvariantsAreNoOtherSlotsEvidence` in
`internal/balance`.

## Context

`DomainModel` read the ID of every invariant, and no rule compared them with
the code. An invariant is prose, and whether a program keeps even a formal
property is undecidable in general, so no rule can answer the whole
question. Three ways to ask a person were weighed first:

- **Evidence lines on existing slots**, as ADR 0005 does for the rest of a
  model. An invariant says how something behaves ("a `Project` always has an
  `Owner`"), which barely bears on whether a subsystem is a bounded context.
  Rewording one would also make every verdict on those slots stale
  (ADR 0007) for a reason unrelated to their question.
- **Advisory findings**, leads pairing an invariant with the rules that
  touch its entity by word overlap, as codetospec reports them. They never
  gate, and nothing records that a person looked, so the same leads return
  on every run.
- **A slot per invariant**, asking whether the code keeps it. The question
  covers the whole program, so its evidence is either fuzzy or every line
  that touches the entity, and reviewers would answer
  `insufficient-evidence` (ADR 0009).

Each asked a person the whole question. Most of it is not judgement: code
can show which functions change an entity's state, and a person who knows
where an invariant is enforced can say so once, in the code.

## Decision

**Settle what the code can.** For each invariant, the entities it governs
are the one that declares it and those its statement names in backticks,
resolved to types as the domain rules resolve them. Then:

- a write to a governed type's fields, or a composite literal of it, outside
  the type's own package, or an exported field of it, bypasses the
  actions that preserve the invariant: `invariant-exposed`;
- an `//arch:invariant ID` directive on a function or a test declares where
  the invariant is enforced; an invariant with none is
  `invariant-unenforced`, and a directive naming no invariant of a covering
  model is `unknown-invariant-directive`;
- with a coverage profile, an enforcer no test runs is
  `invariant-untested`.

**Ask a person only whether a declared enforcer expresses the statement,**
in a slot per invariant, `invariant/<id>`, whose evidence is the statement
and the enforcers. The slot waits until the findings above have run on real
models.

**Leave to the existing rules what they settle.** An entity with no type
is `domain-entity-missing` and an action with no method is
`domain-action-missing`; a name in backticks that resolves to nothing is
`modelith lint`'s warning.

## Consequences

The findings are facts about the domain rule, so they belong to `fitness`
beside the other domain findings, and `fitness` still asks no one anything.
Authors pay for the directive as they do for `//arch:entity`: the link from
prose to code is made once by someone who knows it, not guessed on every
run. Governing every field of an entity is coarse, since the model does not
say which fields an invariant reads; the same package may write them, so
constructors and decoders beside the type do not count as bypasses. When the
slot comes, an approved `not-expressed` verdict confirms a finding and
suppresses none, so ADR 0010 permits automatic judging to reach it.
