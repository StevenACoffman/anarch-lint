package fitness

import (
	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// ExplainImport says which fitness import rules examine the import of to by
// from and what each decides: a rule it violates forbids it, and a rule that
// names it among its allowances, such as a layer in mayOnlyAccess, allows
// it. A rule that neither forbids nor names an import does not say whether
// it examined it, so it is left out, as is a rule whose gate is off. The
// rules are compiled and run as Analyze runs them.
func ExplainImport(in *arch.Input, from, to string) []arch.RuleExplanation {
	s := newSubject(in)
	var imp *arch.Import
	for _, candidate := range s.p.RuleImports() {
		if candidate.From == from && candidate.To == to {
			imp = &candidate
			break
		}
	}
	out := make([]arch.RuleExplanation, 0)
	if imp == nil {
		return out
	}
	for _, c := range s.importConstraints() {
		if c.rule.off {
			continue
		}
		if f, ok := s.importFinding(&c, imp); ok {
			s.violation(c.rule, &f)
			out = append(out, arch.Forbids(&s.findings[len(s.findings)-1]))
			continue
		}
		for _, a := range c.allowances {
			if a.names(from, to) {
				out = append(out, arch.RuleExplanation{
					ModelRule: c.rule.modelRule(), Outcome: arch.OutcomeAllows,
					Why: "allowed by name: " + a.item,
				})
			}
		}
	}
	arch.SortExplanations(out)
	return out
}
