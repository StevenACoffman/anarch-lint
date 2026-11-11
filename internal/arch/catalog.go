package arch

import (
	"cmp"
	"slices"
)

// The rules of the analyze report, one per architecture predicate (mmi.md
// §7.4).
const (
	RuleUnassignedPackage     = "unassigned-package"
	RuleAmbiguousPackage      = "ambiguous-package"
	RuleStalePattern          = "stale-pattern"
	RuleLayerOrder            = "layer-order-violation"
	RuleSubsystemOrder        = "subsystem-order-violation"
	RuleInterfaceBypass       = "interface-bypass"
	RuleSubsystemWithoutIface = "subsystem-without-interface"
	RuleForbiddenImport       = "forbidden-import"
	RuleIsolation             = "isolation-violation"
	RuleHiddenDependency      = "hidden-dependency"
	RuleRoleOrder             = "role-order-violation"
	RuleUnknownRole           = "unknown-role-directive"
	RuleSubsystemCycle        = "subsystem-cycle"
	RuleTypeCycleTooLarge     = "type-cycle-too-large"
	RuleLargeFunc             = "large-func"
	RuleComplexFunc           = "complex-func"
	RuleLargeType             = "large-type"
	RuleLargeFile             = "large-file"
	RuleDeepNesting           = "deep-nesting"
	RuleCognitive             = "cognitive-complexity"
)

// The rules of the fitness report.
const (
	RuleLayerAccess              = "layer-access"
	RuleStrictLayers             = "strict-layers"
	RuleOnion                    = "onion-ring"
	RuleSliceDependency          = "slice-dependency"
	RuleSliceCycle               = "slice-cycle"
	RuleDependents               = "dependent-restriction"
	RuleForbiddenCall            = "forbidden-call"
	RuleCallRestriction          = "call-restriction"
	RulePossibleForbiddenCall    = "possible-forbidden-call"
	RulePossibleCallRestriction  = "possible-call-restriction"
	RuleForbiddenFieldAccess     = "forbidden-field-access"
	RuleFieldRestriction         = "field-restriction"
	RuleUnusedExport             = "unused-export"
	RuleInterfacesOnly           = "interfaces-only"
	RuleEmptySelector            = "empty-selector"
	RuleWithoutReason            = "rule-without-reason"
	RuleOutsideArchitecture      = "package-outside-architecture"
	RuleUnapprovedLibrary        = "unapproved-library"
	RuleUnusedApproval           = "unused-approval"
	RuleDomainEntityMissing      = "domain-entity-missing"
	RuleDomainEntityAmbiguous    = "domain-entity-ambiguous"
	RuleDomainRelationship       = "domain-relationship-missing"
	RuleDomainEnumMissing        = "domain-enum-missing"
	RuleDomainEnumDrift          = "domain-enum-drift"
	RuleDomainTypeUnmapped       = "domain-type-unmapped"
	RuleDomainContractEntity     = "domain-contract-entity"
	RuleDomainEnumOrder          = "domain-enum-order"
	RuleDomainActionMissing      = "domain-action-missing"
	RuleDomainAttributeMissing   = "domain-attribute-missing"
	RuleDomainOwnership          = "domain-ownership-drift"
	RuleInvariantExposed         = "invariant-exposed"
	RuleInvariantUnenforced      = "invariant-unenforced"
	RuleInvariantUntested        = "invariant-untested"
	RuleGuardsUnclaimed          = "domain-guards-unclaimed"
	RuleRedundant                = "redundant-rule"
	RuleUnusableAllowance        = "unusable-allowance"
	RuleFanOut                   = "fan-out"
	RuleExpectedDependency       = "expected-dependency-missing"
	RuleOperationUnserved        = "openapi-operation-unserved"
	RuleRouteUndocumented        = "openapi-route-undocumented"
	RuleFieldUnresolved          = "graphql-field-unresolved"
	RuleOperationInvalid         = "graphql-operation-invalid"
	RuleServiceUnregistered      = "proto-service-unregistered"
	RuleServiceUndeclared        = "proto-service-undeclared"
	RuleConstraintInvalid        = "proto-constraint-invalid"
	RuleCaseContradicted         = "model-case-contradicted"
	RuleBoundaryCrossed          = "boundary-crossed"
	RuleUnknownEntity            = "unknown-entity-directive"
	RuleUnknownInvariant         = "unknown-invariant-directive"
	RuleMissingCoChange          = "missing-co-change"
	ruleTightCouplingFunctional  = "tight-coupling-functional"
	ruleTightCouplingIntrusive   = "tight-coupling-intrusive"
	RuleLeakedModel              = "leaked-model"
	RuleLowCohesion              = "low-cohesion"
	RuleIntrusiveReflection      = "intrusive-reflection"
	RuleOwnersDiffer             = "subsystem-owners-differ"
	RuleUpstreamExposure         = "strong-upstream-exposure"
	RuleUntestedDependency       = "untested-dependency"
	RuleUntestedCrossing         = "untested-crossing"
	RuleAccidentalVolatility     = "accidental-volatility"
	RuleFrozenCore               = "frozen-core"
	RuleHiddenCoupling           = "hidden-coupling"
	RuleUnstableInterface        = "unstable-interface"
	RuleCrossing                 = "crossing"
	RuleHotspot                  = "hotspot"
	ruleTightCouplingPrefix      = "tight-coupling-"
	connascenceRulePrefix        = "connascence-"
	reportAnalyze                = "analyze"
	reportFitness                = "fitness"
	reportConnascence            = "connascence"
	reportBalance                = "balance"
	summaryConnascenceOverLimits = "connascence of this kind, stronger or of higher degree than the model allows at its locality"
)

