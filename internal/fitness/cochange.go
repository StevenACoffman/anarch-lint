package fitness

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// ruleMissingCoChange names the finding of a coChange rule.
const ruleMissingCoChange = "missing-co-change"

// checkCoChange checks each coChange rule against the change set: when a
// changed file matches When and not Except, some changed file must match
// Require. Without a change set every such rule is no-change. Matched counts
// the changed files that set the rule off; a rule nothing set off passed.
func checkCoChange(l *ledger, rules []arch.CoChangeRule, changed arch.ChangedLines) {
	files := slices.Sorted(maps.Keys(changed))
	for i := range rules {
		cc := &rules[i]
		r := l.addRule("coChange", cc.RuleOptions, arch.DefaultRuleName(cc.Name, "coChange", i))
		r.quiet = StatusPassed
		if changed == nil {
			r.quiet = StatusNoChange
			continue
		}
		var triggers []string
		required := false
		for _, f := range files {
			if matchesGlob(cc.When, f) && !matchesGlob(cc.Except, f) {
				triggers = append(triggers, f)
			}
			required = required || matchesGlob(cc.Require, f)
		}
		r.result.Matched = len(triggers)
		if len(triggers) == 0 || required {
			continue
		}
		first := changed[triggers[0]][0]
		f := arch.NewFinding(
			arch.FindingID{Rule: ruleMissingCoChange, Subject: r.result.Name},
			arch.SeverityError,
		)
		f.Location = &arch.Span{File: triggers[0], Start: first.Start, End: first.Start}
		f.Message = fmt.Sprintf("the change touches %s but no file matching %s",
			strings.Join(triggers, ", "), strings.Join(cc.Require, ", "))
		f.Fix = "change the files the rule requires along with these, such as regenerated " +
			"code or the model that describes them"
		l.violation(r, &f)
	}
}

// matchesGlob reports whether any of the globs matches the file. The model's
// validation rejects a malformed glob, so a match cannot fail.
func matchesGlob(globs []string, file string) bool {
	return slices.ContainsFunc(globs, func(g string) bool {
		return doublestar.MatchUnvalidated(g, file)
	})
}
