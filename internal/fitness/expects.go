package fitness

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// checkExpects reports each expects rule whose dependency the build lacks:
// no package matching its from imports one matching its to. It is one
// finding per rule, since an absence has no line of code to point at.
func checkExpects(l *ledger, rules []arch.ExpectRule, imports []arch.Import, packages []string) {
	for i := range rules {
		e := &rules[i]
		r := l.addRule("expects", e.RuleOptions, arch.DefaultRuleName(e.Name, "expects", i))
		r.result.Matched, r.readsCode = countMatching(packages, e.From), matching(e.From)
		if r.result.Matched == 0 || expected(e, imports) {
			continue
		}
		f := arch.NewFinding(
			arch.FindingID{Rule: arch.RuleExpectedDependency, Subject: r.result.Name},
			arch.SeverityError,
		)
		f.Message = fmt.Sprintf("no package matching %s imports one matching %s",
			strings.Join(e.From, ", "), strings.Join(e.To, ", "))
		f.Fix = "add the dependency the design expects, or correct the rule if the design changed"
		l.violation(r, &f)
	}
}

// expected reports whether an import of the build gives the dependency the
// rule expects.
func expected(e *arch.ExpectRule, imports []arch.Import) bool {
	for _, imp := range imports {
		if !imp.Test && !imp.Inactive && arch.MatchesAny(e.From, imp.From) &&
			arch.MatchesAny(e.To, imp.To) {
			return true
		}
	}
	return false
}
