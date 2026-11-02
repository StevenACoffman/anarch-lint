package arch

import (
	"cmp"
	"maps"
	"slices"
)

// The quality attributes a rule can serve: ISO 25010's characteristics, with
// the availability, scalability and compliance that spec-artifacts-iso lists
// apart, and safety, as codetospec's internal/quality enumerates them.
const (
	QualityFunctionalSuitability QualityAttribute = "functional_suitability"
	QualityPerformanceEfficiency QualityAttribute = "performance_efficiency"
	QualityCompatibility         QualityAttribute = "compatibility"
	QualityUsability             QualityAttribute = "usability"
	QualityReliability           QualityAttribute = "reliability"
	QualityAvailability          QualityAttribute = "availability"
	QualitySecurity              QualityAttribute = "security"
	QualityMaintainability       QualityAttribute = "maintainability"
	QualityPortability           QualityAttribute = "portability"
	QualityScalability           QualityAttribute = "scalability"
	QualityCompliance            QualityAttribute = "compliance"
	QualitySafety                QualityAttribute = "safety"
)

// RuleName is a finding's rule, as a key of the model's findingRules.
type RuleName string

// QualityAttribute names the quality a rule protects, so the model doubles as
// a list of the module's architectural nonfunctional requirements, classified
// by declaration rather than inferred.
type QualityAttribute string

// FindingRule governs the findings of one rule that no entry of the model
// declares, such as layer-order-violation or large-func: the model's
// findingRules map it by the finding's rule.
type FindingRule struct {
	// Gate overrides the rule's own severity; off drops its findings, and
	// empty keeps the severity the rule gives.
	Gate             RuleGate         `json:"gate,omitempty"`
	Reason           string           `json:"reason,omitempty"`
	QualityAttribute QualityAttribute `json:"qualityAttribute,omitempty"`
}

// ReasonedRule is one rule the model declares, with the reason it gives and
// the quality it serves.
type ReasonedRule struct {
	// Name is how findings name the rule in ModelRule, such as "layers",
	// "forbiddenImports[2]" or "fitness.calls no-clock".
	Name             string
	Reason           string
	QualityAttribute QualityAttribute
	// FindingRule is the findingRules key the reason comes from, when the
	// rule has no entry of its own to hold one.
	FindingRule RuleName
}

// governance is what a model says about the findings of one rule.
type governance struct {
	gate    RuleGate
	quality QualityAttribute
	// reason is appended to the message; empty when the rule's own message
	// already gives it, as a forbidden import's does.
	reason string
}

// QualityAttributes returns every quality attribute, in ISO 25010's order.
func QualityAttributes() []QualityAttribute {
	return []QualityAttribute{
		QualityFunctionalSuitability, QualityPerformanceEfficiency, QualityCompatibility,
		QualityUsability, QualityReliability, QualityAvailability, QualitySecurity,
		QualityMaintainability, QualityPortability, QualityScalability, QualityCompliance,
		QualitySafety,
	}
}

// UnmarshalText accepts only a known quality attribute.
func (q *QualityAttribute) UnmarshalText(text []byte) error {
	v := QualityAttribute(text)
	if !slices.Contains(QualityAttributes(), v) {
		return &Error{Code: EINVALID, Message: string(text) + " is not a quality attribute"}
	}
	*q = v
	return nil
}

// Govern applies the model's gates, reasons and quality attributes to
// findings: a forbidden import or isolation by its entry, any other rule by
// its findingRules entry. A finding whose gate is off is dropped. A new slice
// is returned; fs is not modified.
func (m *Model) Govern(fs []Finding) []Finding {
	governanceOf := m.governor()
	out := make([]Finding, 0, len(fs))
	for i := range fs {
		f := fs[i]
		g := governanceOf(&f)
		if g.gate != "" {
			sev, on := g.gate.Severity()
			if !on {
				continue
			}
			f.Severity = sev
		}
		f.QualityAttribute = cmp.Or(f.QualityAttribute, g.quality)
		if g.reason != "" {
			f.Message += ": " + g.reason
		}
		out = append(out, f)
	}
	return out
}

