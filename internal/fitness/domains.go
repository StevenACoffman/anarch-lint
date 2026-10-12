package fitness

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Rule names of the domain-model conformance findings: a reflexion model,
// after Murphy, Notkin and Sullivan, comparing the concepts a modelith
// domain model declares with the code that should express them.
const (
	ruleEntityMissing   = "domain-entity-missing"
	ruleEntityAmbiguous = "domain-entity-ambiguous"
	ruleRelMissing      = "domain-relationship-missing"
	ruleEnumMissing     = "domain-enum-missing"
	ruleEnumDrift       = "domain-enum-drift"
	ruleTypeUnmapped    = "domain-type-unmapped"
	ruleContractEntity  = "domain-contract-entity"
)

// domainCheck compares one domain model with the code.
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
}

// checkDomains runs each fitness.domains rule against its domain model.
//
// Requires: in.Domains holds one model per rule, in the rules' order.
func (s *subject) checkDomains(domains []arch.DomainModel) {
	roles := arch.TypeRoles(s.p, s.m)
	for i := range s.m.Fitness.Domains {
		d := &s.m.Fitness.Domains[i]
		c := domainCheck{
			ledger: s.ledger, rule: d, model: &domains[i], roles: roles,
			types: s.types, refs: s.g.Types, constants: s.p.Facts.Constants,
			r:     s.addRule("domains", d.RuleOptions, arch.DefaultRuleName(d.Name, "domains", i)),
			names: arch.NewDomainTypes(s.p, &s.g, d),
		}
		c.r.result.Matched = len(c.model.Entities) + len(c.model.Enums)
		c.unmapped()
		matched := c.entities()
		c.relationships(matched)
		c.enums()
	}
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
				arch.FindingID{Rule: ruleTypeUnmapped, Subject: c.subjectOf(name), Object: id},
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
				arch.FindingID{Rule: ruleEntityMissing, Subject: c.subjectOf(e.Name)},
				fmt.Sprintf(
					"domain model %s declares entity %s, and no type expresses it",
					c.model.Path,
					e.Name,
				),
				"name a type after the entity, map it in the rule's types, or remove the entity from the model",
			)
		case len(ids) > 1:
			c.report(arch.FindingID{Rule: ruleEntityAmbiguous, Subject: c.subjectOf(e.Name)},
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
	c.report(arch.FindingID{Rule: ruleContractEntity, Subject: c.subjectOf(name), Object: id},
		fmt.Sprintf("%s expresses entity %s, yet plays the contract role", id, name),
		"keep the entity type in the model and give the contract its own type, or drop the role")
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
					Rule:    ruleRelMissing,
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
// from its values. A value matches a constant of the type by the
// constant's name or by its string value.
func (c *domainCheck) enums() {
	for _, e := range c.model.Enums {
		ids := c.names.Match(e.Name)
		if len(ids) != 1 {
			if len(ids) == 0 && !c.names.Mapped(e.Name) {
				c.report(
					arch.FindingID{Rule: ruleEnumMissing, Subject: c.subjectOf(e.Name)},
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
		missing, extra := c.enumDrift(ids[0], e.Values)
		if len(missing)+len(extra) == 0 {
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
			arch.FindingID{Rule: ruleEnumDrift, Subject: c.subjectOf(e.Name), Object: ids[0]},
			fmt.Sprintf("enum %s and %s disagree on %s", e.Name, ids[0],
				arch.Plural(len(missing)+len(extra), "value")),
			"make the model's values and the type's constants the same set",
			ev...)
	}
}

// enumDrift returns the model values with no constant of type id, and the
// constants of id that no model value names.
func (c *domainCheck) enumDrift(id string, values []string) (missing, extra []string) {
	used := map[string]bool{}
	names := map[string]string{} // a name or string value -> the constant
	for i := range c.constants {
		k := &c.constants[i]
		if k.Type != id {
			continue
		}
		used[k.ID] = false
		names[k.ID[strings.LastIndex(k.ID, ".")+1:]] = k.ID
		if v, err := strconv.Unquote(k.Value); err == nil {
			names[v] = k.ID
		}
	}
	for _, v := range values {
		if k, ok := names[v]; ok {
			used[k] = true
		} else {
			missing = append(missing, v)
		}
	}
	for _, k := range sortedKeys(used) {
		if !used[k] {
			extra = append(extra, k[strings.LastIndex(k, ".")+1:])
		}
	}
	slices.Sort(missing)
	return missing, extra
}
