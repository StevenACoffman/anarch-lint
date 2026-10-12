package connascence

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Fixed inputs of the detectors. None is a model knob: each is recorded as
// an assumption, and becomes a knob only once a user needs it.
const (
	// longParamList is the parameter count from which callers depend on the
	// order of the arguments (connascence.md §2.4, detector 2).
	longParamList = 4
)

// detectors returns every detector, in the order of
// connascence.md §2 and §3. A function, not a package variable, so no
// global state exists.
func detectors() []func(*subject) []arch.Connascence {
	return []func(*subject) []arch.Connascence{
		nameDetector, typeDetector,
		repeatedLiteralDetector, shadowedConstantDetector, primitiveParamDetector, nilNilDetector,
		unkeyedLiteralDetector, longParamDetector, sameTypeResultsDetector, constIndexDetector,
		samePatternDetector, sameLayoutDetector, sameAlgorithmDetector, cloneDetector,
		guardDetector, lifecycleDetector, unpairedLockDetector,
		timeCallDetector, equalDurationDetector,
		foreignSwitchDetector, equalConstantDetector,
		sharedVariableDetector, sharedStateDetector, pointerIdentityDetector,
		enumCollisionDetector, registrationDetector,
	}
}

// excludedValue reports a value with no meaning of its own, left out of the
// value detectors (connascence.md §2.3, detector 1).
func excludedValue(v string) bool {
	switch v {
	case "0", "1", "-1", `""`:
		return true
	default:
		return false
	}
}

// exemptCallee reports a callee whose constant arguments are messages or
// formats rather than shared values: fmt, errors, log and testing.
func exemptCallee(callee string) bool {
	pkg := packageOfFullName(callee)
	return pkg == "fmt" || pkg == "errors" || pkg == "log" || pkg == "log/slog" || pkg == "testing"
}

// packageOfFullName returns the package of a types.Func full name:
// "strings.Split", "(*net/http.ServeMux).Handle", "(time.Time).Format".
func packageOfFullName(name string) string {
	name = strings.TrimPrefix(strings.TrimPrefix(name, "("), "*")
	if i := strings.Index(name, ")"); i >= 0 {
		name = name[:i]
	}
	if i := strings.LastIndex(name, "."); i > strings.LastIndex(name, "/") {
		return name[:i]
	}
	return name
}

// lastName returns the part of an ID after its last dot.
func lastName(id string) string { return id[strings.LastIndex(id, ".")+1:] }

// distinct counts the distinct keys of sites.
func distinct(sites []arch.Site, key func(arch.Site) string) int {
	seen := map[string]bool{}
	for _, s := range sites {
		seen[key(s)] = true
	}
	return len(seen)
}

func sitePackage(s arch.Site) string { return s.Package }

// siteEntity is the function a site lies in, or the declaration it belongs
// to at package level.
func siteEntity(s arch.Site) string {
	if s.Func != "" {
		return s.Func
	}
	return s.Type + "@" + s.File
}

// grouped returns xs grouped by key, the groups in key order.
func grouped[T any](xs []T, key func(T) (string, bool)) [][]T {
	groups := map[string][]T{}
	for _, x := range xs {
		if k, ok := key(x); ok {
			groups[k] = append(groups[k], x)
		}
	}
	out := make([][]T, 0, len(groups))
	for _, k := range slices.Sorted(maps.Keys(groups)) {
		out = append(out, groups[k])
	}
	return out
}

// inScope reports a package under analysis.
func (s *subject) inScope(pkg string) bool {
	_, ok := s.c.Module(pkg)
	return ok
}

// count writes n with the noun's singular or plural form.
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// verb picks the form of a verb that agrees with n.
func verb(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
