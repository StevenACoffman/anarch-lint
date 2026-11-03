package balance

import (
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// documentEvidence is each consumer of a route that no Go provider serves
// but an OpenAPI document of a fitness.openapi rule declares: coupling to
// the subsystems of the rule's serving packages through the published
// contract, which the code alone does not show, as when the provider
// builds its routes at run time or under a router's prefix
// (IMPROVEMENTS.md §5.1). A rule that names no serving packages names no
// provider, and gives no evidence.
//
// Requires: k.apis holds the documents of the model's fitness.openapi
// rules, in the rules' order.
func (k *classifier) documentEvidence() []evidence {
	if len(k.apis) == 0 {
		return nil
	}
	_, orphans := arch.MatchContracts(arch.ContractsOf(&k.p.Facts))
	var ev []evidence
	for i := range orphans {
		c := &orphans[i]
		if rt, ok := c.Route(); ok && c.Role != arch.ContractProvider {
			ev = append(ev, k.documentedConsumer(c, rt)...)
		}
	}
	return ev
}

// documentedConsumer is the evidence of one consumer of a route: one for
// each document that declares the route, and each subsystem other than the
// consumer's that serves it.
func (k *classifier) documentedConsumer(c *arch.Contract, rt arch.HTTPRoute) []evidence {
	from := k.subsystemOf(c.Site.Package)
	var ev []evidence
	for j := range min(len(k.apis), len(k.m.Fitness.OpenAPI)) {
		doc := &k.apis[j]
		op, ok := doc.Serves(rt)
		if !ok {
			continue
		}
		for _, to := range k.servingPackages(k.m.Fitness.OpenAPI[j].ServedBy) {
			if k.subsystemOf(to) == from {
				continue
			}
			ev = append(ev, evidence{
				from: from, to: k.subsystemOf(to), fromPkg: c.Site.Package, toPkg: to,
				level: LevelContract, basis: arch.BasisFact, kind: arch.StrengthName,
				nonImport: true, site: c.Site,
				note: fmt.Sprintf("%s consumes %s %s, which %s documents at line %d for %s",
					siteName(c.Site), op.Method, op.Path, doc.Path, op.Pos.Line, to),
			})
		}
	}
	return ev
}

// servingPackages returns the first package of each subsystem that a
// rule's serving packages match, sorted.
func (k *classifier) servingPackages(servedBy []string) []string {
	if len(servedBy) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for i := range k.p.Packages {
		pkg := k.p.Packages[i].Path
		if sub := k.subsystemOf(pkg); arch.MatchesAny(servedBy, pkg) && !seen[sub] {
			seen[sub] = true
			out = append(out, pkg)
		}
	}
	slices.Sort(out)
	return out
}
