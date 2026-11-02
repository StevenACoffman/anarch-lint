package connascence

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Dynamic connascence exists only when the code runs, so every detector here
// reports evidence of it: candidates, apart from the few facts the source
// shows directly (connascence.md §3).

// guardDetector reports a method that returns early on a receiver field that
// another method of the type sets: the signature of a required call order
// (connascence.md §3.1, detector 1).
func guardDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range s.p.Facts.Guards {
		sites := []arch.Site{g.Site}
		for _, w := range s.p.Facts.FieldWrites {
			if w.Type == g.Type && w.Field == g.Field && w.Method != g.Method {
				sites = append(sites, w.Site)
			}
		}
		if len(sites) == 1 {
			continue
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthExecution,
			detector:   "execution.guarded-field",
			subject:    g.Type + "." + g.Field,
			object:     g.Type,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf("%s refuses to run once %s.%s is set by another method",
				g.Method, lastName(g.Type), g.Field),
			fix: "return a ready value from a constructor, or a type whose methods are only " +
				"reachable in the valid order",
		}))
	}
	return out
}

// lifecycleDetector reports a type with both halves of a lifecycle pair:
// callers must call them in order (connascence.md §3.1, detector 2).
func lifecycleDetector(s *subject) []arch.Connascence {
	methods := map[string]map[string]arch.Func{}
	for i := range s.p.Funcs {
		f := &s.p.Funcs[i]
		if f.Receiver != "" && isExported(f.Name) {
			if methods[f.Receiver] == nil {
				methods[f.Receiver] = map[string]arch.Func{}
			}
			methods[f.Receiver][f.Name] = *f
		}
	}
	var out []arch.Connascence
	for _, typ := range slices.Sorted(maps.Keys(methods)) {
		for _, pair := range [][2]string{{"Open", "Close"}, {"Start", "Stop"}, {"Connect", "Disconnect"}, {"Begin", "Commit"}} {
			first, ok1 := methods[typ][pair[0]]
			second, ok2 := methods[typ][pair[1]]
			if !ok1 || !ok2 {
				continue
			}
			sites := slices.Concat(s.usesFrom(first.ID), s.usesFrom(second.ID))
			out = append(out, s.instance(&instanceSpec{
				kind:       arch.StrengthExecution,
				detector:   "execution.lifecycle",
				subject:    typ + " " + pair[0] + "/" + pair[1],
				object:     typ,
				basis:      arch.BasisCandidate,
				confidence: arch.ConfidenceLow,
				sites:      sites,
				message: fmt.Sprintf(
					"callers of %s must call %s before %s",
					typ,
					pair[0],
					pair[1],
				),
				fix: "return the opened value from a constructor and close it with a defer at the same site",
			}))
		}
	}
	return out
}

// unpairedLockDetector reports a function that locks a mutex without
// unlocking the same expression, directly or in a defer, leaving the unlock
// to another function (connascence.md §3.1, detector 3).
func unpairedLockDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.LockCalls, func(l arch.LockCall) (string, bool) {
		return l.Site.Func + "\x00" + l.Expr, l.Site.Func != ""
	}) {
		for _, pair := range [][2]string{{"Lock", "Unlock"}, {"RLock", "RUnlock"}} {
			locks := slices.ContainsFunc(
				g,
				func(l arch.LockCall) bool { return l.Method == pair[0] },
			)
			unlocks := slices.ContainsFunc(
				g,
				func(l arch.LockCall) bool { return l.Method == pair[1] },
			)
			if !locks || unlocks {
				continue
			}
			out = append(out, s.unpaired(g, pair))
		}
	}
	return out
}

// unpaired builds the instance of one function's unpaired lock, with every
// other function of the same type that releases the same expression.
func (s *subject) unpaired(g []arch.LockCall, pair [2]string) arch.Connascence {
	site := g[0].Site
	sites := []arch.Site{site}
	for _, l := range s.p.Facts.LockCalls {
		if l.Expr == g[0].Expr && l.Method == pair[1] && l.Site.Type == site.Type &&
			l.Site.Func != site.Func {
			sites = append(sites, l.Site)
		}
	}
	return s.instance(&instanceSpec{
		kind:       arch.StrengthExecution,
		detector:   "execution.unpaired-lock",
		subject:    site.Func + ": " + g[0].Expr + "." + pair[0],
		object:     cmp.Or(site.Type, site.Func),
		basis:      arch.BasisCandidate,
		confidence: arch.ConfidenceLow,
		sites:      sites,
		message: fmt.Sprintf(
			"%s calls %s.%s without %s",
			site.Func,
			g[0].Expr,
			pair[0],
			pair[1],
		),
		fix: fmt.Sprintf(
			"call defer %s.%s() right after the %s",
			g[0].Expr,
			pair[1],
			pair[0],
		),
	})
}