// RuleInfo describes one rule a finding or candidate can break.
type RuleInfo struct {
	// Name is the finding's rule.
	Name string `json:"name"`
	// Report names the command whose report has the rule: analyze, fitness,
	// connascence or balance.
	Report string `json:"report"`
	// Summary says in one line what a finding of the rule means.
	Summary string `json:"summary"`
	// GovernedBy names the model entry whose own gate governs the rule's
	// findings, such as "forbiddenImports" or "fitness"; empty when the
	// model's findingRules may.
	GovernedBy string `json:"governedBy,omitempty"`
}

// Rules returns every rule, sorted by name: the one table of rule names,
// which SARIF's rule descriptions and the model's validation read.
func Rules() []RuleInfo {
	var rules []RuleInfo
	for report, summaries := range map[string]map[string]string{
		reportAnalyze: analyzeRules(), reportFitness: fitnessRules(), reportBalance: balanceRules(),
	} {
		for name, summary := range summaries {
			rules = append(rules, RuleInfo{
				Name: name, Report: report, Summary: summary, GovernedBy: governorOf(name, report),
			})
		}
	}
	for s := StrengthContranascence; s <= StrengthIdentity; s++ {
		rules = append(rules, RuleInfo{
			Name: s.Rule(), Report: reportConnascence, Summary: summaryConnascenceOverLimits,
			GovernedBy: reportConnascence,
		})
	}
	slices.SortFunc(rules, func(a, b RuleInfo) int { return cmp.Compare(a.Name, b.Name) })
	return rules
}

// governorOf names the model entry whose own gate governs a rule's findings:
// the forbidden imports and isolations, each fitness rule, and the
// connascence and balance sections. The structural rules of analyze and the
// fitness report's findings about the model itself have no entry of their
// own, so findingRules governs them.
func governorOf(name, report string) string {
	switch name {
	case RuleForbiddenImport, RuleHiddenDependency:
		return "forbiddenImports"
	case RuleIsolation:
		return "isolations"
	case RuleEmptySelector, RuleWithoutReason, RuleRedundant, RuleUnusableAllowance,
		RuleStrictLayers, RuleOutsideArchitecture:
		return ""
	}
	if report == reportAnalyze {
		return ""
	}
	return report
}

