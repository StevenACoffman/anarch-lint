# The Balance Score Never Gates

`balance` reports archfit's numeric balance for each pair of subsystems,
`max(|S−D|, 10−V) + 1`, and never gates on it, nor on any metric derived from
it. What gates is the binary model of `connascence.md` §9.5: the quadrant a
pair falls in, and the findings and candidates it yields.

Pinned by `TestADR_0014_TheBalanceScoreNeverGates` in `internal/balance`.

## Context

`TODO.md` asked to check archfit's ordinals and formula against the book's
model before deciding whether the score may gate. The book, _Balancing
Coupling in Software Design_, is not at hand. archfit
(`internal/relationship/scoring/scorer_book.go`, commit `2d20da8`) states its
formula and ordinals are "verbatim from Khononov Ch8–Ch10":

| Axis       | archfit                                                                      | anarch-lint                                                                                  |
| ---------- | ---------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Strength   | contract 1, model 3, functional 8, symmetric 9, intrusive 10                 | the same                                                                                     |
| Distance   | same module 2, same owner 4, other owner 7, other deploy unit 9, external 10 | package in one subsystem 2, subsystem 4, owner or async 7, other Go module 7, other deploy 9 |
| Volatility | frozen 1, low 3, medium 6, high 10, undeclared 10                            | supporting or generic 3, core or unjudged 10                                                 |

The two agree wherever anarch-lint has the distinction. anarch-lint has no
medium or frozen volatility, since a subdomain verdict gives only core,
supporting or generic, and it treats another Go module as an owner
boundary's equal, where archfit has no such rung.

## Decision

**Report the score; never gate on it.** Three reasons:

- The comparison is with archfit's reading of the book, not the book.
- The formula treats ordinal levels as an interval scale: |S−D| says model
  coupling at distance 4 is as balanced as functional coupling at 9, which
  the book's binary model, MODULARITY = S XOR D, does not claim.
- The binary model already gates, through the tight-coupling and leaked-model
  findings, and a second, numeric gate on the same pairs would double-count
  them.

## Consequences

A change that only moves a pair's score passes the gate. A reader who wants
to rank pairs can sort by the score, which the report keeps.
