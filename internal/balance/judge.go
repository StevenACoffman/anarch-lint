package balance

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Rules of the Balanced Coupling findings and candidates.
const (
	ruleTightCoupling = "tight-coupling-"
	ruleLeakedModel   = "leaked-model"
	ruleLowCohesion   = "low-cohesion"
	ruleReflection    = "intrusive-reflection"
)

// judge turns pairs and the program's own structure into findings and
// candidates. A finding rests on facts; a candidate that would gate gets a
// slot, and gates once an approved verdict confirms it.
type judge struct {
	p        *arch.Program
	slots    *slotBook
	severity arch.Severity
}

// apply judges every pair, and adds the low-cohesion and reflection
// candidates.
func (j *judge) apply(r *Report) {
	for i := range r.Pairs {
		j.judgePair(r, &r.Pairs[i])
	}
	r.Candidates = append(r.Candidates, j.cohesionCandidates()...)
	r.Candidates = append(r.Candidates, j.reflectionCandidates()...)
}

// judgePair reports tight coupling, and a model leaked between bounded
// contexts.
func (j *judge) judgePair(r *Report, p *Pair) {
	if p.Quadrant == QuadrantTightCoupling &&
		p.Volatility != subdomainSupporting && p.Volatility != subdomainGeneric {
		j.gateable(r, p, &Candidate{
			Rule:    ruleTightCoupling + p.Level.String(),
			Subject: p.From,
			Object:  p.To,
			Message: fmt.Sprintf("%s couples to %s at the %s level across a distance of %d, "+
				"and %s: a step toward a distributed monolith",
				p.From, p.To, p.Level, p.Distance, volatilityText(p)),
			Fix: tightFix(p.Level),
		})
	}
	if p.Level == LevelModel && j.bothBoundedContexts(p) && !j.sharesModelOnPurpose(p) {
		j.gateable(r, p, &Candidate{
			Rule:    ruleLeakedModel,
			Subject: p.From,
			Object:  p.To,
			Message: fmt.Sprintf("bounded contexts %s and %s share %s's model", p.From, p.To, p.To),
			Fix:     "integrate through an anti-corruption layer or an open-host service with a published language",
		})
	}
}

// gateable files c as a finding when the pair's level rests on facts, and
// otherwise as a candidate a reviewer can confirm into a finding.
func (j *judge) gateable(r *Report, p *Pair, c *Candidate) {
	c.Evidence = slices.Concat(pairEvidence(p), p.Evidence)
	c.Site = &p.Site
	cand := fingerprinted(c)
	if p.Basis == arch.BasisFact {
		r.Findings = append(r.Findings, j.finding(&cand))
		return
	}
	switch j.slots.ask("balance/"+cand.Fingerprint, cand.Evidence) {
	case "confirmed":
		r.Candidates = append(r.Candidates, cand)
		r.Findings = append(r.Findings, j.finding(&cand))
	case "rejected":
		r.Rejected = append(r.Rejected, cand)
	default:
		r.Candidates = append(r.Candidates, cand)
	}
}

// finding turns a candidate into a finding with the same fingerprint.
func (j *judge) finding(c *Candidate) arch.Finding {
	f := arch.NewFinding(
		arch.FindingID{Rule: c.Rule, Subject: c.Subject, Object: c.Object},
		j.severity,
	)
	f.Message, f.Fix, f.Evidence = c.Message, c.Fix, c.Evidence
	if c.Site != nil {
		f.Location = arch.SpanAt(arch.Position{File: c.Site.File, Line: c.Site.Line})
	}
	return f
}

// pairEvidence names a pair's dimensions, as summary_rules principle 6 asks.
func pairEvidence(p *Pair) []string {
	return []string{
		fmt.Sprintf(
			"strength %s (%s), distance %d (locality %s, owner boundary %t, deploy boundary %t, async %t), "+
				"volatility %s, balance %d",
			p.Level,
			p.Basis,
			p.Distance,
			p.Locality,
			p.OwnerBoundary,
			p.DeployBoundary,
			p.Async,
			p.Volatility,
			p.Score,
		),
	}
}

// tightFix says how to rebalance a tight coupling at each level.
func tightFix(l Level) string {
	switch l {
	case LevelIntrusive:
		return "integrate through the upstream's interface, a façade or an anti-corruption layer, " +
			"instead of its implementation"
	case LevelFunctional:
		return "co-locate the shared functionality, or replace the duplicated knowledge with a " +
			"published event or a contract"
	default:
		return "lower the shared knowledge to a contract, or move the two closer together"
	}
}

