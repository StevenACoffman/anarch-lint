package fitness

import "github.com/StevenACoffman/anarch-lint/internal/arch"

// assumptions lists the choices behind the fitness measures.
func assumptions() []arch.Assumption {
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
		{
			ID: "cycles",
			Text: "Elementary cycles by Johnson's algorithm, starts and arcs in sorted order, at most " +
				"100 per graph and 20 imports per edge. Go forbids import cycles between packages, so " +
				"only the type and subsystem graphs have them.",
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
	}
}