// timeCallDetector reports functions that wait for elapsed time instead of an
// event (connascence.md §3.2).
func timeCallDetector(s *subject) []arch.Connascence {
	waits := map[string]bool{
		"time.Sleep": true, "time.After": true, "time.Tick": true,
		"time.NewTimer": true, "time.NewTicker": true, "time.AfterFunc": true,
	}
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.TimeCalls, func(t arch.TimeCall) (string, bool) {
		return t.Site.Func, waits[t.Callee] && t.Site.Func != ""
	}) {
		sites := make([]arch.Site, len(g))
		callees := map[string]bool{}
		for i := range g {
			sites[i], callees[g[i].Callee] = g[i].Site, true
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthTiming,
			detector:   "timing.wait",
			subject:    g[0].Site.Func,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf("%s waits on elapsed time with %s", g[0].Site.Func,
				strings.Join(slices.Sorted(maps.Keys(callees)), ", ")),
			fix: "synchronize on an event, a channel or context cancellation, instead of a duration",
		}))
	}
	return out
}

// noDeadlineDetector reports outbound HTTP calls through the net/http
// helpers that use the default client, which has no timeout: the caller
// waits as long as the remote takes, connascence of timing with a service
// it does not control (connascence.md §9.7, runtime coupling). A fact, one
// instance per calling function.
func noDeadlineDetector(s *subject) []arch.Connascence {
	helpers := map[string]bool{
		"net/http.Get": true, "net/http.Head": true, "net/http.Post": true,
		"net/http.PostForm": true,
	}
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.Calls, func(c arch.Call) (string, bool) {
		return c.Site.Func, helpers[c.Callee] && c.Site.Func != ""
	}) {
		sites := make([]arch.Site, len(g))
		callees := map[string]bool{}
		for i := range g {
			sites[i], callees[g[i].Callee] = g[i].Site, true
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthTiming,
			detector:   "timing.no-deadline",
			subject:    g[0].Site.Func,
			basis:      arch.BasisFact,
			confidence: arch.ConfidenceHigh,
			sites:      sites,
			message: fmt.Sprintf("%s calls %s, whose default client has no timeout, so it waits "+
				"as long as the remote takes", g[0].Site.Func,
				strings.Join(slices.Sorted(maps.Keys(callees)), ", ")),
			fix: "send the request through an http.Client with a Timeout, or with a context " +
				"that has a deadline",
		}))
	}
	return out
}

// equalDurationDetector reports duration constants with one value in two or
// more packages, such as a client's and a server's timeout
// (connascence.md §3.2).
func equalDurationDetector(s *subject) []arch.Connascence {
	return s.equalConstants(func(k arch.Constant) bool { return k.Type == "time.Duration" },
		arch.StrengthTiming, "timing.equal-duration",
		"keep a shared timeout in one place that both sides read")
}

// foreignSwitchDetector reports switches over another package's enumeration.
// Without a default clause, a new constant reaches none of them by itself, a
// fact; with one, a new value may be mishandled silently, a candidate
// (connascence.md §3.3, detector 1).
func foreignSwitchDetector(s *subject) []arch.Connascence {
	enums := map[string][]arch.Site{}
	for i := range s.p.Facts.Constants {
		k := &s.p.Facts.Constants[i]
		enums[k.Type] = append(enums[k.Type], k.Site)
	}
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.Switches, func(sw arch.Switch) (string, bool) {
		key := sw.Type + "\x00" + strconv.FormatBool(sw.HasDefault)
		return key, sw.Package != sw.Site.Package && len(enums[sw.Type]) > 0
	}) {
		sites := slices.Clone(enums[g[0].Type])
		for _, sw := range g {
			sites = append(sites, sw.Site)
		}
		spec := instanceSpec{
			kind:     arch.StrengthValue,
			detector: "value.foreign-switch",
			subject:  g[0].Type,
			object:   g[0].Type,
			basis:    arch.BasisFact,
			sites:    sites,
			message: fmt.Sprintf(
				"%s outside its package %s %s without a default",
				count(len(g), "switch", "switches"), verb(len(g), "enumerates", "enumerate"),
				g[0].Type,
			),
			fix: "move the behaviour into a method on the type, or a table in its package, so a new value is added in one place",
		}
		if g[0].HasDefault {
			spec.detector, spec.basis, spec.confidence = "value.foreign-switch-default", arch.BasisCandidate, arch.ConfidenceLow
			spec.message = fmt.Sprintf(
				"%s outside its package %s %s and %s back to a default",
				count(len(g), "switch", "switches"), verb(len(g), "enumerates", "enumerate"),
				g[0].Type, verb(len(g), "falls", "fall"),
			)
		}
		out = append(out, s.instance(&spec))
	}
	return out
}

// equalConstantDetector reports package-level constants of one type and
// value in two or more packages; when their names also match, they almost
// certainly must change together (connascence.md §3.3, detector 2).
func equalConstantDetector(s *subject) []arch.Connascence {
	return s.equalConstants(func(k arch.Constant) bool {
		return k.Type != "time.Duration" && !excludedValue(k.Value)
	}, arch.StrengthValue, "value.equal-constants",
		"declare the value once and refer to it from both packages")
}

