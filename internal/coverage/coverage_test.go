package coverage_test

import (
	"reflect"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/coverage"
)

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// Each block counts toward the function holding its first line, by the
// file name a profile gives: import path and base name. A block outside
// every function counts toward none.
func TestAttribute(t *testing.T) {
	t.Parallel()
	p := &arch.Program{
		Packages: []arch.Package{{Path: "m/a", Files: []string{"a/a.go"}}},
		Funcs: []arch.Func{
			{ID: "m/a.F", Package: "m/a", Span: arch.Span{File: "a/a.go", Start: 3, End: 10}},
			{ID: "m/a.G", Package: "m/a", Span: arch.Span{File: "a/a.go", Start: 12, End: 20}},
		},
	}
	profile := "mode: set\n" +
		"m/a/a.go:1.1,1.20 1 1\n" + // a package-level initializer
		"m/a/a.go:4.2,5.10 2 1\n" +
		"m/a/a.go:6.2,8.3 1 0\n" +
		"m/a/a.go:13.2,14.3 3 0\n"
	cov, err := coverage.Attribute([]byte(profile), p)
	if err != nil {
		t.Fatal(err)
	}
	equals(t, []arch.FuncCoverage{
		{Func: "m/a.F", Statements: 3, Covered: 2},
		{Func: "m/a.G", Statements: 3, Covered: 0},
	}, cov.Funcs)
	equals(t, 64, len(cov.Hash))
}

func TestAttributeRejectsMalformedProfile(t *testing.T) {
	t.Parallel()
	_, err := coverage.Attribute([]byte("mode: set\nm/a/a.go:nonsense\n"), &arch.Program{})
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}
