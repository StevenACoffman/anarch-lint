package arch

import (
	"fmt"
	"slices"
)

// Conformance is how much of the program the model governs and how much of
// the model the program obeys: arch-go's coverage and compliance
// (mmi-ref-plan.md §4.4).
type Conformance struct {
	// Coverage is the share of packages, in percent, that a subsystem claims.
	Coverage float64 `json:"coverage"`
	// Rules counts the declared rules; Passing those with no error finding.
	Rules   int `json:"rules"`
	Passing int `json:"passing"`
	// Compliance is Passing ÷ Rules in percent, and 100 with no rules.
	Compliance float64 `json:"compliance"`
	// Failing names the rules with errors, sorted.
	Failing []string `json:"failing"`
	// MinCoverage and MinCompliance are the model's floors, 0 when unset.
	MinCoverage   float64 `json:"minCoverage,omitempty"`
	MinCompliance float64 `json:"minCompliance,omitempty"`
}

// Shortfall is an absolute floor the report falls below.
type Shortfall struct {
	Gate    string  `json:"gate"`
	Minimum float64 `json:"minimum"`
	Actual  float64 `json:"actual"`
}

// conformanceOf computes coverage and compliance from the assignment and the
// findings.
func conformanceOf(s *subject, findings []Finding) Conformance {
	c := Conformance{
		Coverage:      round6(percent(len(s.g.SubsystemAssignment.Of), len(s.p.Packages))),
		MinCoverage:   s.m.Gates.MinCoverage,
		MinCompliance: s.m.Gates.MinCompliance,
		Failing:       make([]string, 0),
	}
	failing := map[string]bool{}
	for i := range findings {
		if f := &findings[i]; f.ModelRule != "" && f.Severity == SeverityError {
			failing[f.ModelRule] = true
		}
	}
	for _, rule := range declaredRules(s.m) {
		c.Rules++
		if failing[rule] {
			c.Failing = append(c.Failing, rule)
			continue
		}
		c.Passing++
	}
	slices.Sort(c.Failing)
	c.Compliance = 100
	if c.Rules > 0 {
		c.Compliance = round6(percent(c.Passing, c.Rules))
	}
	return c
}

// declaredRules names every rule the model declares, in model order: one per
// ordering it declares, one for subsystem interfaces, and one per
// forbidden-import rule and isolation.
func declaredRules(m *Model) []string {
	var rules []string
	if len(m.Layers) >= 2 {
		rules = append(rules, "layers")
	}
	if len(m.SubsystemOrder) >= 2 {
		rules = append(rules, "subsystemOrder")
	}
	if len(m.Subsystems) > 0 {
		rules = append(rules, "interfaces")
	}
	if len(m.Roles) >= 2 {
		rules = append(rules, "roles")
	}
	for i := range m.ForbiddenImports {
		rules = append(rules, instanceName("forbiddenImports", i))
	}
	for i := range m.Isolations {
		rules = append(rules, instanceName("isolations", i))
	}
	return rules
}

// instanceName names the i-th rule of a list in the model.
func instanceName(list string, i int) string { return fmt.Sprintf("%s[%d]", list, i) }

// shortfalls returns the floors the conformance falls below.
func (c *Conformance) shortfalls() []Shortfall {
	out := make([]Shortfall, 0)
	if c.MinCoverage > 0 && c.Coverage < c.MinCoverage {
		out = append(
			out,
			Shortfall{Gate: "minCoverage", Minimum: c.MinCoverage, Actual: c.Coverage},
		)
	}
	if c.MinCompliance > 0 && c.Compliance < c.MinCompliance {
		out = append(
			out,
			Shortfall{Gate: "minCompliance", Minimum: c.MinCompliance, Actual: c.Compliance},
		)
	}
	return out
}
