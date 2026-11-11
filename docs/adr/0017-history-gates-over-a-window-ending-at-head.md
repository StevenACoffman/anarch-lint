# History Gates over a Window Ending at HEAD

With `balance.history: {commits: N}` in the model, `check`, `baseline` and
`balance` read the change history of the last N commits of HEAD's
first-parent line, and a package whose hotspot score, commits in that
window times summed cyclomatic complexity, is above `maxHotspot` is a
`hotspot` finding. It gates when `balance.gate` does.

Pinned by `TestADR_0017_HistoryGatesOverAWindowEndingAtHead` in
`internal/balance`.

## Context

`balance --since-rev` reports co-change, hotspots and hidden-coupling
candidates, and `TODO.md` asked for a gate on them, with the window pinned
to a commit rather than to the clock (`mmi-ref-plan.md` §4.8). A window that
starts at a fixed commit is pinned, but it only grows: every commit adds to
some package's count, so the measures worsen without any change to the
code's design, and someone has to keep moving the pin.

## Decision

**The window is the last N first-parent commits, ending at HEAD.** It is
pinned to commits, so two runs at one commit agree, and it moves with HEAD,
so a package's score falls as its old changes leave the window. A history
of N commits or fewer is read whole. A shallow clone cannot tell how long
its history is, so one that does not reach `HEAD~N` is an error naming the
fetch it needs, never a silently shorter window.

**A hotspot gates as a finding, never as a metric.** Its fingerprint is the
package, so a package that stays above the limit is in the baseline once
and a new one fails the gate. It rests on every function of the package
with complexity, so under `check --new-from-rev` a change to any of them
scopes it (ADR 0011). Hidden-coupling candidates gate only once a reviewer
confirms them, like every other candidate.

## Consequences

`check` needs git and enough history wherever the model sets a window; CI
checks out with `fetch-depth: 0`. Shortening the window, raising
`maxHotspot` or removing the window loosens the bars, so `check
--new-from-rev` refuses it alongside a code change. The self-check sets no
window: it would make `just check` depend on the clone's depth, and the
hotspot limit has no measured value to start from yet.
