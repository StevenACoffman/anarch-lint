package fitness

import (
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// GuardCoverage is how many of the error guards in a domain rule's packages
// its model's invariants claim: a checker shows that each declared rule
// holds, never that every rule was declared, so the unclaimed guards measure
// what the model may leave out (IMPROVEMENTS.md §10.4).
type GuardCoverage struct {
	Rule    string `json:"rule"`
	Guards  int    `json:"guards"`
	Claimed int    `json:"claimed"`
}

// guardCoverage counts the error guards of the rule's packages, and warns of
// those no invariant claims. An invariant claims a guard whose function
// handles a type one of its entities matched, or whose function an
// "//arch:invariant" directive of the model marks. Each unclaimed guard is a
// question for the modeller: a rule the model does not declare, or a check
// of the implementation, which may stay unclaimed.
//
// Requires: the model declares at least one invariant.
func (c *invariantCheck) guardCoverage(matched map[string]string) GuardCoverage {
	governs, enforces := c.governedTypes(matched), c.enforcerFuncs()
	handles := c.handlesByFunc()
	cov := GuardCoverage{Rule: c.r.result.Name}
	var unclaimed []*arch.Site
	for i := range c.program.Facts.ErrorGuards {
		site := &c.program.Facts.ErrorGuards[i].Site
		if !c.inRule(site.Package) {
			continue
		}
		cov.Guards++
		if enforces[site.Func] ||
			slices.ContainsFunc(
				handledBy(site, handles),
				func(t string) bool { return governs[t] },
			) {
			cov.Claimed++
			continue
		}
		unclaimed = append(unclaimed, site)
	}
	if len(unclaimed) > 0 && !c.r.off {
		c.unclaimed(&cov, unclaimed)
	}
	return cov
}

// governedTypes returns the types the entities of the model's invariants
// matched.
func (c *invariantCheck) governedTypes(matched map[string]string) map[string]bool {
	out := map[string]bool{}
	for i := range c.model.Invariants {
		for _, name := range governed(&c.model.Invariants[i], c.model) {
			if id, ok := matched[name]; ok {
				out[id] = true
			}
		}
	}
	return out
}

// enforcerFuncs returns the functions of the rule's packages that an
// "//arch:invariant" directive marks for an invariant of the model.
func (c *invariantCheck) enforcerFuncs() map[string]bool {
	out := map[string]bool{}
	for _, e := range c.program.Enforcers {
		if c.inRule(e.Package) && slices.ContainsFunc(c.model.Invariants,
			func(inv arch.DomainInvariant) bool { return inv.ID == e.Invariant }) {
			out[e.Func] = true
		}
	}
	return out
}

// unclaimed warns of the guards no invariant claims, at the first of them,
// resting on the rest so a change to any brings the warning into scope
// (docs/adr/0011). It is advice, so it never fails the rule.
func (c *invariantCheck) unclaimed(cov *GuardCoverage, sites []*arch.Site) {
	f := arch.NewFinding(arch.FindingID{
		Rule: arch.RuleGuardsUnclaimed, Subject: c.model.Path, Object: c.r.result.Name,
	}, arch.SeverityWarning)
	f.ModelRule, f.QualityAttribute = c.r.modelRule(), c.r.quality
	f.Location = arch.SpanAt(arch.Position{File: sites[0].File, Line: sites[0].Line})
	for _, s := range sites[1:] {
		f.RestOn(*arch.SpanAt(arch.Position{File: s.File, Line: s.Line}))
	}
	f.Message = fmt.Sprintf("no invariant of %s claims %d of the %d error guards in the "+
		"packages of %s: their functions handle no type an invariant governs, and enforce none",
		c.model.Path, len(sites), cov.Guards, c.r.result.Name)
	f.Fix = "for each guard that keeps a domain rule, declare the invariant and mark the " +
		"guard's function with //arch:invariant ID; one that checks the implementation may stay"
	for _, s := range sites[:min(len(sites), maxEdgeEvidence)] {
		f.Evidence = append(f.Evidence, guardLine(s))
	}
	f.Evidence = appendMore(f.Evidence, len(sites), "guards", c.model.Path)
	c.ledger.findings = append(c.ledger.findings, f)
}
