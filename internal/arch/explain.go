package arch

import (
	"cmp"
	"slices"
)

// The outcomes of a rule for one import.
const (
	// OutcomeForbids: the rule's finding is new unless a baseline or waiver
	// accepts it.
	OutcomeForbids Outcome = "forbids"
	// OutcomeAllows: the rule examined the import and allows it, by name or
	// because it breaks nothing the rule states.
	OutcomeAllows Outcome = "allows"
	// OutcomeWaived: the rule forbids the import, and an unexpired waiver
	// accepts the finding.
	OutcomeWaived Outcome = "waived"
)

// Outcome is what one rule decides about one import.
type Outcome string

// ImportExplanation says which declared rules examined one import and what
// each decided (IMPROVEMENTS.md §2.1): it explains a pass as well as a
// finding.
type ImportExplanation struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Location is the import's first site; nil when From does not import To.
	Location *Span `json:"location,omitempty"`
	// Inactive marks an import read from a file the build excludes.
	Inactive bool `json:"inactive,omitempty"`
	// Hypothetical marks the decisions about an import From does not have:
	// what the rules would decide if it did.
	Hypothetical bool              `json:"hypothetical,omitempty"`
	Rules        []RuleExplanation `json:"rules"`
}

// RuleExplanation is one rule's decision about one import.
type RuleExplanation struct {
	// ModelRule names the rule as findings do, such as "layers" or
	// "fitness.layerAccess web".
	ModelRule string  `json:"modelRule"`
	Outcome   Outcome `json:"outcome"`
	Why       string  `json:"why"`
	// Severity and Fingerprint are the finding's, when the rule forbids
	// the import.
	Severity    Severity `json:"severity,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
}

// ExplainImport says which of the model's structural rules examine the import
// of to by from and what each decides: the layer and subsystem orders, the
// subsystem interfaces, the forbidden imports and the isolations. The rules
// run as in Analyze, on the program with that one import, so the
// explanation cannot drift from the check. A rule whose gate is off is left
// out. The fitness rules explain themselves (fitness.ExplainImport).
func ExplainImport(in *Input, from, to string) ImportExplanation {
	e := ImportExplanation{From: from, To: to, Rules: make([]RuleExplanation, 0)}
	p := in.Program
	only := func(imps []Import) []Import {
		var out []Import
		for _, imp := range imps {
			if imp.From == from && imp.To == to {
				out = append(out, imp)
			}
		}
		return out
	}
	p.Imports, p.InactiveImports = only(p.Imports), only(p.InactiveImports)
	p.TestImports = only(p.TestImports)
	imps := p.RuleImports()
	if len(imps) == 0 {
		return e
	}
	e.Location, e.Inactive = SpanAt(imps[0].Pos), imps[0].Inactive
	m := &in.Model
	g := BuildGraphs(&p, m)
	s := &subject{p: &p, m: m, g: &g}
	var findings, passes []Finding
	for _, r := range []ruleResult{
		layerRule(s), subsystemOrderRule(s), interfaceRule(s), forbiddenRule(s), isolationRule(s),
	} {
		findings = append(findings, r.findings...)
		passes = append(passes, r.passes...)
	}
	findings = m.Govern(findings)
	for i := range findings {
		if f := &findings[i]; f.ModelRule != "" && f.Location != nil {
			e.Rules = append(e.Rules, Forbids(f))
		}
	}
	passes = m.Govern(passes)
	for i := range passes {
		e.Rules = append(e.Rules, RuleExplanation{
			ModelRule: passes[i].ModelRule, Outcome: OutcomeAllows, Why: passes[i].Message,
		})
	}
	SortExplanations(e.Rules)
	return e
}

// WithImport returns a copy of the input whose program also imports to from
// from, at pos, for asking what the rules would decide about an import the
// code does not have. The input is not modified.
func (in *Input) WithImport(from, to string, pos Position) Input {
	out := *in
	out.Program.Imports = slices.Concat(
		in.Program.Imports,
		[]Import{{From: from, To: to, Pos: pos}},
	)
	return out
}

// Forbids returns the explanation of a finding that forbids an import.
func Forbids(f *Finding) RuleExplanation {
	return RuleExplanation{
		ModelRule: f.ModelRule, Outcome: OutcomeForbids, Why: f.Message,
		Severity: f.Severity, Fingerprint: f.Fingerprint,
	}
}

// SortExplanations orders explanations by rule, then outcome, then reason.
func SortExplanations(rules []RuleExplanation) {
	slices.SortFunc(rules, func(a, b RuleExplanation) int {
		return cmp.Or(
			cmp.Compare(a.ModelRule, b.ModelRule),
			cmp.Compare(a.Outcome, b.Outcome),
			cmp.Compare(a.Why, b.Why),
		)
	})
}

// Waive marks each forbidding rule whose finding an unexpired waiver accepts
// on today as waived, with the waiver's reason and approver.
func (e *ImportExplanation) Waive(waivers []Waiver, today Date) {
	for i := range e.Rules {
		r := &e.Rules[i]
		for _, w := range waivers {
			if r.Outcome == OutcomeForbids && w.Fingerprint == r.Fingerprint && w.Expires >= today {
				r.Outcome = OutcomeWaived
				r.Why += "; accepted until " + string(w.Expires) + " by " + w.ApprovedBy +
					": " + w.Reason
			}
		}
	}
}
