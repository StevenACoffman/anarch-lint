# The Self-Check Keeps Three Layers

The self-check model draws three technical layers, `command`, `adapter`
and `domain`, the imperative shell, its adapters and the functional core.
The pure measures (`connascence`, `balance`, `fitness` and `judging`) stay
in `domain` beside `internal/arch`.

Pinned by `TestADR_0015_TheSelfCheckKeepsThreeLayers` in `main_test.go`.

## Context

MMI criterion 1.1.4 scores the size relationship of the technical layers:
the lines of the largest layer divided by those of the smallest, divided by the number
of layers. Nearly every feature adds to `domain`, so the score slips as the
core grows: 7 at a ratio of 1.90 (12,812 lines against `command`'s 2,252),
and 6 once the ratio passes 2.0, at about 13,500 lines.

Two other ways of drawing the layers would raise it:

- Splitting `domain` into `vocabulary` (`internal/arch`) and `measures`
  scores 10. The dependency between them is real, but `subsystemOrder`
  already enforces it, and most of the gain comes from dividing by four
  layers instead of three. "Measures" is a functional grouping, not a
  technical layer.
- Merging `command` and `adapter` into `shell` scores 9, but two layers are
  close to none, and the layer rule would no longer catch an adapter
  importing `cmd`.

## Decision

**Keep the three layers.** They describe the design truthfully, and a
model redrawn to raise a score would make the score describe the model
instead of the code.

## Consequences

Criterion 1.1.4 will fall as the core grows, and the baseline will accept
it; the drop says the core is larger than the shell, which is the design.
A change to the layers needs a new decision, and changes this test.
