---
name: anarch-model-context
description: >
  Load a Go module's declared architecture into the working context before a
  coding task. Use at the start of a session in a repository with an
  anarch.json, when the user says "follow the architecture" or "load the
  model", or before adding a package, an import between subsystems, or a
  dependency. Makes the agent place code in the right subsystem and keep the
  declared rules.
---

# Loading the Architecture as Context

`anarch.json` declares the architecture a Go module should keep, and
`anarch-lint check` fails a change that breaks it. Reading the model before
writing code means you place new code in the right subsystem, import only
through declared interfaces, and do not add a regression that check will
reject.

## What to Load

1. **Prefer the rendering, `anarch.md`, beside `anarch.json`.** It has
   the subsystems with their interfaces, the layer order, a diagram of the
   measured dependencies, the forbidden imports and isolations, and the
   fitness rules with their reasons.
2. **Check it is current:** `anarch-lint render --check` exits 3 when it is
   stale; then run `anarch-lint render` to regenerate it. Without anarch-lint
   installed, read `anarch.json` directly and say the rendering may be stale.
3. **Load the domain models it lists,** the `*.modelith.yaml` files of the
   fitness `domains` rules: their entity and enum names are the names the code
   should use.
4. **Read it in full before writing code.** Note for the task at hand:
   - the subsystem the change belongs to, and that subsystem's interface;
   - the layers it may import, and the imports and calls the rules forbid;
   - the reason of every rule the change comes near.

## How to Apply It While Coding

- **Put new packages where a subsystem's patterns claim them.** A package no
  subsystem claims is a finding when `requireAssigned` is set.
- **Import another subsystem only through its interface packages.**
- **Treat a rule's reason as a constraint on the design,** not as a lint to
  silence. If the task seems to need breaking a rule, stop and say so: the
  model may be wrong, and changing it is the user's decision, made with the
  `anarch-model-author` skill.
- **Before finishing, run `anarch-lint check`.** A new error finding or a
  metric regression is a design problem to fix, not a baseline to refresh.