// equalConstants groups the constants keep accepts by type and value, and
// reports the groups spanning packages: high confidence when every name
// matches, low otherwise.
func (s *subject) equalConstants(
	keep func(arch.Constant) bool,
	kind arch.Strength,
	detector, fix string,
) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.Constants, func(k arch.Constant) (string, bool) {
		return k.Type + "\x00" + k.Value, keep(k) && !k.Alias
	}) {
		sites := make([]arch.Site, len(g))
		sameName := true
		for i := range g {
			k := &g[i]
			sites[i] = k.Site
			sameName = sameName && lastName(k.ID) == lastName(g[0].ID)
		}
		if distinct(sites, sitePackage) < 2 {
			continue
		}
		confidence := arch.ConfidenceLow
		if sameName {
			confidence = arch.ConfidenceHigh
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       kind,
			detector:   detector,
			subject:    g[0].Value + " (" + g[0].Type + ")",
			basis:      arch.BasisCandidate,
			confidence: confidence,
			sites:      sites,
			message: fmt.Sprintf("%d packages declare constants of type %s equal to %s",
				distinct(sites, sitePackage), g[0].Type, g[0].Value),
			fix: fix,
		}))
	}
	return out
}

// sharedVariableDetector reports exported package variables that functions
// in other packages use: they must all reference the same entity
// (connascence.md §3.4, detector 1). A variable that some function writes is
// shared mutable state, a fact. One that nothing writes after its
// initializer, such as a sentinel error, behaves as a constant, so it is a
// low-confidence candidate.
func sharedVariableDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.GlobalUses, func(u arch.GlobalUse) (string, bool) {
		return u.Var, u.Exported
	}) {
		if !slices.ContainsFunc(
			g,
			func(u arch.GlobalUse) bool { return u.Site.Package != u.Package },
		) {
			continue
		}
		sites := make([]arch.Site, len(g))
		for i := range g {
			sites[i] = g[i].Site
		}
		spec := instanceSpec{
			kind:     arch.StrengthIdentity,
			detector: "identity.shared-variable",
			subject:  g[0].Var,
			object:   g[0].Var,
			basis:    arch.BasisFact,
			sites:    sites,
			message: fmt.Sprintf(
				"%d packages use the package variable %s",
				distinct(sites, sitePackage),
				g[0].Var,
			),
			fix: "pass the shared value in explicitly instead of reaching for a package variable",
		}
		if !slices.ContainsFunc(g, func(u arch.GlobalUse) bool { return u.Write }) {
			spec.detector, spec.basis, spec.confidence = "identity.shared-read-only", arch.BasisCandidate, arch.ConfidenceLow
			spec.message = fmt.Sprintf(
				"%d packages read the package variable %s, which nothing writes",
				distinct(sites, sitePackage),
				g[0].Var,
			)
			spec.fix = "if it never changes, a constant or a function says so; otherwise pass it in explicitly"
		}
		out = append(out, s.instance(&spec))
	}
	return out
}

// sharedStateDetector reports unexported package variables written in two or
// more functions (connascence.md §3.4, detector 2).
func sharedStateDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.GlobalUses, func(u arch.GlobalUse) (string, bool) {
		return u.Var, !u.Exported && u.Write
	}) {
		sites := make([]arch.Site, len(g))
		for i := range g {
			sites[i] = g[i].Site
		}
		if distinct(sites, siteEntity) < 2 {
			continue
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthIdentity,
			detector:   "identity.shared-state",
			subject:    g[0].Var,
			object:     g[0].Var,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf(
				"%d functions write the package variable %s",
				distinct(sites, siteEntity),
				g[0].Var,
			),
			fix: "keep the state in a value its owner creates and passes in",
		}))
	}
	return out
}

// pointerIdentityDetector reports pointer comparisons on another analysed
// package's type: the two sides must hold the very same instance
// (connascence.md §3.4, detector 3). Types outside the analysis, such as
// go/types's, are their own packages' business.
func pointerIdentityDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.PointerEqs, func(p arch.PointerEq) (string, bool) {
		return p.Type, p.Package != p.Site.Package && s.inScope(p.Package)
	}) {
		sites := make([]arch.Site, len(g))
		for i := range g {
			sites[i] = g[i].Site
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthIdentity,
			detector:   "identity.pointer-equality",
			subject:    g[0].Type,
			object:     g[0].Type,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf(
				"%s of *%s by identity outside its package",
				count(len(g), "comparison", "comparisons"),
				g[0].Type,
			),
			fix: "compare by a key the type exposes, or let its package decide equality",
		}))
	}
	return out
}

// isExported reports a Go identifier that begins with an upper-case letter.
func isExported(name string) bool { return name != "" && name[0] >= 'A' && name[0] <= 'Z' }
