package arch_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestThresholdOverrides: the first override that matches a package gives
// its limits, filled from the module's where it sets none; any other
// package keeps the module's.
func TestThresholdOverrides(t *testing.T) {
	t.Parallel()
	fn := func(pkg string, lines, nesting int) arch.Func {
		return arch.Func{
			ID: pkg + ".F", Package: pkg, Name: "F", Lines: lines, MaxNesting: nesting,
			Span: arch.Span{File: pkg + ".go", Start: 1, End: lines},
		}
	}
	p := arch.Program{
		Packages: []arch.Package{{Path: "m/gen"}, {Path: "m/app"}},
		Funcs:    []arch.Func{fn("m/gen", 200, 6), fn("m/app", 200, 6)},
	}
	m := arch.Model{Version: 1, ThresholdOverrides: []arch.ThresholdOverride{
		{Packages: []string{"m/gen"}, Thresholds: arch.Thresholds{LargeFuncLines: 500}},
		{Packages: []string{"m/..."}, Thresholds: arch.Thresholds{MaxNesting: 10}},
	}}
	r := arch.Analyze(&arch.Input{Program: p, Model: m})
	var got []string
	for _, f := range r.Findings {
		got = append(got, f.Rule+" "+f.Subject)
	}
	// m/gen's override lifts its function limit but not its nesting limit,
	// and the second override never applies to it.
	equals(t, []string{"deep-nesting m/gen.F", "large-func m/app.F"}, got)

	_, err := arch.DecodeModel(strings.NewReader(
		`{"version": 1, "thresholdOverrides": [{"thresholds": {"largeFuncLines": 1}}]}`))
	equals(
		t,
		"model: incomplete-rule: thresholdOverrides[0] names no packages",
		arch.ErrorMessage(err),
	)
}
