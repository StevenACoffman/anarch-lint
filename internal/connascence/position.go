package connascence

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// unkeyedLiteralDetector reports composite literals of another package's
// struct type written without field names: every one depends on the field
// order (connascence.md §2.4, detector 1).
func unkeyedLiteralDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.UnkeyedLits, func(u arch.UnkeyedLit) (string, bool) {
		return u.Type, u.Package != u.Site.Package
	}) {
		typ := g[0].Type
		sites := make([]arch.Site, 0, len(g)+1)
		for _, u := range g {
			sites = append(sites, u.Site)
		}
		if d, ok := s.decl[typ]; ok {
			sites = append(sites, d)
		}
		out = append(out, s.instance(&instanceSpec{
			kind:     arch.StrengthPosition,
			detector: "position.unkeyed-literal",
			subject:  typ,
			object:   typ,
			basis:    arch.BasisFact,
			sites:    sites,
			message: fmt.Sprintf(
				"%s of %s %s its fields by position",
				count(len(g), "literal", "literals"),
				typ,
				verb(len(g), "lists", "list"),
			),
			fix: "name the fields in the literal, which replaces connascence of position with connascence of name",
		}))
	}
	return out
}

// longParamDetector reports exported functions with a long parameter list:
// every caller depends on the order (connascence.md §2.4, detector 2).
func longParamDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for i := range s.p.Facts.Signatures {
		sig := &s.p.Facts.Signatures[i]
		if !sig.Exported || len(sig.Params) < longParamList {
			continue
		}
		out = append(out, s.instance(&instanceSpec{
			kind:     arch.StrengthPosition,
			detector: "position.long-parameter-list",
			subject:  sig.Func,
			object:   sig.Func,
			basis:    arch.BasisFact,
			sites:    append([]arch.Site{sig.Site}, s.useSites(sig.Func)...),
			message:  fmt.Sprintf("%s takes %d parameters", sig.Func, len(sig.Params)),
			fix:      "pass an options struct with named fields, so callers agree on names instead of an order",
		}))
	}
	return out
}

// sameTypeResultsDetector reports exported functions returning several values
// of one type, not counting a final error: callers must know which is which
// (connascence.md §2.4, detector 3).
func sameTypeResultsDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for i := range s.p.Facts.Signatures {
		sig := &s.p.Facts.Signatures[i]
		results := sig.Results
		if n := len(results); n > 0 && results[n-1].Type == "error" {
			results = results[:n-1]
		}
		repeated := repeatedTypes(results)
		if !sig.Exported || len(repeated) == 0 {
			continue
		}
		out = append(out, s.instance(&instanceSpec{
			kind:     arch.StrengthPosition,
			detector: "position.same-type-results",
			subject:  sig.Func,
			object:   sig.Func,
			basis:    arch.BasisFact,
			sites:    append([]arch.Site{sig.Site}, s.useSites(sig.Func)...),
			message: fmt.Sprintf(
				"%s returns several results of type %s",
				sig.Func,
				strings.Join(repeated, ", "),
			),
			fix: "return a named struct, so callers select results by name",
		}))
	}
	return out
}

// repeatedTypes returns the types that occur more than once, sorted.
func repeatedTypes(refs []arch.TypeRef) []string {
	count := map[string]int{}
	for _, r := range refs {
		count[r.Type]++
	}
	var out []string
	for _, t := range slices.Sorted(maps.Keys(count)) {
		if count[t] > 1 {
			out = append(out, t)
		}
	}
	return out
}

// constIndexDetector reports constant indexes into a slice that a function
// of another package returns: a value used as a record whose fields are
// positions (connascence.md §2.4, detector 4).
func constIndexDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.ConstIndexes, func(c arch.ConstIndex) (string, bool) {
		return c.Origin, packageOfFullName(c.Origin) != c.Site.Package
	}) {
		sites := make([]arch.Site, len(g))
		indexes := map[string]bool{}
		for i, c := range g {
			sites[i] = c.Site
			indexes[c.Index] = true
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthPosition,
			detector:   "position.constant-index",
			subject:    g[0].Origin,
			object:     g[0].Origin,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf("the result of %s is read by position, at indexes %s",
				g[0].Origin, strings.Join(slices.Sorted(maps.Keys(indexes)), ", ")),
			fix: "turn the positions into a struct with named fields where the value is produced",
		}))
	}
	return out
}
