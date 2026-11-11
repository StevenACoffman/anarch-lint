package balance

import "github.com/StevenACoffman/anarch-lint/internal/arch"

// assumptions lists the choices behind the Balanced Coupling measures.
func assumptions() []arch.Assumption {
	return append(append(scaleAssumptions(), inputAssumptions()...),
		coChangeAssumption(), patternsAssumption(), coverageAssumption(), listsAssumption())
}

// listsAssumption records the caps on what the report lists.
func listsAssumption() arch.Assumption {
	return arch.Assumption{
		ID: "lists",
		Text: "A pair lists at most 20 pieces of evidence, with evidenceCount the total, and an " +
			"upstream-exposure finding at most 20 dependents, with the total in its message. The " +
			"report lists at most 20 hotspots and 20 packages of coverage, ranked; capped says " +
			"when a list was cut, and candidates are drawn from the whole list.",
		Source: "IMPROVEMENTS.md 1.5",
	}
}

// scaleAssumptions define the levels, the binary cuts and the numeric score.
func scaleAssumptions() []arch.Assumption {
	return []arch.Assumption{
		{
			ID: "integration-levels",
			Text: "Intrusive: an import of a package outside the upstream's declared interface, a " +
				"go:linkname into it, or an unsafe.Pointer conversion of its type. Functional: " +
				"connascence of algorithm, execution, timing, value or identity spanning the pair, " +
				"including clones. Model: a reference exposing an upstream type whose role is not " +
				"contract. Contract: every other reference. A subsystem without a declared interface " +
				"gives no intrusive evidence by import.",
			Source: "connascence.md §9.2",
		},
		{
			ID: "binary-cuts",
			Text: "High strength is functional or intrusive. High distance is an ordinal of 7 or more: " +
				"a module, owner or deploy boundary, or approved asynchronous integration. A subsystem " +
				"boundary within one team and one binary is medium distance, 4, as DDD calls a bounded " +
				"context.",
			Source: "connascence.md §9.5",
		},
		{
			ID: "ordinals",
			Text: "Strength ordinals contract 1, model 3, functional 8, duplicated code 9, intrusive 10; " +
				"volatility supporting or generic 3, core or unjudged 10; balance = max(|S−D|, 10−V)+1. " +
				"archfit's ordinals and formula, not checked against the book; the score is reported, " +
				"never gated.",
			Source: "connascence.md §9.5; archfit docs/guide/concepts.md",
		},
	}
}

// inputAssumptions say how volatility, owners, deploy units, integration
// patterns and exposure are read.
func inputAssumptions() []arch.Assumption {
	return []arch.Assumption{
		{
			ID: "volatility",
			Text: "Volatility is the upstream subsystem's subdomain from an approved verdict; an " +
				"unjudged subsystem counts as core. Churn is evidence for the verdict, and supports two " +
				"diagnostics: 20 or more commits on a supporting or generic subsystem, and none on a " +
				"core one.",
			Source: "connascence.md §9.4",
		},
		{
			ID: "owners",
			Text: "A subsystem's owner is the model's, or else the union of the owners CODEOWNERS gives " +
				"its packages' files, with GitHub's last-match rule. Git authorship is never an owner.",
			Source: "connascence.md §9.3",
		},
		{
			ID: "deploy-units",
			Text: "A binary is a main package in scope and everything it imports. Two packages no binary " +
				"holds together lie across a deploy boundary; only couplings without an import, such as " +
				"clones or shared values, can cross one.",
			Source: "connascence.md §9.3",
		},
		{
			ID: "integration-patterns",
			Text: "Shared kernel, partnership and conformist share a model on purpose; " +
				"customer-supplier comes from Learning Domain-Driven Design, not coupling.dev.",
			Source: "connascence.md §9.7",
		},
		{
			ID:     "exposure",
			Text:   "A package with 10 or more importers and an intrusive, or factual functional, upstream dependency is a finding.",
			Source: "connascence.md §9.6",
		},
		{
			ID: "slot-applicability",
			Text: "A subdomain slot opens for a subsystem another depends on, or for every subsystem " +
				"when a revision range gives churn; a bounded-context slot opens for either end of a " +
				"model-level pair. Each slot not opened is listed in skipped, with the reason.",
			Source: "mmi-plan.md §9.3 H",
		},
	}
}