// analyzeRules summarises the rules of the analyze report.
func analyzeRules() map[string]string {
	return map[string]string{
		RuleUnassignedPackage:     "a package matches no subsystem pattern",
		RuleAmbiguousPackage:      "a package matches two subsystems' patterns equally well",
		RuleStalePattern:          "a subsystem or layer pattern matches no package",
		RuleLayerOrder:            "a package imports a package of a higher layer",
		RuleSubsystemOrder:        "a subsystem imports one later in the subsystem order",
		RuleInterfaceBypass:       "a package imports another subsystem's non-interface package",
		RuleSubsystemWithoutIface: "a subsystem declares no interface packages",
		RuleForbiddenImport:       "a package imports, or reaches, a package a rule forbids",
		RuleIsolation:             "a package imports outside what its isolation allows",
		RuleHiddenDependency:      "a forbidden package's concrete type is passed in as an interface",
		RuleRoleOrder:             "a type of one role refers to a type of a later role",
		RuleUnknownRole:           "an //arch:role directive names an undeclared role",
		RuleSubsystemCycle:        "subsystems import each other in a cycle",
		RuleTypeCycleTooLarge:     "more types form a cycle than the threshold allows",
		RuleLargeFunc:             "a function is longer than the threshold",
		RuleComplexFunc:           "a function's cyclomatic complexity is above the threshold",
		RuleLargeType:             "a type and its methods are longer than the threshold",
		RuleLargeFile:             "a file is longer than the threshold",
		RuleDeepNesting:           "a function nests blocks deeper than the threshold",
		RuleCognitive:             "a function's cognitive complexity is above the threshold",
	}
}

// fitnessRules summarises the rules of the fitness report.
func fitnessRules() map[string]string {
	return map[string]string{
		RuleLayerAccess:             "an import a layerAccess rule does not allow",
		RuleStrictLayers:            "a layer imports one other than the layer directly below",
		RuleOnion:                   "a ring imports one outside it, or an adapter another adapter",
		RuleSliceDependency:         "a slice imports another slice its rule keeps apart",
		RuleSliceCycle:              "slices import each other in a cycle",
		RuleDependents:              "a package imports one a dependents rule reserves for others",
		RuleForbiddenCall:           "a function calls what a calls rule forbids",
		RuleCallRestriction:         "a function outside the allowed packages calls a restricted callee",
		RulePossibleForbiddenCall:   "a dynamic call may reach what a calls rule forbids, by the VTA call graph",
		RulePossibleCallRestriction: "a dynamic call outside the allowed packages may reach a restricted callee",
		RuleForbiddenFieldAccess:    "a function reads or writes a field a fields rule forbids",
		RuleFieldRestriction:        "a function outside the allowed packages accesses a restricted field",
		RuleInterfacesOnly:          "a concrete type is used where only interfaces are allowed",
		RuleEmptySelector:           "a rule's package selector matches nothing",
		RuleWithoutReason:           "a rule gives no reason",
		RuleOutsideArchitecture:     "a package belongs to no subsystem, though the model requires it",
		RuleUnapprovedLibrary:       "a package imports a library its rule does not approve",
		RuleUnusedApproval:          "a libraries rule approves a library nothing imports",
		RuleDomainEntityMissing:     "a domain model's entity has no type in the code",
		RuleDomainEntityAmbiguous:   "a domain model's entity matches several types",
		RuleDomainRelationship:      "two related entities' types never refer to each other",
		RuleDomainEnumMissing:       "a domain model's enum has no type in the code",
		RuleDomainEnumDrift:         "an enum's values differ from its constants",
		RuleDomainTypeUnmapped:      "a domains rule maps a name to a type the program lacks",
		RuleDomainContractEntity:    "an entity's type plays the contract role",
		RuleDomainEnumOrder:         "an enum's integer constants run in another order than its values",
		RuleDomainActionMissing:     "an entity's action is no method of its type and no function taking it",
		RuleDomainAttributeMissing:  "an entity's attribute is no field of its type",
		RuleDomainOwnership:         "an owned entity's type is not held by its owner alone",
		RuleInvariantExposed:        "an invariant governs a type whose exported fields code outside its package can change",
		RuleInvariantUnenforced:     "no //arch:invariant directive marks a function or test as enforcing an invariant",
		RuleInvariantUntested:       "an invariant's enforcers are functions no test runs, by a coverage profile",
		RuleGuardsUnclaimed:         "error guards in a domain rule's packages that no invariant of its model claims",
		RuleRedundant:               "a rule forbids only what other rules already forbid",
		RuleUnusableAllowance:       "a rule allows by name what other rules forbid entirely",
		RuleFanOut:                  "a package imports more packages than its fanOut rule allows",
		RuleExpectedDependency:      "no package an expects rule names imports what the rule expects it to",
		RuleOperationUnserved:       "an OpenAPI document declares an operation no serving package registers",
		RuleRouteUndocumented:       "a serving package registers a route its OpenAPI document does not declare",
		RuleFieldUnresolved:         "a GraphQL root field has no resolver method in a serving package",
		RuleOperationInvalid:        "a GraphQL operation document is invalid against its schema",
		RuleServiceUnregistered:     "a .proto file declares a gRPC service no serving package registers",
		RuleServiceUndeclared:       "a serving package registers a gRPC service its .proto files do not declare",
		RuleConstraintInvalid:       "a message's buf.validate constraints do not compile",
		RuleCaseContradicted:        "the rules decide a declared case's import otherwise than it expects",
		RuleBoundaryCrossed:         "a changed line uses a package its boundaries rule keeps the file from",
		RuleUnknownEntity:           "an //arch:entity directive names no entity of a domain model covering it",
		RuleUnknownInvariant:        "an //arch:invariant directive names no invariant of a domain model covering it",
		RuleMissingCoChange:         "a change lacks the change its coChange rule requires",
	}
}

