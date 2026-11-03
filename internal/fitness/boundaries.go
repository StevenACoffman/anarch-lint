package fitness

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// crossing is one changed line's use of a package a boundary keeps out.
type crossing struct {
	site arch.Site
	pkg  string // the package used
	what string // the import, or the object used
}

// checkBoundaries checks each boundaries rule against the change set: a
// changed line of a file the rule names may not import, nor use an object
// of, a package it keeps out. Without a change set every rule is
// no-change. Matched counts the changed files the rule names.
func checkBoundaries(l *ledger, rules []arch.BoundaryRule, changed arch.ChangedLines,
	facts *arch.Facts, imports []arch.Import,
) {
	for i := range rules {
		b := &rules[i]
		r := l.addRule("boundaries", b.RuleOptions, arch.DefaultRuleName(b.Name, "boundaries", i))
		r.quiet = StatusPassed
		if changed == nil {
			r.quiet = StatusNoChange
			continue
		}
		for file := range changed {
			if matchesGlob(b.Files, file) {
				r.result.Matched++
			}
		}
		for _, c := range crossings(b, changed, facts, imports) {
			f := arch.NewFinding(arch.FindingID{
				Rule: arch.RuleBoundaryCrossed, Subject: c.site.File + ": " + c.what,
				Object: r.result.Name,
			}, arch.SeverityError)
			f.Location = &arch.Span{File: c.site.File, Start: c.site.Line, End: c.site.Line}
			f.Message = fmt.Sprintf("a changed line of %s uses %s, from %s, which %s keeps "+
				"changes to these files from", c.site.File, c.what, c.pkg, r.result.Name)
			f.Fix = "reach it through the package that replaces it, or leave this line as it was"
			l.violation(r, &f)
		}
	}
}

// crossings returns the changed lines of a rule's files that import or use
// the packages it keeps out, one per line and package, sorted by position.
func crossings(b *arch.BoundaryRule, changed arch.ChangedLines, facts *arch.Facts,
	imports []arch.Import,
) []crossing {
	var out []crossing
	add := func(site arch.Site, pkg, what string) {
		span := &arch.Span{File: site.File, Start: site.Line, End: site.Line}
		if arch.MatchesAny(b.Uses, pkg) && matchesGlob(b.Files, site.File) &&
			changed.Touches(span) {
			out = append(out, crossing{site: site, pkg: pkg, what: what})
		}
	}
	for _, imp := range imports {
		add(arch.Site{File: imp.Pos.File, Line: imp.Pos.Line, Package: imp.From}, imp.To,
			"the import "+imp.To)
	}
	for i := range facts.Uses {
		u := &facts.Uses[i]
		add(u.Site, u.Package, u.Object)
	}
	slices.SortFunc(out, func(a, b crossing) int {
		return cmp.Or(arch.CompareSites(a.site, b.site), cmp.Compare(a.pkg, b.pkg),
			cmp.Compare(a.what, b.what))
	})
	return slices.CompactFunc(out, func(a, b crossing) bool {
		return a.site.File == b.site.File && a.site.Line == b.site.Line && a.pkg == b.pkg
	})
}
