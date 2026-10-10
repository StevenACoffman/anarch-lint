// Package arch is the domain of anarch-lint: the facts a loader extracts from a
// Go program, the target-architecture model, and every measure, rule, judgement
// slot and gate computed from them.
//
// Everything here is a pure function of its inputs. Nothing reads the clock,
// the environment, the file system or a subprocess; the adapters in the other
// internal packages do that and hand arch plain values. Every slice the package
// returns is sorted on a total order, so the same inputs always produce the
// same output, byte for byte, once encoded.
//
// The measures follow "Measuring architectural complexity in Go with
// Lilienthal's model" (mmi.md): the dissertation's thirteen ordering measures,
// the Modularity Maturity Index criteria that tools can determine, and von
// Zitzewitz's coupling metrics. Criteria that need a reviewer are never
// scored here; they become judgement slots.
package arch
