package fitness

import (
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// assumptions lists the choices behind the fitness measures: the metrics'
// definitions, then what the rules check.
func assumptions() []arch.Assumption {
	return slices.Concat(metricAssumptions(), ruleAssumptions())
}

// metricAssumptions lists how the metrics are defined.
func metricAssumptions() []arch.Assumption {
	return []arch.Assumption{
		{
			ID: "nccd",
			Text: "NCCD is CCD ÷ Σᵢ₌₁ⁿ (⌊log₂ i⌋ + 1), the CCD of a balanced binary tree of the same " +
				"size, as ArchUnit's LakosMetrics computes it; RACD is analyze's propagation cost.",
			Source: "TODO.md, Ideas from ArchUnit: Metrics",
		},
		{
			ID: "visibility",
			Text: "An element is a package-level type or a function that is not a method; it is " +
				"visible when exported. An internal package stays visible to its parent's tree, " +
				"which one module mostly is.",
			Source: "TODO.md, Ideas from ArchUnit: Metrics (Dowalil)",
		},
		{
			ID: "component-dependency",
			Text: "Unlike analyze's Martin metrics, abstractness counts exported types only and an " +
				"isolated package has instability 1, as ArchUnit's ComponentDependencyMetrics has it.",
			Source: "TODO.md, Ideas from ArchUnit: Metrics",
		},
	}
}

// ruleAssumptions lists what the rules check, and what they leave out.
func ruleAssumptions() []arch.Assumption {
	return []arch.Assumption{
		{
			ID: "verification",
			Text: "The import rules are compared over classes of packages every rule reads alike, " +
				"at most 250,000 cells (classes inside × all classes), beyond which the report says " +
				"they were not verified; isolations are left out, since what they allow depends on " +
				"each package's captures.",
			Source: "IMPROVEMENTS.md 1.1 (feelc's verifier)",
		},
		{
			ID: "cycles",
			Text: "Elementary cycles by Johnson's algorithm, starts and arcs in sorted order, at most " +
				"100 per graph and 20 imports per edge. A cycle set or slices rule that reaches the " +
				"first cap is marked truncated, and evidence past the second says how many it left " +
				"out; so does a fanOut finding's list of imports. Go forbids import cycles between " +
				"packages, so only the type and subsystem graphs have them.",
			Source: "TODO.md, Ideas from ArchUnit: Cycles",
		},
		{
			ID: "empty-rules",
			Text: "A fitness rule whose subject selectors match nothing is a finding unless optional, " +
				"as ArchUnit's failOnEmptyShould defaults to true; so is a forbidden import, isolation " +
				"or role whose patterns match nothing.",
			Source: "TODO.md, Ideas from ArchUnit: Rules That Match Nothing",
		},
		{
			ID: "layers",
			Text: "Layer access counts imports between two layered packages, and with consideringAll " +
				"also imports of in-scope packages without a layer. strictLayers reports imports that " +
				"skip a layer; imports upward are analyze's layer-order findings.",
			Source: "TODO.md, Ideas from ArchUnit: Layer Access",
		},
		{
			ID: "onion",
			Text: "A package belongs to the innermost onion ring that matches it, adapters tried in " +
				"name order; packages in no ring are not checked.",
			Source: "TODO.md, Ideas from ArchUnit: Layer Access",
		},
		{
			ID: "invariants",
			Text: "An invariant governs every field of its entities' types, since a model does not " +
				"say which fields it reads, and the type's own package may write them. An exported " +
				"field exposes it; a zero-value literal from another package, or a method returning " +
				"a slice or map the type holds, is not seen.",
			Source: "docs/adr/0016",
		},
	}
}
