package fitness

import (
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// checkCases runs each declared case: the import it names is added to the
// program, and the structural and fitness rules explain it, as explain
// does; a decision other than the expected one is a violation. A case
// naming a package not under analysis matches nothing.
//
// Requires: packages is sorted.
func checkCases(l *ledger, in *arch.Input, packages []string) {
	for i := range in.Model.Fitness.Cases {
		c := &in.Model.Fitness.Cases[i]
		r := l.addRule("cases", c.RuleOptions, arch.DefaultRuleName(c.Name, "cases", i))
		_, fromOK := slices.BinarySearch(packages, c.From)
		_, toOK := slices.BinarySearch(packages, c.Imports)
		if !fromOK || !toOK {
			continue
		}
		r.result.Matched = 1
		rulings := explainCase(in, c)
		if decided(c, rulings) {
			continue
		}
		f := arch.NewFinding(arch.FindingID{
			Rule: arch.RuleCaseContradicted, Subject: c.From + " → " + c.Imports,
			Object: r.result.Name,
		}, arch.SeverityError)
		f.Message = fmt.Sprintf("%s expects %s importing %s to be %s%s, and the rules decide "+
			"otherwise",
			r.result.Name, c.From, c.Imports, c.Expect, byText(c.By))
		for _, e := range rulings {
			f.Evidence = append(
				f.Evidence,
				fmt.Sprintf("%s %s it: %s", e.ModelRule, e.Outcome, e.Why),
			)
		}
		f.Fix = "restore the rule the case relies on, or change the case if the model's " +
			"intent changed"
		l.violation(r, &f)
	}
}

// explainCase returns every rule's decision about a case's import, on a
// copy of the program with the import added.
func explainCase(in *arch.Input, c *arch.RuleCase) []arch.RuleExplanation {
	withCase := in.WithImport(c.From, c.Imports, arch.Position{File: "case " + c.Name, Line: 1})
	withCase.Model.Fitness.Cases = nil
	e := arch.ExplainImport(&withCase, c.From, c.Imports)
	return slices.Concat(e.Rules, ExplainImport(&withCase, c.From, c.Imports))
}

// decided reports whether the rulings decide a case as it expects: a
// forbidden import is forbidden by its rule, or by any when it names none;
// an allowed import is forbidden by no rule, and allowed by its rule when it
// names one.
func decided(c *arch.RuleCase, rulings []arch.RuleExplanation) bool {
	by := func(outcome arch.Outcome) bool {
		return slices.ContainsFunc(rulings, func(e arch.RuleExplanation) bool {
			return e.Outcome == outcome && (c.By == "" || e.ModelRule == c.By)
		})
	}
	if c.Expect == arch.ExpectForbidden {
		return by(arch.OutcomeForbids)
	}
	forbidden := slices.ContainsFunc(rulings, func(e arch.RuleExplanation) bool {
		return e.Outcome == arch.OutcomeForbids
	})
	return !forbidden && (c.By == "" || by(arch.OutcomeAllows))
}

// byText names the rule a case expects to decide, " by NAME", or nothing.
func byText(by string) string {
	if by == "" {
		return ""
	}
	return " by " + by
}
