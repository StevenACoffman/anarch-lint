package fitness

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// directiveCheck checks the "//arch:entity" and "//arch:invariant"
// directives against the domain models of the rules that cover them.
type directiveCheck struct {
	p           *arch.Program
	rules       []arch.DomainRule
	subsystemOf map[string]string
}

// domainCheck compares one domain model with the code: a reflexion model,
// after Murphy, Notkin and Sullivan, comparing the concepts a modelith
// domain model declares with the code that should express them.
type domainCheck struct {
	ledger *ledger
	r      *rule
	rule   *arch.DomainRule
	model  *arch.DomainModel
	// types, refs and constants are the program's: its types by ID, its
	// type graph, and its constants.
	types     map[string]*arch.Type
	refs      arch.Graph
	constants []arch.Constant
	names     arch.DomainTypes
	roles     map[string]string
	// program is what the shape checks read: functions, signatures and the
	// kinds of each reference; inRule reports a package of the rule's
	// subsystem, or any package when the rule names none.
	program *arch.Program
	inRule  func(pkg string) bool
}

// checkDomains runs each fitness.domains rule against its domain model,
// with the coverage profile, nil without one, that the invariant checks
// read.
//
// Requires: domains holds one model per rule, in the rules' order.
func (s *subject) checkDomains(domains []arch.DomainModel, coverage *arch.Coverage) {
	roles := arch.TypeRoles(s.p, s.m)
	ran, writes := ranFuncs(coverage), outsideWrites(s.p.Facts.Uses)
	for i := range s.m.Fitness.Domains {
		d := &s.m.Fitness.Domains[i]
		c := domainCheck{
			ledger:    s.ledger,
			rule:      d,
			model:     &domains[i],
			roles:     roles,
			types:     s.types,
			refs:      s.g.Types,
			constants: s.p.Facts.Constants,
			r: s.addRule(
				"domains",
				d.RuleOptions,
				arch.DefaultRuleName(d.Name, "domains", i),
			),
			names:   arch.NewDomainTypes(s.p, &s.g, d),
			program: s.p,
			inRule: func(pkg string) bool {
				return d.Subsystem == "" || s.g.SubsystemAssignment.Of[pkg] == d.Subsystem
			},
		}
		c.r.result.Matched = len(c.model.Entities) + len(c.model.Enums) + len(c.model.Invariants)
		c.unmapped()
		matched := c.entities()
		c.relationships(matched)
		c.ownership(matched)
		c.attributes(matched)
		c.actions(matched)
		ic := invariantCheck{domainCheck: &c, ran: ran, writes: writes}
		ic.check(matched)
		c.enums()
	}
	dc := directiveCheck{
		p:           s.p,
		rules:       s.m.Fitness.Domains,
		subsystemOf: s.g.SubsystemAssignment.Of,
	}
	s.findings = append(s.findings, dc.unknownEntities(domains)...)
	s.findings = append(s.findings, dc.unknownInvariants(domains)...)
}

// unknownEntities reports each "//arch:entity" directive that names no
// entity of a domain model whose rule covers the type's package, so a typo
// cannot quietly fall back to matching by name. Without a domains rule
// there is nothing to check a directive against.
//
// Requires: domains holds one model per rule, in the rules' order.
func (c *directiveCheck) unknownEntities(domains []arch.DomainModel) []arch.Finding {
	if len(domains) == 0 {
		return nil
	}
	var out []arch.Finding
	for i := range c.p.Types {
		t := &c.p.Types[i]
		if t.Entity == "" || c.declares(domains, t.Package, func(m *arch.DomainModel) bool {
			return slices.ContainsFunc(m.Entities, func(e arch.DomainEntity) bool {
				return e.Name == t.Entity
			})
		}) {
			continue
		}
		f := arch.NewFinding(arch.FindingID{
			Rule: arch.RuleUnknownEntity, Subject: t.ID, Object: t.Entity,
		}, arch.SeverityWarning)
		f.Location = &t.Span
		f.Message = fmt.Sprintf("%s names entity %q, which no domain model covering %s declares",
			t.ID, t.Entity, t.Package)
		f.Fix = fmt.Sprintf("declare entity %q in the domain model, or correct the directive",
			t.Entity)
		out = append(out, f)
	}
	return out
}