// governor returns what the model says about a finding's rule: a forbidden
// import or isolation's own rules are governed by its entry, every other
// rule by its findingRules entry. A redundant-rule finding about a forbidden
// import names it too, but is not one of its violations.
func (m *Model) governor() func(f *Finding) governance {
	byEntry := map[string]governance{}
	for i, fi := range m.ForbiddenImports {
		byEntry[instanceName("forbiddenImports", i)] = governance{
			gate: fi.Gate, quality: fi.QualityAttribute,
		}
	}
	for i, iso := range m.Isolations {
		byEntry[instanceName("isolations", i)] = governance{
			gate: iso.Gate, quality: iso.QualityAttribute,
		}
	}
	entryRules := map[string]bool{}
	for _, r := range Rules() {
		if r.GovernedBy == "forbiddenImports" || r.GovernedBy == "isolations" {
			entryRules[r.Name] = true
		}
	}
	return func(f *Finding) governance {
		if g, ok := byEntry[f.ModelRule]; ok && entryRules[f.Rule] {
			return g
		}
		fr := m.FindingRules[RuleName(f.Rule)]
		return governance{gate: fr.Gate, quality: fr.QualityAttribute, reason: fr.Reason}
	}
}

// ReasonedRules returns every rule the model declares, in model order: the
// orderings and interfaces it declares, under their findingRules entries; each
// forbidden import and isolation; each fitness rule with options; and each
// findingRules entry of a rule that no declaration above covers.
func (m *Model) ReasonedRules() []ReasonedRule {
	covered := map[RuleName]bool{}
	var out []ReasonedRule
	for _, d := range []struct {
		name     string
		rule     RuleName
		declared bool
	}{
		{"layers", RuleLayerOrder, len(m.Layers) >= 2},
		{"subsystemOrder", RuleSubsystemOrder, len(m.SubsystemOrder) >= 2},
		{"interfaces", RuleInterfaceBypass, len(m.Subsystems) > 0},
		{"roles", RuleRoleOrder, len(m.Roles) >= 2},
	} {
		if d.declared {
			fr := m.FindingRules[d.rule]
			covered[d.rule] = true
			out = append(out, ReasonedRule{d.name, fr.Reason, fr.QualityAttribute, d.rule})
		}
	}
	for i, fi := range m.ForbiddenImports {
		out = append(out, ReasonedRule{
			Name: instanceName("forbiddenImports", i), Reason: fi.Reason,
			QualityAttribute: fi.QualityAttribute,
		})
	}
	for i, iso := range m.Isolations {
		out = append(out, ReasonedRule{
			Name: instanceName("isolations", i), Reason: iso.Reason,
			QualityAttribute: iso.QualityAttribute,
		})
	}
	for _, r := range m.Fitness.Rules() {
		out = append(out, ReasonedRule{
			Name: r.ModelRule(), Reason: r.Reason, QualityAttribute: r.QualityAttribute,
		})
	}
	for _, rule := range slices.Sorted(maps.Keys(m.FindingRules)) {
		if !covered[rule] {
			fr := m.FindingRules[rule]
			out = append(out, ReasonedRule{
				Name: "findingRules." + string(rule), Reason: fr.Reason,
				QualityAttribute: fr.QualityAttribute, FindingRule: rule,
			})
		}
	}
	return out
}

// ModelRule names the rule as its findings' ModelRule: its kind under
// fitness, and its name when that differs.
func (d *DeclaredRule) ModelRule() string {
	if d.Name == d.Kind {
		return "fitness." + d.Kind
	}
	return "fitness." + d.Kind + " " + d.Name
}

// validateFindingRules rejects a findingRules entry for an unknown rule, for
// a rule whose own entry governs it, and an unknown gate.
func (m *Model) validateFindingRules(invalid invalidFunc) error {
	for _, rule := range slices.Sorted(maps.Keys(m.FindingRules)) {
		info, ok := RuleNamed(string(rule))
		switch {
		case !ok:
			return invalid(ProblemReference, "findingRules names unknown rule %q", rule)
		case info.GovernedBy != "":
			return invalid(ProblemReference,
				"findingRules names %q, which its %s entry governs", rule, info.GovernedBy)
		}
		switch m.FindingRules[rule].Gate {
		case "", GateFail, GateWarn, GateOff:
		default:
			return invalid(ProblemValue, "findingRules %q gate %q is not fail, warn or off",
				rule, m.FindingRules[rule].Gate)
		}
	}
	return nil
}
