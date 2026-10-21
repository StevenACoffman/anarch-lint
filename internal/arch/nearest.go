package arch

import (
	"cmp"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"
)

// maxSuggestions bounds the corrections a finding suggests.
const maxSuggestions = 3

// Suggest returns up to three corrections of a pattern that matches
// nothing, nearest first. A correction is a misspelling of one segment: the
// pattern's literal leading segments, the ones before its first "$*", "$**"
// or "/...", are a candidate's leading segments but for one, which is within
// a Levenshtein distance of a third of its length, and at least one, of the
// candidate's. A pattern that starts with a wildcard, or whose literal
// segments some candidate already starts with, has no suggestion: its
// problem is not a misspelling.
//
// Ensures: sorted by distance, then by suggestion; no duplicates.
func Suggest(pattern string, candidates []string) []string {
	literal, tail := literalHead(pattern)
	if literal == "" {
		return nil
	}
	want := strings.Split(literal, "/")
	best := map[string]int{}
	for _, c := range candidates {
		have := strings.Split(c, "/")
		if len(have) < len(want) {
			continue
		}
		have = have[:len(want)]
		d, ok := oneSegmentApart(want, have)
		if d == 0 {
			return nil
		}
		if !ok {
			continue
		}
		s := strings.Join(have, "/") + tail
		if old, seen := best[s]; !seen || d < old {
			best[s] = d
		}
	}
	out := make([]string, 0, len(best))
	for s := range best {
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b string) int {
		return cmp.Or(cmp.Compare(best[a], best[b]), cmp.Compare(a, b))
	})
	return out[:min(len(out), maxSuggestions)]
}

// Corrections returns a finding's evidence for patterns that match nothing:
// one line, "PATTERN: did you mean A or B?", for each pattern Suggest
// corrects from candidates, in the patterns' order, or nil.
func Corrections(patterns, candidates []string) []string {
	var out []string
	for _, p := range patterns {
		if s := Suggest(p, candidates); len(s) > 0 {
			out = append(out, p+": did you mean "+strings.Join(s, " or ")+"?")
		}
	}
	return out
}

// oneSegmentApart compares two paths of as many segments. It returns 0 when
// they are equal, and otherwise the distance between the one segment they
// differ in and whether that is close enough to be a misspelling. Paths that
// differ in two segments are not.
func oneSegmentApart(want, have []string) (distance int, ok bool) {
	differ := -1
	for i := range want {
		if want[i] == have[i] {
			continue
		}
		if differ >= 0 {
			return -1, false
		}
		differ = i
	}
	if differ < 0 {
		return 0, false
	}
	d := levenshtein.ComputeDistance(want[differ], have[differ])
	return d, d <= max(1, len(have[differ])/3)
}

// literalHead splits a pattern into its literal leading segments and the
// rest, which starts with "/" when it is not empty.
func literalHead(pattern string) (literal, tail string) {
	segs := strings.Split(pattern, "/")
	for i, s := range segs {
		if s == tokenOne || s == tokenMany || s == "..." {
			return strings.Join(segs[:i], "/"), "/" + strings.Join(segs[i:], "/")
		}
	}
	return pattern, ""
}
