package arch

import (
	"slices"
	"strconv"
	"strings"
)

// Pattern tokens. A model pattern is an import path, segment by segment:
//
//   - a literal segment matches itself;
//   - "$*" matches exactly one segment and captures it;
//   - "$**" matches one or more segments and captures them, joined by "/";
//   - a final "/..." matches the path before it and everything beneath it.
//
// "$1" to "$9" in another pattern refer back to the captures, in order
// (spaghetti-cutter's back-references; mmi-ref-plan.md §4.4). So
// "example.com/app/$*/..." with "example.com/app/$1/..." says "a package may
// import only within its own top-level area".
const (
	tokenOne  = "$*"
	tokenMany = "$**"
	tokenRest = "/..."
)

// matchPattern reports whether pattern matches path and how specifically.
// The score doubles the length of the pattern's literal text and adds one
// when the pattern has no wildcard at all, so an exact pattern beats a "/..."
// pattern of the same root, and a longer literal beats a shorter one.
func matchPattern(pattern, path string) (score int, ok bool) {
	score, _, ok = MatchCaptures(pattern, path)
	return score, ok
}

// MatchCaptures is matchPattern that also returns the captures of "$*" and
// "$**", in order. Where "$**" could capture several ways, it takes the
// shortest capture that lets the rest match, so the result is deterministic.
func MatchCaptures(pattern, path string) (score int, captures []string, ok bool) {
	root, rest := strings.CutSuffix(pattern, tokenRest)
	pat, segs := strings.Split(root, "/"), strings.Split(path, "/")
	captures, ok = matchSegments(pat, segs, rest, nil)
	if !ok {
		return 0, nil, false
	}
	literal := 0
	for _, p := range pat {
		if p != tokenOne && p != tokenMany {
			literal += len(p)
		}
	}
	literal += len(pat) - 1 // the separators
	score = 2 * literal
	if !rest && !strings.Contains(root, "$") {
		score++
	}
	return score, captures, true
}

// matchSegments matches pattern segments against path segments. rest allows
// path segments beyond the pattern, for a trailing "/...".
func matchSegments(pat, segs []string, rest bool, captures []string) ([]string, bool) {
	if len(pat) == 0 {
		return captures, len(segs) == 0 || rest
	}
	switch pat[0] {
	case tokenOne:
		if len(segs) == 0 {
			return nil, false
		}
		return matchSegments(pat[1:], segs[1:], rest, append(captures, segs[0]))
	case tokenMany:
		return matchMany(pat[1:], segs, rest, captures)
	default:
		if len(segs) == 0 || segs[0] != pat[0] {
			return nil, false
		}
		return matchSegments(pat[1:], segs[1:], rest, captures)
	}
}

// matchMany matches a "$**" followed by the pattern segments rest of it:
// the shortest capture of one or more segments that lets the remainder match.
// Each attempt appends to its own copy of captures, so one attempt cannot
// overwrite another's.
func matchMany(pat, segs []string, rest bool, captures []string) ([]string, bool) {
	for n := 1; n <= len(segs); n++ {
		attempt := append(slices.Clone(captures), strings.Join(segs[:n], "/"))
		if got, ok := matchSegments(pat, segs[n:], rest, attempt); ok {
			return got, true
		}
	}
	return nil, false
}

// substitute replaces "$1" to "$9" in pattern with the captures they name.
// A reference past the last capture is left as it is, so it matches nothing;
// a valid model never has one.
func substitute(pattern string, captures []string) string {
	for i := len(captures); i >= 1; i-- {
		pattern = strings.ReplaceAll(pattern, "$"+strconv.Itoa(i), captures[i-1])
	}
	return pattern
}

// captureCount returns the number of "$*" and "$**" tokens in pattern.
func captureCount(pattern string) int {
	n := 0
	for _, seg := range strings.Split(strings.TrimSuffix(pattern, tokenRest), "/") {
		if seg == tokenOne || seg == tokenMany {
			n++
		}
	}
	return n
}

// highestReference returns the largest "$n" a pattern refers to, or 0.
func highestReference(pattern string) int {
	highest := 0
	for i := 0; i+1 < len(pattern); i++ {
		if pattern[i] == '$' && pattern[i+1] >= '1' && pattern[i+1] <= '9' {
			highest = max(highest, int(pattern[i+1]-'0'))
		}
	}
	return highest
}
