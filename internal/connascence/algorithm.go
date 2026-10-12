package connascence

import (
	"fmt"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// algorithmCallee reports a callee whose constant first argument configures
// an algorithm: a regular expression or a time layout.
func algorithmCallee(callee string) bool {
	_, pattern := patternCallees()[callee]
	_, layout := layoutCallees()[callee]
	return pattern || layout
}

// patternCallees maps the regexp compilers to their pattern argument.
func patternCallees() map[string]int {
	return map[string]int{
		"regexp.MustCompile": 0, "regexp.Compile": 0,
		"regexp.MustCompilePOSIX": 0, "regexp.CompilePOSIX": 0,
	}
}

// layoutCallees maps the time functions to their layout argument.
func layoutCallees() map[string]int {
	return map[string]int{
		"time.Parse": 0, "time.ParseInLocation": 0,
		"(time.Time).Format": 0, "(time.Time).AppendFormat": 1,
	}
}

// samePatternDetector reports one regular expression compiled in two or more
// packages: the site's validation example (connascence.md §2.5, detector 1).
// An identical constant pattern rarely recurs by chance.
func samePatternDetector(s *subject) []arch.Connascence {
	return s.sameConfiguration(patternCallees(), "algorithm.same-pattern", "regular expression",
		"compile the pattern once, in one function both sides call")
}

// sameLayoutDetector reports one time layout used in two or more packages
// (connascence.md §2.5, detector 2).
func sameLayoutDetector(s *subject) []arch.Connascence {
	return s.sameConfiguration(layoutCallees(), "algorithm.same-layout", "time layout",
		"name the layout once, or format and parse through one function")
}

// sameConfiguration groups the literal configurations passed to callees by
// value and reports those used in two or more packages.
func (s *subject) sameConfiguration(
	callees map[string]int,
	detector, what, fix string,
) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.ConstExprs, func(c arch.ConstExpr) (string, bool) {
		arg, ok := callees[c.Callee]
		return c.Value, ok && c.Arg == arg && c.Literal
	}) {
		sites := make([]arch.Site, len(g))
		for i := range g {
			sites[i] = g[i].Site
		}
		if distinct(sites, sitePackage) < 2 {
			continue
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthAlgorithm,
			detector:   detector,
			subject:    g[0].Value,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceHigh,
			sites:      sites,
			message: fmt.Sprintf("the %s %s is written in %d packages",
				what, g[0].Value, distinct(sites, sitePackage)),
			fix: fix,
		}))
	}
	return out
}

// sameAlgorithmDetector reports one hash, checksum, encoding or encryption
// algorithm used in two or more packages (connascence.md §2.5, detector 3).
// That two packages use it is the evidence; which side writes and which
// reads is not told apart.
func sameAlgorithmDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.AlgoCalls, func(a arch.AlgoCall) (string, bool) {
		return a.Algorithm, true
	}) {
		sites := make([]arch.Site, len(g))
		for i := range g {
			sites[i] = g[i].Site
		}
		if distinct(sites, sitePackage) < 2 {
			continue
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthAlgorithm,
			detector:   "algorithm.same-algorithm",
			subject:    g[0].Algorithm,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf(
				"%d packages call %s",
				distinct(sites, sitePackage),
				g[0].Algorithm,
			),
			fix: "let one function own the algorithm and call it from both sides",
		}))
	}
	return out
}

// cloneDetector reports function bodies of the same shape in two or more
// packages: duplicated knowledge, connascence of algorithm and the extreme of
// functional coupling (connascence.md §2.5, detector 4; §9.2).
func cloneDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Funcs, func(f arch.Func) (string, bool) { return f.BodyHash, f.BodyHash != "" }) {
		sites := make([]arch.Site, len(g))
		for i := range g {
			sites[i] = arch.Site{
				File: g[i].Span.File, Line: g[i].Span.Start, Package: g[i].Package,
				Func: g[i].ID, Type: g[i].Receiver,
			}
		}
		if distinct(sites, sitePackage) < 2 {
			continue
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthAlgorithm,
			detector:   "algorithm.clone",
			subject:    g[0].ID,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf(
				"%d functions in %d packages have the same body shape as %s",
				len(g),
				distinct(sites, sitePackage),
				g[0].ID,
			),
			fix: "keep one copy and call it, or confirm that the copies may evolve apart",
		}))
	}
	return out
}
