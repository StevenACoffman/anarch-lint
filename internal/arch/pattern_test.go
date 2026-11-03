package arch_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestPatternTruthTable: every form of a model pattern against the paths
// that tell its cases apart (IMPROVEMENTS.md §8.4).
func TestPatternTruthTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pattern, path string
		ok            bool
		captures      []string
	}{
		{pattern: "m/a", path: "m/a", ok: true},
		{pattern: "m/a", path: "m/a/b"},
		{pattern: "m/a", path: "m/ab"},
		{pattern: "m/a/...", path: "m/a", ok: true},
		{pattern: "m/a/...", path: "m/a/b/c", ok: true},
		{pattern: "m/a/...", path: "m/ab"},
		{pattern: "m/$*", path: "m/a", ok: true, captures: []string{"a"}},
		{pattern: "m/$*", path: "m/a/b"},
		{pattern: "m/$*", path: "m"},
		{pattern: "m/$*/...", path: "m/a/b", ok: true, captures: []string{"a"}},
		{pattern: "m/$*/x", path: "m/a/x", ok: true, captures: []string{"a"}},
		{pattern: "m/$*/x", path: "m/a/y"},
		{pattern: "m/$**", path: "m/a/b", ok: true, captures: []string{"a/b"}},
		{pattern: "m/$**", path: "m"},
		{pattern: "m/$**/x", path: "m/a/b/x", ok: true, captures: []string{"a/b"}},
		{pattern: "m/$**/x", path: "m/x/x/x", ok: true, captures: []string{"x/x"}},
		{pattern: "m/$**/x/...", path: "m/x/x/x", ok: true, captures: []string{"x"}},
		{pattern: "m/$**/x/...", path: "m/a/x/y/x", ok: true, captures: []string{"a"}},
		{pattern: "m/$*/$*", path: "m/a/b", ok: true, captures: []string{"a", "b"}},
		{pattern: "m/$*/$**", path: "m/a/b/c", ok: true, captures: []string{"a", "b/c"}},
	}
	for _, tc := range cases {
		t.Run(tc.pattern+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			_, captures, ok := arch.MatchCaptures(tc.pattern, tc.path)
			equals(t, tc.ok, ok)
			equals(t, tc.captures, captures)
		})
	}
}

// TestPatternScores: an exact pattern beats a "/..." pattern of the same
// root, and a longer literal beats a shorter one, whatever the wildcards.
func TestPatternScores(t *testing.T) {
	t.Parallel()
	score := func(pattern string) int {
		s, _, ok := arch.MatchCaptures(pattern, "m/a/b")
		assert(t, ok, pattern+" does not match m/a/b")
		return s
	}
	assert(t, score("m/a/b") > score("m/a/b/..."), "exact does not beat /...")
	assert(t, score("m/a/b/...") > score("m/a/..."), "longer does not beat shorter")
	assert(t, score("m/a/$*") > score("m/$*/$*"), "more literal does not beat less")
}

// TestSubstituteRoundTrip: a pattern's captures, put back into the same
// pattern with "$1" for each "$*", give back the path it matched.
func TestSubstituteRoundTrip(t *testing.T) {
	t.Parallel()
	segment := rapid.StringMatching(`[a-z]{1,4}`)
	rapid.Check(t, func(t *rapid.T) {
		a, b, c := segment.Draw(t, "a"), segment.Draw(t, "b"), segment.Draw(t, "c")
		path := "m/" + a + "/" + b + "/" + c
		_, captures, ok := arch.MatchCaptures("m/$*/"+b+"/$*", path)
		if !ok {
			t.Fatalf("m/$*/%s/$* does not match %s", b, path)
		}
		if got := arch.Substitute("m/$1/"+b+"/$2", captures); got != path {
			t.Fatalf("substituted %q, want %q", got, path)
		}
	})
}
