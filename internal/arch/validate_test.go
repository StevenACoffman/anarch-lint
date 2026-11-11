package arch_test

import (
	"fmt"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// A commit reads as a fix by a word of its subject, not a part of one.
func TestIsFix(t *testing.T) {
	t.Parallel()
	for subject, expected := range map[string]bool{
		"balance: fix the hotspot cap":       true,
		"Fixes #12":                          true,
		"Revert \"fitness: add expects\"":    true,
		"hotfix for the release":             true,
		"arch: prefix every finding":         false,
		"judging: debug output for the gold": false,
	} {
		if got := arch.IsFix(subject); got != expected {
			t.Errorf("IsFix(%q) = %v, expected %v", subject, got, expected)
		}
	}
}

// Complexity that ranks the packages as their later fixes do, with size
// varying apart from both, is predictive; a metric that does not vary, such
// as Ca with no imports, cannot be tested; and too few packages test
// nothing.
func TestValidate(t *testing.T) {
	t.Parallel()
	lines := []int{50, 110, 20, 80, 120, 10, 70, 30, 100, 60, 90, 40}
	var p arch.Program
	fixes := map[string][]string{}
	for i, n := range lines {
		pkg := fmt.Sprintf("m/p%02d", i)
		file := fmt.Sprintf("p%02d/f.go", i)
		p.Packages = append(p.Packages, arch.Package{Path: pkg, Files: []string{file}})
		p.Files = append(p.Files, arch.File{Path: file, Package: pkg, Lines: n})
		p.Funcs = append(p.Funcs, arch.Func{
			ID: pkg + ".F", Package: pkg, Name: "F", Complexity: arch.Complexity{Modified: i + 1},
		})
		for c := range i + 1 {
			fixes[file] = append(fixes[file], fmt.Sprintf("c%d", c))
		}
	}
	history := arch.FixHistory{Since: "r", Until: "h", Commits: 20, FixCommits: 12, Fixes: fixes}
	v := arch.Validate(
		&arch.ValidationInput{Program: p, Model: arch.Model{Version: 1}, History: history},
	)
	verdicts := map[string]string{}
	for _, mv := range v.Metrics {
		verdicts[mv.Metric] = mv.Verdict
	}
	equals(t, "predictive", verdicts["complexity"])
	equals(t, "insufficient-data", verdicts["ca"])
	equals(t, 12, v.Packages)

	p.Packages, p.Files, p.Funcs = p.Packages[:9], p.Files[:9], p.Funcs[:9]
	v = arch.Validate(
		&arch.ValidationInput{Program: p, Model: arch.Model{Version: 1}, History: history},
	)
	for _, mv := range v.Metrics {
		equals(t, "insufficient-data", mv.Verdict)
	}
}
