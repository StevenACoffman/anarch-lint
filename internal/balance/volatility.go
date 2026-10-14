package balance

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Subdomain answers, and the volatility of a subsystem nobody has judged.
const (
	subdomainCore       = "core"
	subdomainSupporting = "supporting"
	subdomainGeneric    = "generic"
	subdomainUnjudged   = "unjudged"
)

// Churn thresholds of the diagnostics (connascence.md §9.4): borrowed
// guidance, listed among the assumptions.
const (
	highChurn = 20
	noChurn   = 0
)

// SubsystemVolatility is a subsystem's judged subdomain and its observed
// change history, which is evidence for the judgement, never the volatility
// itself (connascence.md §9.4).
type SubsystemVolatility struct {
	Name string `json:"name"`
	// Subdomain is core, supporting or generic from an approved verdict, or
	// unjudged.
	Subdomain string `json:"subdomain"`
	// BoundedContext is yes or no from an approved verdict, or "".
	BoundedContext string `json:"boundedContext,omitempty"`
	// Commits counts the commits of the revision range that changed the
	// subsystem's files; nil without a range.
	Commits *int `json:"commits,omitempty"`
}

// volatility asks for each declared subsystem's subdomain and whether it is a
// bounded context, with its packages, roles, churn and couplings as evidence.
//
// A subsystem's volatility decides something only when another depends on
// it or a revision range gives its churn; otherwise its slot is skipped.
func (s *subject) volatility(
	ev []evidence, groups map[pairKey][]evidence,
) ([]SubsystemVolatility, []SkippedSlot) {
	upstream := map[string]bool{}
	for k := range groups {
		upstream[k.to] = true
	}
	out := make([]SubsystemVolatility, 0, len(s.m.Subsystems))
	var skipped []SkippedSlot
	for _, sub := range s.m.Subsystems {
		v := SubsystemVolatility{Name: sub.Name, Subdomain: subdomainUnjudged}
		if s.in.Churn == nil && !upstream[sub.Name] {
			skipped = append(skipped, SkippedSlot{
				Criterion: "subdomain/" + sub.Name,
				Reason: "no subsystem depends on it and no revision range gives its churn, " +
					"so its volatility decides nothing",
			})
			out = append(out, v)
			continue
		}
		evidence := s.subsystemEvidence(sub.Name, ev)
		if s.in.Churn != nil {
			n := s.commitsOf(sub.Name)
			v.Commits = &n
			evidence = append(evidence, fmt.Sprintf("churn: %d commits from %s to %s",
				n, short(s.in.Churn.Since), short(s.in.Churn.Head)))
		}
		if answer := s.slots.ask(&arch.SlotRequest{
			Criterion: "subdomain/" + sub.Name, Evidence: evidence,
		}); answer != "" {
			v.Subdomain = answer
		}
		out = append(out, v)
	}
	return out, skipped
}

// boundedContexts asks whether each subsystem at either end of a
// model-level pair is a bounded context, which decides whether the pair
// leaks a model, and skips the others.
func (s *subject) boundedContexts(r *Report, ev []evidence) []SkippedSlot {
	ends := map[string]bool{}
	for i := range r.Pairs {
		if p := &r.Pairs[i]; p.Level == LevelModel {
			ends[p.From], ends[p.To] = true, true
		}
	}
	var skipped []SkippedSlot
	for i := range r.Subsystems {
		v := &r.Subsystems[i]
		if !ends[v.Name] {
			skipped = append(skipped, SkippedSlot{
				Criterion: "bounded-context/" + v.Name,
				Reason: "it shares no model with another subsystem, " +
					"so whether it is a bounded context decides nothing",
			})
			continue
		}
		v.BoundedContext = s.slots.ask(&arch.SlotRequest{
			Criterion: "bounded-context/" + v.Name,
			Evidence: append(s.subsystemEvidence(v.Name, ev),
				s.domains.contextEvidence(v.Name)...),
		})
	}
	return skipped
}

