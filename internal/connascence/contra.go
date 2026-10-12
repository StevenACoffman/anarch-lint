package connascence

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Contranascence is the opposite of connascence: a property two elements
// must hold *different* (Page-Jones; connascence.md §3.5). It has no rank on
// the site's scale and is reported with strength 0.
const strengthContra arch.Strength = 0

// enumCollisionDetector reports two constants of one defined type with the
// same value, where neither is written as the other (connascence.md §3.5,
// detector 1).
func enumCollisionDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.Constants, func(k arch.Constant) (string, bool) {
		return k.Type + "\x00" + k.Value, !k.Alias && !strings.HasPrefix(k.Type, "untyped") && s.inScope(packageOf(k.Type))
	}) {
		if len(g) < 2 {
			continue
		}
		sites := make([]arch.Site, len(g))
		names := make([]string, len(g))
		for i := range g {
			k := &g[i]
			sites[i], names[i] = k.Site, lastName(k.ID)
		}
		out = append(out, s.instance(&instanceSpec{
			kind:     strengthContra,
			detector: "contranascence.enum-collision",
			subject:  g[0].Type + " = " + g[0].Value,
			object:   g[0].Type,
			basis:    arch.BasisFact,
			sites:    sites,
			message: fmt.Sprintf(
				"%s share the value %s of %s",
				strings.Join(names, ", "),
				g[0].Value,
				g[0].Type,
			),
			fix: "derive the values from one place, with iota, so that a collision cannot be written",
		}))
	}
	return out
}

// registrationCallees maps the registration functions to the argument that
// holds their key, and names the family whose keys must differ.
func registrationCallees() map[string]struct {
	arg    int
	family string
} {
	type reg = struct {
		arg    int
		family string
	}
	out := map[string]reg{
		"(*net/http.ServeMux).Handle": {
			0,
			"http route",
		},
		"(*net/http.ServeMux).HandleFunc": {0, "http route"},
		"net/http.Handle":                 {0, "http route"},
		"net/http.HandleFunc":             {0, "http route"},
	}
	for _, kind := range []string{"Bool", "Int", "Int64", "Uint", "Uint64", "String", "Float64", "Duration", "arch.Func", "Var"} {
		out["flag."+kind] = reg{0, "flag name"}
		out["(*flag.FlagSet)."+kind] = reg{0, "flag name"}
		if kind != "arch.Func" && kind != "Var" {
			out["flag."+kind+"Var"] = reg{1, "flag name"}
			out["(*flag.FlagSet)."+kind+"Var"] = reg{1, "flag name"}
		}
	}
	return out
}

// registrationDetector reports two registrations with the same constant key
// in one family. Two calls may register on different instances, so each is a
// candidate (connascence.md §3.5, detector 2).
func registrationDetector(s *subject) []arch.Connascence {
	regs := registrationCallees()
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.ConstExprs, func(c arch.ConstExpr) (string, bool) {
		r, ok := regs[c.Callee]
		return r.family + "\x00" + c.Value, ok && c.Arg == r.arg
	}) {
		if len(g) < 2 {
			continue
		}
		family := regs[g[0].Callee].family
		sites := make([]arch.Site, len(g))
		for i := range g {
			sites[i] = g[i].Site
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       strengthContra,
			detector:   "contranascence.registration",
			subject:    family + " " + g[0].Value,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message:    fmt.Sprintf("the %s %s is registered %d times", family, g[0].Value, len(g)),
			fix:        "register from one table, so that a key cannot be written twice",
		}))
	}
	return out
}
