package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

func TestSuggest(t *testing.T) {
	t.Parallel()
	pkgs := []string{
		"m/a/service", "m/a/service/sub", "m/a/store", "m/b/service", "m/billing/invoice",
	}
	cases := map[string]struct {
		pattern string
		want    []string
	}{
		"a misspelled package":       {"m/a/servce", []string{"m/a/service"}},
		"a misspelled tree":          {"m/a/servce/...", []string{"m/a/service/..."}},
		"before a capture":           {"m/biling/$*", []string{"m/billing/$*"}},
		"two equally near":           {"m/c/service", []string{"m/a/service", "m/b/service"}},
		"too far to be a typo":       {"m/a/database", []string{}},
		"two segments apart":         {"m/c/servce", []string{}},
		"another last segment":       {"m/a/service/none", []string{}},
		"a wildcard first":           {"$*/service", nil},
		"the literal already exists": {"m/a/service", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			equals(t, tc.want, arch.Suggest(tc.pattern, pkgs))
		})
	}
}

// Corrections come nearest first.
func TestSuggestNearestFirst(t *testing.T) {
	t.Parallel()
	equals(t, []string{"m/x/abcdef", "m/x/abcdyz"},
		arch.Suggest("m/x/abcdex", []string{"m/x/abcdyz", "m/x/abcdef"}))
}

// A stale subsystem pattern suggests the package it probably meant.
func TestStalePatternSuggestsCorrections(t *testing.T) {
	t.Parallel()
	p := arch.Program{Packages: []arch.Package{{Path: "fx/billing/invoice"}}}
	m := arch.Model{Version: 1, Subsystems: []arch.Subsystem{
		{Name: "billing", Packages: []string{"fx/billing/invoice", "fx/biling/..."}},
	}}
	r := arch.Analyze(&arch.Input{Program: p, Model: m})
	fs := findingsOf(&r, arch.RuleStalePattern)
	if len(fs) != 1 {
		t.Fatalf("want one stale pattern, got %v", fs)
	}
	equals(t, []string{"fx/biling/...: did you mean fx/billing/...?"}, fs[0].Evidence)
}