// bothBoundedContexts reports that approved verdicts call both ends of a
// pair bounded contexts.
func (j *judge) bothBoundedContexts(p *Pair) bool {
	contexts := 0
	for _, sub := range []string{p.From, p.To} {
		if j.slots.answered("bounded-context/"+sub) == "yes" {
			contexts++
		}
	}
	return contexts == 2
}

// sharesModelOnPurpose reports an approved integration pattern under which
// the two contexts share a model by design.
func (j *judge) sharesModelOnPurpose(p *Pair) bool {
	ev := append([]string{fmt.Sprintf("%s depends on %s at the model level", p.From, p.To)},
		p.Evidence...)
	switch j.slots.ask("integration-pattern/"+p.From+"->"+p.To, ev) {
	case "shared-kernel", "partnership", "conformist":
		return true
	default:
		return false
	}
}

// cohesionCandidates are the low-cohesion quadrant's evidence inside one
// package: types whose methods split into independent groups, and packages
// whose files share no names (connascence.md §9.5). Unrelatedness is a
// matter of meaning, so these are candidates that never gate.
func (j *judge) cohesionCandidates() []Candidate {
	var out []Candidate
	for _, c := range arch.CohesionOf(j.p) {
		if c.LCOM4 > 1 {
			out = append(out, fingerprinted(&Candidate{
				Rule:    ruleLowCohesion,
				Subject: c.Type,
				Message: fmt.Sprintf(
					"%s has %d independent groups of methods and fields",
					c.Type,
					c.LCOM4,
				),
				Fix: "split the type along those groups, or move each group closer to what it serves",
			}))
		}
	}
	for _, pkg := range j.unrelatedFilePackages() {
		out = append(out, fingerprinted(&Candidate{
			Rule:    ruleLowCohesion,
			Subject: pkg,
			Message: fmt.Sprintf(
				"no file of %s refers to a name another of its files declares",
				pkg,
			),
			Fix: "split the package by file, or name what its files have in common",
		}))
	}
	return out
}

// unrelatedFilePackages returns the packages of two or more files in which no
// file uses an object another file declares.
func (j *judge) unrelatedFilePackages() []string {
	declFile := map[string]string{}
	for i := range j.p.Types {
		declFile[j.p.Types[i].ID] = j.p.Types[i].Span.File
	}
	for i := range j.p.Funcs {
		declFile[j.p.Funcs[i].ID] = j.p.Funcs[i].Span.File
	}
	related := map[string]bool{}
	for i := range j.p.Facts.Uses {
		u := &j.p.Facts.Uses[i]
		if f, ok := declFile[u.Object]; ok && u.Package == u.Site.Package && f != u.Site.File {
			related[u.Package] = true
		}
	}
	var out []string
	for _, pkg := range j.p.Packages {
		if len(pkg.Files) > 1 && !related[pkg.Path] {
			out = append(out, pkg.Path)
		}
	}
	return out
}

// reflectionCandidates are reflect lookups of an unexported name: access to
// some type's implementation details, intrusive coupling to a type the
// source does not name.
func (j *judge) reflectionCandidates() []Candidate {
	var out []Candidate
	for i := range j.p.Facts.ReflectByNames {
		rb := &j.p.Facts.ReflectByNames[i]
		if first, _ := utf8.DecodeRuneInString(rb.Name); !unicode.IsLower(first) {
			continue
		}
		site := rb.Site
		out = append(out, fingerprinted(&Candidate{
			Rule: ruleReflection, Subject: rb.Site.Package, Object: rb.Name,
			Message: fmt.Sprintf(
				"%s looks up the unexported name %q with %s",
				siteName(
					rb.Site,
				),
				rb.Name,
				strings.NewReplacer("(", "", ")", "").Replace(rb.Method),
			),
			Fix:  "ask the owning package for the value through its interface",
			Site: &site,
		}))
	}
	return out
}

// volatilityText says how volatile a pair's upstream is.
func volatilityText(p *Pair) string {
	if p.Volatility == subdomainUnjudged {
		return p.To + "'s volatility is unjudged, so it counts as core"
	}
	return p.To + " is a " + p.Volatility + " subdomain"
}
