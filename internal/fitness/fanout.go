package fitness

import (
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// checkFanOut reports each package matching a fanOut rule that imports more
// packages under analysis than the rule's max, at its first such import, with
// the imported packages as evidence. importsOf maps each package under
// analysis to its imports of packages under analysis.
func checkFanOut(l *ledger, rules []arch.FanOutRule, packages []string,
	importsOf map[string][]arch.Import,
) {
	for i := range rules {
		fo := &rules[i]
		r := l.addRule("fanOut", fo.RuleOptions, arch.DefaultRuleName(fo.Name, "fanOut", i))
		r.result.Matched = countMatching(packages, fo.Packages)
		for _, pkg := range packages {
			if !arch.MatchesAny(fo.Packages, pkg) {
				continue
			}
			imports := importsOf[pkg]
			if len(imports) <= fo.Max {
				continue
			}
			f := arch.NewFinding(
				arch.FindingID{Rule: arch.RuleFanOut, Subject: pkg, Object: r.result.Name},
				arch.SeverityError,
			)
			first := slices.MinFunc(imports, func(a, b arch.Import) int {
				return arch.ComparePositions(a.Pos, b.Pos)
			})
			f.Location = arch.SpanAt(first.Pos)
			f.Message = fmt.Sprintf("%s imports %d packages under analysis, above %d",
				pkg, len(imports), fo.Max)
			for _, imp := range imports[:min(len(imports), maxEdgeEvidence)] {
				f.Evidence = append(f.Evidence, "imports "+imp.To)
			}
			f.Fix = "split the package so each part depends on less, or reach several of these " +
				"through one package that hides them"
			l.violation(r, &f)
		}
	}
}

// importsInScope maps each importing package to its imports of packages
// under analysis, in the program's order.
func importsInScope(p *arch.Program, g *arch.Graphs) map[string][]arch.Import {
	out := map[string][]arch.Import{}
	for _, imp := range p.Imports {
		if _, in := g.SubsystemOf[imp.To]; in {
			out[imp.From] = append(out[imp.From], imp)
		}
	}
	return out
}
