package arch

import (
	"fmt"
	"slices"
	"strings"
)

// Ways a type can be marked with its role, from the most explicit to the
// least. Every marker but markerPackage is an explicit mapping of the pattern
// for criterion 3.3.
const (
	markerDirective roleMarker = "directive"
	markerEmbedding roleMarker = "embedding"
	markerInterface roleMarker = "interface"
	markerSuffix    roleMarker = "suffix"
	markerPackage   roleMarker = "package"
)

// roleMarker says how a type was matched to its role.
type roleMarker string

// roleMatch is the role a type plays and how it was matched.
type roleMatch struct {
	role   int // position in Model.Roles
	marker roleMarker
}

// roles maps each type that plays a role to the position of its role
// (MMI 3.1).
func (s *subject) roles() map[string]int {
	roles := map[string]int{}
	for i := range s.p.Types {
		if m, ok := s.roleOf(&s.p.Types[i]); ok {
			roles[s.p.Types[i].ID] = m.role
		}
	}
	return roles
}

// TypeRoles maps each type of p that plays one of m's roles to the role's
// name (MMI 3.1); a type that plays none is left out.
func TypeRoles(p *Program, m *Model) map[string]string {
	s := &subject{p: p, m: m}
	out := map[string]string{}
	for t, r := range s.roles() {
		out[t] = m.Roles[r].Name
	}
	return out
}

// roleOf returns the role t plays: the role its directive names, or else the
// first role it matches by embedding, interface, suffix or package, in that
// order. A directive naming no declared role matches nothing; the
// unknown-role-directive rule reports it.
func (s *subject) roleOf(t *Type) (roleMatch, bool) {
	if t.Directive != "" {
		r := slices.IndexFunc(s.m.Roles, func(rl Role) bool { return rl.Name == t.Directive })
		return roleMatch{role: r, marker: markerDirective}, r >= 0
	}
	for _, m := range []roleMarker{markerEmbedding, markerInterface, markerSuffix, markerPackage} {
		for r := range s.m.Roles {
			if marks(&s.m.Roles[r], t, m) {
				return roleMatch{role: r, marker: m}, true
			}
		}
	}
	return roleMatch{}, false
}

// marks reports whether role rl matches t by the marker m.
func marks(rl *Role, t *Type, m roleMarker) bool {
	switch m {
	case markerEmbedding:
		return slices.ContainsFunc(
			rl.Embeds,
			func(id string) bool { return slices.Contains(t.Embeds, id) },
		)
	case markerInterface:
		return slices.ContainsFunc(
			rl.Implements,
			func(id string) bool { return slices.Contains(t.Implements, id) },
		)
	case markerSuffix:
		return rl.Suffix != "" && strings.HasSuffix(t.Name, rl.Suffix)
	case markerPackage:
		return MatchesAny(rl.Packages, t.Package)
	case markerDirective:
		return t.Directive == rl.Name
	default:
		return false
	}
}

// roleRule is criterion 3.2: type references that run against the role order.
func roleRule(s *subject) ruleResult {
	var r ruleResult
	if len(s.m.Roles) < 2 {
		return r
	}
	roles := s.roles()
	for _, ref := range s.p.Refs {
		rf, okFrom := roles[ref.From]
		rt, okTo := roles[ref.To]
		if !okFrom || !okTo || rf == rt {
			continue
		}
		r.candidates++
		if rf < rt {
			continue
		}
		r.violations++
		from, to := s.m.Roles[rf].Name, s.m.Roles[rt].Name
		f := NewFinding(
			FindingID{Rule: RuleRoleOrder, Subject: ref.From, Object: ref.To},
			SeverityError,
		)
		f.ModelRule = "roles"
		f.Location = SpanAt(ref.Pos)
		f.Criterion = "3.2"
		f.Message = fmt.Sprintf(
			"%s (%s) refers to %s (%s), against the role order",
			ref.From,
			from,
			ref.To,
			to,
		)
		f.Fix = fmt.Sprintf("invert the reference through an interface owned by the %s role, "+
			"or give one of the types a role that fits", from)
		r.findings = append(r.findings, f)
	}
	return r
}

// unknownRoleRule reports "//arch:role" directives that name no declared
// role, so a typo cannot silently leave a type without its role.
func unknownRoleRule(s *subject) []Finding {
	var fs []Finding
	for i := range s.p.Types {
		t := &s.p.Types[i]
		if t.Directive == "" ||
			slices.ContainsFunc(s.m.Roles, func(rl Role) bool { return rl.Name == t.Directive }) {
			continue
		}
		f := NewFinding(
			FindingID{Rule: RuleUnknownRole, Subject: t.ID, Object: t.Directive},
			SeverityWarning,
		)
		f.Criterion = "3.3"
		f.Location = &t.Span
		f.Message = fmt.Sprintf(
			"%s names role %q, which the model does not declare",
			t.ID,
			t.Directive,
		)
		f.Fix = fmt.Sprintf("declare role %q in the model, or correct the directive", t.Directive)
		fs = append(fs, f)
	}
	return fs
}

// roleEvidence is the evidence for 3.3: how many types have a role, and by
// which marker, explicit or only by their package.
func roleEvidence(s *subject) []string {
	if len(s.m.Roles) == 0 {
		return []string{"roles: none declared, so no type's pattern is mapped explicitly"}
	}
	counts := map[roleMarker]int{}
	matched := 0
	for i := range s.p.Types {
		if m, ok := s.roleOf(&s.p.Types[i]); ok {
			counts[m.marker]++
			matched++
		}
	}
	return []string{
		fmt.Sprintf(
			"roles: %d declared; of %d types, %d have a role by directive, "+
				"%d by embedding a marker, %d by implementing an interface, %d by name suffix, "+
				"%d only by package, %d none",
			len(s.m.Roles),
			len(s.p.Types),
			counts[markerDirective],
			counts[markerEmbedding],
			counts[markerInterface],
			counts[markerSuffix],
			counts[markerPackage],
			len(s.p.Types)-matched,
		),
	}
}