// balanceRules summarises the rules of the balance report.
func balanceRules() map[string]string {
	return map[string]string{
		ruleTightCouplingFunctional: "a functional coupling across a high distance to a volatile upstream",
		ruleTightCouplingIntrusive:  "an intrusive coupling across a high distance to a volatile upstream",
		RuleLeakedModel:             "two bounded contexts share one's model",
		RuleLowCohesion:             "a type or package falls apart into unrelated groups",
		RuleIntrusiveReflection:     "reflection looks up an unexported name",
		RuleOwnersDiffer:            "a subsystem's packages have different owners",
		RuleUpstreamExposure:        "a widely imported package depends strongly on another subsystem",
		RuleUntestedDependency:      "a widely imported package's tests run little of it",
		RuleUntestedCrossing:        "a changed function calls into another subsystem, and no test runs it",
		RuleAccidentalVolatility:    "a subsystem judged generic or supporting changes often",
		RuleFrozenCore:              "a subsystem judged core never changed",
		RuleHiddenCoupling:          "two subsystems change together without importing each other",
		RuleUnstableInterface:       "a package many import changes in the same commits as its importers",
		RuleCrossing:                "a package with many importers and imports changes with both",
		RuleHotspot:                 "a package changes often and is complex, above the model's limit",
	}
}

// GovernableRules returns the rules the model's findingRules may name: those
// with no model entry of their own, sorted.
func GovernableRules() []RuleName {
	var out []RuleName
	for _, r := range Rules() {
		if r.GovernedBy == "" {
			out = append(out, RuleName(r.Name))
		}
	}
	return out
}

// RuleNamed returns the rule of that name, and false when there is none.
func RuleNamed(name string) (RuleInfo, bool) {
	rules := Rules()
	i, ok := slices.BinarySearchFunc(rules, name, func(r RuleInfo, n string) int {
		return cmp.Compare(r.Name, n)
	})
	if !ok {
		return RuleInfo{}, false
	}
	return rules[i], true
}

// Rule returns the rule of a connascence finding of this kind.
func (s Strength) Rule() string { return connascenceRulePrefix + s.String() }

// TightCouplingRule returns the rule of a tight-coupling finding at the named
// integration level.
func TightCouplingRule(level string) string { return ruleTightCouplingPrefix + level }