// subsystemEvidence describes a subsystem for a reviewer: its packages, the
// roles its types play, and how many subsystems depend on it.
func (s *subject) subsystemEvidence(sub string, ev []evidence) []string {
	var pkgs []string
	for _, pkg := range s.p.Packages {
		if s.subsystemOf(pkg.Path) == sub {
			pkgs = append(pkgs, pkg.Path)
		}
	}
	roles := map[string]int{}
	for typ, role := range s.roles {
		if t, ok := s.types[typ]; ok && s.subsystemOf(t.Package) == sub {
			roles[role]++
		}
	}
	var roleText []string
	for _, role := range slices.Sorted(maps.Keys(roles)) {
		roleText = append(roleText, fmt.Sprintf("%s %d", role, roles[role]))
	}
	downstream := map[string]bool{}
	for i := range ev {
		if ev[i].to == sub {
			downstream[ev[i].from] = true
		}
	}
	out := []string{"packages: " + strings.Join(pkgs, ", ")}
	if len(roleText) > 0 {
		out = append(out, "roles: "+strings.Join(roleText, ", "))
	}
	return append(out, fmt.Sprintf("downstream subsystems: %d", len(downstream)))
}

// commitsOf counts the distinct commits that changed any file of a
// subsystem.
func (s *subject) commitsOf(sub string) int {
	commits := map[string]bool{}
	for _, pkg := range s.p.Packages {
		if s.subsystemOf(pkg.Path) != sub {
			continue
		}
		for _, f := range pkg.Files {
			for _, c := range s.in.Churn.Commits[f] {
				commits[c] = true
			}
		}
	}
	return len(commits)
}

// churnCandidates compares each judged subdomain with the observed churn: a
// supporting or generic subsystem that changes often may be changing for
// accidental reasons, and a core one that never changes may be frozen by the
// cost of change. Both are candidates; the site presents each as only a
// possible cause (connascence.md §9.4).
func (s *subject) churnCandidates(subs []SubsystemVolatility) []Candidate {
	var out []Candidate
	for _, v := range subs {
		if v.Commits == nil {
			continue
		}
		switch {
		case (v.Subdomain == subdomainSupporting || v.Subdomain == subdomainGeneric) && *v.Commits >= highChurn:
			out = append(out, fingerprinted(&Candidate{
				Rule:    "accidental-volatility",
				Subject: v.Name,
				Message: fmt.Sprintf(
					"%s is judged %s, yet %d commits changed it",
					v.Name,
					v.Subdomain,
					*v.Commits,
				),
				Fix: "look for strong coupling at a distance that forces these changes",
			}))
		case v.Subdomain == subdomainCore && *v.Commits <= noChurn:
			out = append(out, fingerprinted(&Candidate{
				Rule:    "frozen-core",
				Subject: v.Name,
				Message: fmt.Sprintf("%s is judged core, yet no commit changed it", v.Name),
				Fix:     "ask whether the business wants changes that the cost of making them blocks",
			}))
		}
	}
	return out
}

// volatilityOrdinal is archfit's ordinal of a subdomain; an unjudged
// subsystem counts as core, the worst case.
func volatilityOrdinal(subdomain string) int {
	if subdomain == subdomainSupporting || subdomain == subdomainGeneric {
		return 3
	}
	return 10
}

// short returns the first twelve characters of a hash.
func short(hash string) string { return hash[:min(len(hash), 12)] }

// fingerprinted returns c with its fingerprint, which hashes its rule,
// subject and object as a finding's does, so a confirmed candidate and its
// finding share one.
func fingerprinted(c *Candidate) Candidate {
	c.Fingerprint = arch.NewFinding(
		arch.FindingID{Rule: c.Rule, Subject: c.Subject, Object: c.Object},
		arch.SeverityWarning,
	).Fingerprint
	return *c
}
