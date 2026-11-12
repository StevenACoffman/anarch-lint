# Cohesion Is Measured at the Boundary

anarch-lint measures whether the things that belong together live together
with two measures, both taken where edges cross a module's boundary:

- **The dependency profile** (Bouwers, van Deursen and Visser, ICSM 2011):
  the size of the elements hidden inside their module, reached from outside
  it (inbound), reaching outside it (outbound), or both (transit).
- **The cluster factor** (Mitchell and Mancoridis's TurboMQ): for each
  module, CF = 2μ ÷ (2μ + Σ(εij + εji)), its internal edge weight against
  the weight of the edges crossing its boundary in either direction, and 0
  when it has no internal edge.

Only these two can gate, and only as ratchets against the baseline.
Relational cohesion, Newman's modularity Q and contextual package cohesion
are reports at most, and never gate.

Pinned by `TestADR_0021_CohesionIsMeasuredAtTheBoundary` in
`internal/arch`.

## Context

anarch-lint measures coupling closely, through connascence, Balanced
Coupling and von Zitzewitz's metrics, but measures cohesion only with LCOM4
inside a type and with candidates that never gate: packages whose files
share no names, communities, co-change. The literature offers several
deterministic measures for packages and subsystems. Fixing the graph and
the partition fixes each value. The randomness in software clustering comes
from searching for a partition, and the model declares its own.

Briand, Morasca and Basili (TSE 1996; CS-TR-3368 §3.4) state four
properties a cohesion measure must have: a value in `[0, Max]`, 0
when there are no intra-module relationships, adding one never lowers it,
and merging two modules with no relationships between them never raises it
above the larger of the two. Checked against them:

| Measure                                    | 1   | 2   | 3   | 4   | Why                                                                                                                                |
| ------------------------------------------ | --- | --- | --- | --- | ---------------------------------------------------------------------------------------------------------------------------------- |
| Relational cohesion, H = (R + 1) ÷ N       | ✗   | ✗   | ✓   | ✓   | With no internal edges it gives 1 ÷ N, and it grows without a fixed bound.                                                         |
| Bunch intra-connectivity, μ ÷ N²           | ✓   | ✓   | ✓   | ✓   | A density: a 50-type package needs 2,500 edges to score 1, so it punishes size.                                                    |
| Cluster factor                             | ✓   | ✓   | ✓   | ✓   | Merging unrelated modules gives the mediant of their factors.                                                                      |
| Modularity Q                               | ✗   | ✗   | —   | —   | It scores a whole partition, is negative without internal edges, and has a resolution limit (Fortunato and Barthélemy, PNAS 2007). |
| Contextual package cohesion (Ponisio 2006) | —   | —   | ✗   | ✗   | It measures whether clients use classes together, and Ponisio's thesis says it fails 3 and 4 by design.                            |

The dependency profile is not a cohesion measure in Briand's sense: it
profiles a whole system. anarch-lint takes it because it alone of these
measures has evidence that it predicts whether changes stay local: "with
more code encapsulated within the components of a system more of the
changes remain localized to a single component" (Bouwers and Visser, 2011),
followed by a study of 12 such metrics over 10 open-source systems (ICSME 2014).

Abreu and Goulão (CSMR 2001) found that real systems are not organised to
maximise cohesion and minimise coupling, so nothing justifies an absolute
threshold. Both the Bunch papers and Ponisio warn that omnipresent modules,
used by or using nearly everything, distort every one of these measures.

## Decision

**The dependency profile and the cluster factor are the measures.**
`DependencyProfileOf` and `ClusterFactorsOf` compute them over any graph
and any assignment of its elements to modules, so they serve packages and
subsystems alike. An element assigned to no module is outside every module:
an edge to it crosses the boundary of the module at the other end.

**They gate as ratchets, never against a threshold.** A baseline records
the shares of the profile and each module's factor. A change that lowers
the hidden share, or a module's factor, by more than a tolerance fails the
gate.

**The other measures never gate.** Relational cohesion fails two of
Briand's properties. Q measures a partition against a random graph, not a
module's cohesion. Contextual package cohesion answers a different
question, whether clients use a package's parts together, and stays a
candidate beside the `low-cohesion` ones.

## Consequences

Only the computation exists so far: `analyze` does not report the measures,
the baseline does not record them, and the gate does not read them. Wiring
them in needs three more decisions: which elements the profile counts
(types or files) and what weighs them (lines of code), how to set
omnipresent packages aside, and the tolerance a ratchet move must exceed.
`IMPROVEMENTS.md` §11 weighs each against the tools that implement these
papers. Setting a package aside means removing it and its edges, as Bunch
does, not leaving it unassigned: by the rule above, an unassigned package's
edges lower every factor they touch.
`validate --rev` correlates only Martin's metrics and complexity today. It
should include both measures before either gates, so that the codebase
under the gate tests the claim that they predict local change.
