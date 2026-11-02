package fitness

import (
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// ownership reports an owned relationship the code does not keep: the
// owner's type holds no field of the owned entity's type, or another
// entity's type holds one too, so the owned entity is shared rather than
// part of its owner (modelith's ownership: owned is composition).
func (c *domainCheck) ownership(matched map[string]string) {
	holds := c.containment()
	for _, e := range c.model.Entities {
		owner, ok := matched[e.Name]
		if !ok {
			continue
		}
		for _, rel := range e.Relationships {
			owned, ok := matched[rel.Entity]
			if !rel.Owned || !ok || owner == owned {
				continue
			}
			c.ownedBy(e.Name, rel.Entity, owner, owned, holds, matched)
		}
	}
}

// ownedBy checks one owned relationship from entity to part, whose types are
// owner and owned.
func (c *domainCheck) ownedBy(
	entity, part, owner, owned string, holds map[[2]string]bool, matched map[string]string,
) {
	id := arch.FindingID{
		Rule: arch.RuleDomainOwnership, Subject: c.subjectOf(entity), Object: part,
	}
	// Types that never refer to each other are relationships() to report.
	related := c.refs.Weight(owner, owned) > 0 || c.refs.Weight(owned, owner) > 0
	if related && !holds[[2]string{owner, owned}] {
		c.report(id,
			fmt.Sprintf("the model says %s owns %s, but %s holds no field of %s",
				entity, part, owner, owned),
			"give the owner a field of the owned type, or relate them without ownership")
	}
	for _, other := range sortedKeys(matched) {
		if typ := matched[other]; other != entity && other != part && holds[[2]string{typ, owned}] {
			id.Object = part + " shared by " + other
			c.report(id,
				fmt.Sprintf("the model says %s owns %s, yet %s of entity %s holds one too",
					entity, part, typ, other),
				"let the other entity refer to the owner instead, or relate them without ownership")
		}
	}
}

// containment returns which type holds a field of, or embeds, which other.
func (c *domainCheck) containment() map[[2]string]bool {
	out := map[[2]string]bool{}
	for _, r := range c.program.Refs {
		if slices.Contains(r.Kinds, arch.RefField) || slices.Contains(r.Kinds, arch.RefEmbed) {
			out[[2]string{r.From, r.To}] = true
		}
	}
	return out
}

// attributes reports each stored attribute of an entity whose struct type
// has no field of that name, ignoring case and the separators "_", "-" and
// " ". A type that is not a struct has no fields to compare.
func (c *domainCheck) attributes(matched map[string]string) {
	for _, e := range c.model.Entities {
		t := c.types[matched[e.Name]]
		if t == nil || t.Kind != arch.KindStruct {
			continue
		}
		fields := map[string]bool{}
		for _, f := range t.Fields {
			fields[normalized(f)] = true
		}
		for _, a := range e.Attributes {
			if !fields[normalized(a)] {
				c.report(
					arch.FindingID{
						Rule:    arch.RuleDomainAttributeMissing,
						Subject: c.subjectOf(e.Name),
						Object:  a,
					},
					fmt.Sprintf(
						"entity %s has attribute %s, but %s has no such field",
						e.Name,
						a,
						t.ID,
					),
					"add the field, rename it after the attribute, or correct the model",
				)
			}
		}
	}
}

// actions reports each action of an entity that is neither a method of its
// type nor a function of the rule's subsystem that takes the type, by name,
// ignoring case and separators.
func (c *domainCheck) actions(matched map[string]string) {
	takes := c.functionsTaking()
	for _, e := range c.model.Entities {
		id, ok := matched[e.Name]
		if !ok {
			continue
		}
		for _, a := range e.Actions {
			if c.hasMethod(id, a) || slices.Contains(takes[id], normalized(a)) {
				continue
			}
			c.report(
				arch.FindingID{
					Rule: arch.RuleDomainActionMissing, Subject: c.subjectOf(e.Name), Object: a,
				},
				fmt.Sprintf("entity %s has action %s, but %s has no such method and no function "+
					"takes it", e.Name, a, id),
				"add the method, rename one after the action, or correct the model",
			)
		}
	}
}

// functionsTaking maps each named type to the normalized names of the
// functions of the rule's subsystem with a parameter of that type.
func (c *domainCheck) functionsTaking() map[string][]string {
	takes := map[string][]string{}
	for i := range c.program.Facts.Signatures {
		sig := &c.program.Facts.Signatures[i]
		if !c.inRule(sig.Site.Package) {
			continue
		}
		for _, param := range sig.Params {
			if param.Named != "" {
				takes[param.Named] = append(takes[param.Named], normalized(funcName(sig.Func)))
			}
		}
	}
	return takes
}

// hasMethod reports whether the type has a method named after the action.
func (c *domainCheck) hasMethod(typ, action string) bool {
	return slices.ContainsFunc(c.program.Funcs, func(f arch.Func) bool {
		return f.Receiver == typ && normalized(f.Name) == normalized(action)
	})
}

// enumOrder reports an enum whose constants, each an integer, run in
// another order than the model's values: an iota enum whose order means
// something, such as Low, Mid, High. Constants of another kind have no
// order to compare.
//
// Requires: every value of e matched a constant (matched).
func (c *domainCheck) enumOrder(e *arch.DomainEnum, id string, matched map[string]*arch.Constant) {
	byValue := slices.Clone(e.Values)
	for _, v := range byValue {
		if _, ok := new(big.Int).SetString(matched[v].Value, 10); !ok {
			return
		}
	}
	slices.SortStableFunc(byValue, func(a, b string) int {
		x, _ := new(big.Int).SetString(matched[a].Value, 10)
		y, _ := new(big.Int).SetString(matched[b].Value, 10)
		return x.Cmp(y)
	})
	if slices.Equal(byValue, e.Values) {
		return
	}
	c.report(
		arch.FindingID{Rule: arch.RuleDomainEnumOrder, Subject: c.subjectOf(e.Name), Object: id},
		fmt.Sprintf("enum %s lists %s, but %s's constants run %s", e.Name,
			strings.Join(e.Values, ", "), id, strings.Join(byValue, ", ")),
		"declare the constants in the model's order, or reorder the model's values",
	)
}

// normalized is a name with case and the separators "_", "-" and " "
// ignored, so an attribute "created_at" names a field CreatedAt.
func normalized(name string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(name))
}

// funcName returns a function's own name from its full name.
func funcName(full string) string { return full[strings.LastIndex(full, ".")+1:] }