// unknownInvariants reports each "//arch:invariant" directive that names no
// invariant of a domain model whose rule covers the enforcer's package, so a
// typo cannot leave an invariant unenforced while its code looks marked.
// Without a domains rule there is nothing to check a directive against.
//
// Requires: domains holds one model per rule, in the rules' order.
func (c *directiveCheck) unknownInvariants(domains []arch.DomainModel) []arch.Finding {
	if len(domains) == 0 {
		return nil
	}
	var out []arch.Finding
	for i := range c.p.Enforcers {
		e := &c.p.Enforcers[i]
		if c.declares(domains, e.Package, func(m *arch.DomainModel) bool {
			return slices.ContainsFunc(m.Invariants, func(inv arch.DomainInvariant) bool {
				return inv.ID == e.Invariant
			})
		}) {
			continue
		}
		f := arch.NewFinding(arch.FindingID{
			Rule: arch.RuleUnknownInvariant, Subject: e.Func, Object: e.Invariant,
		}, arch.SeverityWarning)
		f.Location = arch.SpanAt(e.Pos)
		f.Message = fmt.Sprintf("%s names invariant %q, which no domain model covering %s declares",
			e.Func, e.Invariant, e.Package)
		f.Fix = fmt.Sprintf("declare invariant %q in the domain model, or correct the directive",
			e.Invariant)
		out = append(out, f)
	}
	return out
}

// declares reports whether a domain model whose rule covers pkg has what
// has looks for.
func (c *directiveCheck) declares(
	domains []arch.DomainModel, pkg string, has func(*arch.DomainModel) bool,
) bool {
	for i := range c.rules {
		d := &c.rules[i]
		if (d.Subsystem == "" || c.subsystemOf[pkg] == d.Subsystem) && has(&domains[i]) {
			return true
		}
	}
	return false
}

// subjectOf names a model item in a finding: the model's path and the name.
func (c *domainCheck) subjectOf(name string) string { return c.model.Path + ":" + name }

// report records one violation.
func (c *domainCheck) report(id arch.FindingID, message, fix string, evidence ...string) {
	f := arch.NewFinding(id, arch.SeverityError)
	f.Message, f.Fix, f.Evidence = message, fix, evidence
	if len(evidence) == 0 {
		f.Evidence = nil
	}
	c.ledger.violation(c.r, &f)
}

// unmapped reports explicit mappings that name no type of the program.
func (c *domainCheck) unmapped() {
	for _, name := range sortedKeys(c.rule.Types) {
		if id := c.rule.Types[name]; c.types[id] == nil {
			c.report(
				arch.FindingID{
					Rule:    arch.RuleDomainTypeUnmapped,
					Subject: c.subjectOf(name),
					Object:  id,
				},
				fmt.Sprintf(
					"%s maps %s to %s, which is no type of the program",
					c.r.result.Name,
					name,
					id,
				),
				"correct the type ID, or remove the mapping if the concept has no type yet",
			)
		}
	}
}

// entities reports each entity with no type or several, and an entity type
// that also plays the contract role. It returns the type of each entity
// matched exactly once.
func (c *domainCheck) entities() map[string]string {
	matched := map[string]string{}
	for _, e := range c.model.Entities {
		ids := c.names.Match(e.Name)
		switch {
		case len(ids) == 0 && !c.names.Mapped(e.Name):
			c.report(
				arch.FindingID{Rule: arch.RuleDomainEntityMissing, Subject: c.subjectOf(e.Name)},
				fmt.Sprintf(
					"domain model %s declares entity %s, and no type expresses it",
					c.model.Path,
					e.Name,
				),
				"name a type after the entity, map it in the rule's types, or remove the entity from the model",
			)
		case len(ids) > 1:
			c.report(
				arch.FindingID{Rule: arch.RuleDomainEntityAmbiguous, Subject: c.subjectOf(e.Name)},
				fmt.Sprintf("entity %s matches %d types", e.Name, len(ids)),
				"map the entity to one type in the rule's types, or rename the others",
				ids...)
		case len(ids) == 1:
			matched[e.Name] = ids[0]
			c.contractEntity(e.Name, ids[0])
		}
	}
	return matched
}

// contractEntity reports an entity's type that plays the contract role: a
// type the domain model names is part of the model, not an integration
// contract (connascence.md §9.2).
func (c *domainCheck) contractEntity(name, id string) {
	if c.roles[id] != "contract" {
		return
	}
	c.report(
		arch.FindingID{Rule: arch.RuleDomainContractEntity, Subject: c.subjectOf(name), Object: id},
		fmt.Sprintf("%s expresses entity %s, yet plays the contract role", id, name),
		"keep the entity type in the model and give the contract its own type, or drop the role",
	)
}

