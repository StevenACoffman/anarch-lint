package fitness

import (
	"fmt"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// libraryConstraint lets the packages a libraries rule names import, beyond
// the standard library and the packages under analysis (those in inScope),
// only the libraries it allows (docs/adr/0012).
func libraryConstraint(l *ledger, lr *arch.LibraryRule, i int, p *arch.Program,
	inScope map[string]string,
) constraint {
	r := l.addRule("libraries", lr.RuleOptions, arch.DefaultRuleName(lr.Name, "libraries", i))
	r.readsCode = matching(lr.From)
	for _, pkg := range p.Packages {
		if arch.MatchesAny(lr.From, pkg.Path) {
			r.result.Matched++
		}
	}
	third := func(to string) bool {
		_, in := inScope[to]
		return !in && !arch.IsStandard(to)
	}
	if !r.off {
		unusedApprovals(l, r, lr, p.RuleImports(), third)
	}
	return constraint{
		rule: r, kind: arch.RuleUnapprovedLibrary, anyTarget: true,
		reads: func(pkg string) string {
			return matchBits(pkg, lr.From, lr.Allow) + string(bit(arch.IsStandard(pkg)))
		},
		allowances: libraryAllowances(lr, third),
		fix: "use the standard library, or approve the library by adding it to the rule's " +
			"allow, with a decision record saying why",
		violates: func(from, to string) string {
			if !arch.MatchesAny(lr.From, from) || !third(to) || arch.MatchesAny(lr.Allow, to) {
				return ""
			}
			if len(lr.Allow) == 0 {
				return "it may import no third-party code"
			}
			return "it may import only " + strings.Join(
				lr.Allow,
				", ",
			) + " beyond the standard library"
		},
	}
}

// unusedApprovals warns of each library a rule approves that none of its
// packages imports: an approval outlives the need for it unless it is
// removed with the last import.
func unusedApprovals(
	l *ledger, r *rule, lr *arch.LibraryRule, imports []arch.Import, third func(string) bool,
) {
	for _, pat := range lr.Allow {
		used := slices.ContainsFunc(imports, func(imp arch.Import) bool {
			return arch.MatchesAny(lr.From, imp.From) && third(imp.To) &&
				arch.MatchesAny([]string{pat}, imp.To)
		})
		if used {
			continue
		}
		f := arch.NewFinding(
			arch.FindingID{Rule: arch.RuleUnusedApproval, Subject: r.result.Name, Object: pat},
			arch.SeverityWarning,
		)
		f.ModelRule = r.modelRule()
		f.Message = fmt.Sprintf("%s approves %s, but none of its packages imports it",
			r.result.Name, pat)
		f.Fix = "remove " + pat + " from the rule's allow"
		l.findings = append(l.findings, f)
	}
}

// libraryAllowances are the patterns a libraries rule approves.
func libraryAllowances(lr *arch.LibraryRule, third func(string) bool) []allowance {
	out := make([]allowance, 0, len(lr.Allow))
	for _, pat := range lr.Allow {
		out = append(out, allowance{
			item: "allow " + pat,
			names: func(from, to string) bool {
				return arch.MatchesAny(lr.From, from) && third(to) &&
					arch.MatchesAny([]string{pat}, to)
			},
		})
	}
	return out
}
