package fitness

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// checkFields reports reads and writes of fields a rule forbids, or from
// outside the packages a rule allows. With writes, only a use that writes a
// field counts.
func checkFields(l *ledger, rules []arch.FieldRule, uses []arch.Use, packages []string) {
	for i := range rules {
		checkFieldRule(l, &rules[i], i, uses, packages)
	}
}

// checkFieldRule checks fields rule i, fr, against every use of a field.
func checkFieldRule(l *ledger, fr *arch.FieldRule, i int, uses []arch.Use, packages []string) {
	r := l.addRule("fields", fr.RuleOptions, arch.DefaultRuleName(fr.Name, "fields", i))
	kind := arch.RuleForbiddenFieldAccess
	r.readsCode = everyPackage
	if fr.Only {
		kind = arch.RuleFieldRestriction
	} else {
		r.result.Matched, r.readsCode = countMatching(packages, fr.From), matching(fr.From)
	}
	for j := range uses {
		u := &uses[j]
		if !fieldRuleCounts(fr, u) {
			continue
		}
		if fr.Only {
			r.result.Matched++
		}
		if arch.MatchesAny(fr.From, u.Site.Package) != fr.Only {
			l.violation(r, fieldFinding(kind, r.result.Name, fr.Writes, u))
		}
	}
}

// fieldRuleCounts reports whether a use is one a fields rule restricts: of a
// field it names, and a write when the rule counts only writes.
func fieldRuleCounts(fr *arch.FieldRule, u *arch.Use) bool {
	return u.Kind == arch.ObjectField && matchesField(fr.Fields, u) && (!fr.Writes || u.Writes > 0)
}

// fieldFinding is the finding of one function's accesses of a restricted
// field.
func fieldFinding(kind, rule string, writes bool, u *arch.Use) *arch.Finding {
	accessor := cmp.Or(u.Site.Func, u.Site.Type, u.Site.Package)
	f := arch.NewFinding(
		arch.FindingID{Rule: kind, Subject: accessor, Object: rule + ": " + u.Object},
		arch.SeverityError,
	)
	f.Location = arch.SpanAt(arch.Position{File: u.Site.File, Line: u.Site.Line})
	verb, n := "accesses", u.Count
	if writes {
		verb, n = "writes", u.Writes
	}
	f.Message = fmt.Sprintf("%s %s %s", accessor, verb, u.Object)
	if n > 1 {
		f.Message += fmt.Sprintf(" %d times", n)
	}
	f.Fix = "call a method of " + u.Owner + " that does what the access does, " +
		"so the field's type keeps its state to itself"
	return &f
}

// matchesField reports a field that one of the patterns names exactly, or
// whose type or package a "pkg.Type.*" or "pkg.*" pattern names.
func matchesField(patterns []string, u *arch.Use) bool {
	for _, p := range patterns {
		if p == u.Object {
			return true
		}
		if prefix, ok := strings.CutSuffix(
			p,
			".*",
		); ok &&
			(prefix == u.Owner || prefix == u.Package) {
			return true
		}
	}
	return false
}