// relationships reports each relationship between two matched entities of
// this model with no type reference either way. A qualified relationship,
// to scope.Entity of another model, is left to that model's rule.
func (c *domainCheck) relationships(matched map[string]string) {
	for _, e := range c.model.Entities {
		from, ok := matched[e.Name]
		if !ok {
			continue
		}
		for _, rel := range e.Relationships {
			to, ok := matched[rel.Entity]
			if !ok || strings.Contains(rel.Entity, ".") || from == to {
				continue
			}
			if c.refs.Weight(from, to) > 0 || c.refs.Weight(to, from) > 0 {
				continue
			}
			c.report(
				arch.FindingID{
					Rule:    arch.RuleDomainRelationship,
					Subject: c.subjectOf(e.Name),
					Object:  rel.Entity,
				},
				fmt.Sprintf("the model relates %s to %s, but %s and %s never refer to each other",
					e.Name, rel.Entity, from, to),
				"add the reference the relationship needs, or correct the model",
			)
		}
	}
}

// enums reports each enum with no type, and each whose Go constants differ
// from its values, or whose integer constants run in another order than its
// values (enumMatch says how a value matches a constant).
func (c *domainCheck) enums() {
	for _, e := range c.model.Enums {
		ids := c.names.Match(e.Name)
		if len(ids) != 1 {
			if len(ids) == 0 && !c.names.Mapped(e.Name) {
				c.report(
					arch.FindingID{Rule: arch.RuleDomainEnumMissing, Subject: c.subjectOf(e.Name)},
					fmt.Sprintf(
						"domain model %s declares enum %s, and no type expresses it",
						c.model.Path,
						e.Name,
					),
					"name a type after the enum, or map it in the rule's types",
				)
			}
			continue
		}
		matched, missing, extra := c.enumMatch(ids[0], e.Values)
		if len(missing)+len(extra) == 0 {
			c.enumOrder(&e, ids[0], matched)
			continue
		}
		var ev []string
		for _, v := range missing {
			ev = append(ev, "in the model only: "+v)
		}
		for _, v := range extra {
			ev = append(ev, "in the code only: "+v)
		}
		c.report(
			arch.FindingID{
				Rule:    arch.RuleDomainEnumDrift,
				Subject: c.subjectOf(e.Name),
				Object:  ids[0],
			},
			fmt.Sprintf("enum %s and %s disagree on %s", e.Name, ids[0],
				arch.Plural(len(missing)+len(extra), "value")),
			"make the model's values and the type's constants the same set",
			ev...)
	}
}

// enumMatch matches the model's values with the constants of type id, by
// the constant's name, its name with the type's name as a prefix removed
// and case ignored (LocalityFunction for "function"), or its string value.
// It returns each matched value's constant, the values with none, and the
// constants no value names.
func (c *domainCheck) enumMatch(id string, values []string) (
	matched map[string]*arch.Constant, missing, extra []string,
) {
	names, folded := c.enumConstants(id)
	used := map[string]bool{}
	for _, k := range names {
		used[k.ID] = false
	}
	matched = map[string]*arch.Constant{}
	for _, v := range values {
		k, ok := names[v]
		if !ok {
			k, ok = folded[normalized(v)]
		}
		if !ok {
			missing = append(missing, v)
			continue
		}
		used[k.ID], matched[v] = true, k
	}
	for _, k := range sortedKeys(used) {
		if !used[k] {
			extra = append(extra, k[strings.LastIndex(k, ".")+1:])
		}
	}
	slices.Sort(missing)
	return matched, missing, extra
}

// enumConstants indexes the constants of type id by their name and string
// value, and by their name with the type's name as a prefix removed,
// normalized.
func (c *domainCheck) enumConstants(id string) (names, folded map[string]*arch.Constant) {
	typeName := id[strings.LastIndex(id, ".")+1:]
	names, folded = map[string]*arch.Constant{}, map[string]*arch.Constant{}
	for i := range c.constants {
		k := &c.constants[i]
		if k.Type != id {
			continue
		}
		name := k.ID[strings.LastIndex(k.ID, ".")+1:]
		names[name] = k
		if rest, ok := strings.CutPrefix(name, typeName); ok && rest != "" {
			folded[normalized(rest)] = k
		}
		if v, err := strconv.Unquote(k.Value); err == nil {
			names[v] = k
		}
	}
	return names, folded
}
