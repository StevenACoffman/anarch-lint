package connascence

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// repeatedLiteralDetector reports a meaningful literal compared with, cased
// on, or passed as an argument in two or more functions (connascence.md
// §2.3, detector 1). That two places use the literal is a fact; that they
// must agree on what it means is a claim, so each is a candidate.
func repeatedLiteralDetector(s *subject) []arch.Connascence {
	shadowed := s.constantsByValue()
	groups := grouped(s.p.Facts.ConstExprs, func(c arch.ConstExpr) (string, bool) {
		meaningful := c.Literal && !excludedValue(c.Value) && !exemptCallee(c.Callee) &&
			!algorithmCallee(c.Callee) && len(shadowed[c.Type+"\x00"+c.Value]) == 0
		return c.Type + "\x00" + c.Value, meaningful
	})
	var out []arch.Connascence
	for _, g := range groups {
		sites := make([]arch.Site, len(g))
		for i := range g {
			sites[i] = g[i].Site
		}
		if distinct(sites, siteEntity) < 2 {
			continue
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthMeaning,
			detector:   "meaning.repeated-literal",
			subject:    g[0].Value + " (" + g[0].Type + ")",
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf("the literal %s of type %s is used in %d functions",
				g[0].Value, g[0].Type, distinct(sites, siteEntity)),
			fix: "name the value with a constant, making it connascence of name, or give it a " +
				"domain type, making it connascence of type",
		}))
	}
	return out
}

// shadowedConstantDetector reports a literal that takes the value of a
// constant declared in the analysed code (connascence.md §2.3, detector 2).
// With a typed constant used as its type, the match is certain; with an
// untyped one, a literal 3 can match MaxRetries = 3 by coincidence.
func shadowedConstantDetector(s *subject) []arch.Connascence {
	byValue := s.constantsByValue()
	type match struct {
		constant arch.Constant
		typed    bool
		site     arch.Site
	}
	var matches []match
	for i := range s.p.Facts.ConstExprs {
		c := &s.p.Facts.ConstExprs[i]
		if !c.Literal {
			continue
		}
		for i := range byValue[c.Type+"\x00"+c.Value] {
			k := &byValue[c.Type+"\x00"+c.Value][i]
			typed := !strings.HasPrefix(k.Type, "untyped")
			if !typed && (excludedValue(c.Value) || exemptCallee(c.Callee)) {
				continue
			}
			matches = append(matches, match{constant: *k, typed: typed, site: c.Site})
			break
		}
	}
	var out []arch.Connascence
	for _, g := range grouped(matches, func(m match) (string, bool) { return m.constant.ID, true }) {
		k := g[0].constant
		sites := []arch.Site{k.Site}
		for i := range g {
			m := &g[i]
			sites = append(sites, m.site)
		}
		spec := instanceSpec{
			kind:     arch.StrengthMeaning,
			detector: "meaning.shadowed-constant",
			subject:  k.ID,
			object:   k.ID,
			basis:    arch.BasisFact,
			sites:    sites,
			message: fmt.Sprintf(
				"%s declares %s = %s; %s the literal instead",
				packageOf(
					k.ID,
				),
				lastName(k.ID),
				k.Value,
				count(len(g), "place writes", "places write"),
			),
			fix: fmt.Sprintf("use %s, so that connascence of meaning becomes connascence of name",
				path.Base(packageOf(k.ID))+"."+lastName(k.ID)),
		}
		if !g[0].typed {
			spec.basis, spec.confidence = arch.BasisCandidate, arch.ConfidenceLow
		}
		out = append(out, s.instance(&spec))
	}
	return out
}

// constantsByValue maps "type\x00value" to the in-scope constants that are
// not aliases. An untyped constant is filed under its default type, which is
// how a literal compared with it is typed.
func (s *subject) constantsByValue() map[string][]arch.Constant {
	out := map[string][]arch.Constant{}
	for i := range s.p.Facts.Constants {
		k := &s.p.Facts.Constants[i]
		if k.Alias {
			continue
		}
		typ := k.Type
		if def, ok := strings.CutPrefix(k.Type, "untyped "); ok {
			typ = map[string]string{"rune": "rune", "float": "float64", "complex": "complex128"}[def]
			if typ == "" {
				typ = def
			}
		}
		out[typ+"\x00"+k.Value] = append(out[typ+"\x00"+k.Value], *k)
	}
	return out
}

// primitiveParamDetector reports exported functions taking a bool, or runs of
// adjacent parameters of one basic type, which a caller can pass wrongly
// without the compiler noticing (connascence.md §2.3, detector 3).
func primitiveParamDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for i := range s.p.Facts.Signatures {
		sig := &s.p.Facts.Signatures[i]
		if !sig.Exported {
			continue
		}
		problem := primitiveProblem(sig.Params)
		if problem == "" {
			continue
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthMeaning,
			detector:   "meaning.primitive-parameters",
			subject:    sig.Func,
			object:     sig.Func,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      append([]arch.Site{sig.Site}, s.useSites(sig.Func)...),
			message:    fmt.Sprintf("%s takes %s", sig.Func, problem),
			fix: "replace a flag argument with two functions or a named option, and adjacent " +
				"values of one basic type with domain types or an options struct",
		}))
	}
	return out
}

// primitiveProblem describes a bool parameter or a run of basic parameters of
// one type, or returns "".
func primitiveProblem(params []arch.TypeRef) string {
	var problems []string
	if slices.ContainsFunc(params, func(p arch.TypeRef) bool { return p.Type == "bool" }) {
		problems = append(problems, "a bool parameter")
	}
	for i := 0; i < len(params); {
		j := i
		for j+1 < len(params) && params[j+1].Basic && params[j+1].Type == params[i].Type {
			j++
		}
		if params[i].Basic && j > i {
			problems = append(
				problems,
				fmt.Sprintf("%d adjacent %s parameters", j-i+1, params[i].Type),
			)
		}
		i = j + 1
	}
	return strings.Join(problems, " and ")
}

// nilNilDetector reports functions returning nil, nil: the nil value then
// means "not found" while a nil error means "success", and every caller must
// know both conventions (connascence.md §2.3, detector 4).
func nilNilDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.NilNils, func(n arch.NilNil) (string, bool) {
		return n.Site.Func, n.Site.Func != ""
	}) {
		fn := g[0].Site.Func
		sites := s.useSites(fn)
		for _, n := range g {
			sites = append(sites, n.Site)
		}
		out = append(out, s.instance(&instanceSpec{
			kind:     arch.StrengthMeaning,
			detector: "meaning.nil-nil",
			subject:  fn,
			object:   fn,
			basis:    arch.BasisFact,
			sites:    sites,
			message:  fmt.Sprintf("%s returns nil, nil on %s", fn, count(len(g), "path", "paths")),
			fix:      "return a not-found error instead of nil, nil, so a nil error always means a value",
		}))
	}
	return out
}

// useSites returns the sites of an object's uses, without its declaration.
func (s *subject) useSites(object string) []arch.Site {
	uses := s.usesOf[object]
	sites := make([]arch.Site, len(uses))
	for i := range uses {
		sites[i] = uses[i].Site
	}
	return sites
}

// packageOf returns the package part of a package-level object's ID.
func packageOf(id string) string { return id[:strings.LastIndex(id, ".")] }
