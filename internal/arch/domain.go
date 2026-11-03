package arch

import "path"

// DomainModel is the part of a modelith domain model (a *.modelith.yaml
// file) that anarch-lint reads: the concepts a subsystem's code should
// express, which the fitness rules compare with the code and which serve as
// evidence for judgement slots (mmi-plan.md §10). It is pure data; a loader
// reads it from the file, and every slice is sorted by name.
//
// A domain model is not the architecture model, Model: it describes what the
// software is about, not how its packages are arranged.
type DomainModel struct {
	// Path is the file, as the fitness rule names it.
	Path string `json:"path"`
	// Shared marks a model that exists to be imported by others.
	Shared     bool           `json:"shared,omitempty"`
	Imports    []DomainImport `json:"imports,omitempty"`
	Glossary   []string       `json:"glossary,omitempty"`
	Enums      []DomainEnum   `json:"enums,omitempty"`
	Entities   []DomainEntity `json:"entities,omitempty"`
	Invariants []string       `json:"invariants,omitempty"`
}

// DomainTypes resolves the names of one domain rule's model to the
// program's types: the rule's explicit mapping, or the types of that name in
// the rule's subsystem, or in the whole program when it names none.
type DomainTypes struct {
	rule   *DomainRule
	exists map[string]bool
	byName map[string][]string
	// byDirective maps an entity name to the in-scope types whose
	// "//arch:entity" directive names it.
	byDirective map[string][]string
}

// DomainImport is another domain model this one refers to as Scope.Name.
type DomainImport struct {
	Scope string `json:"scope"`
	Path  string `json:"path"`
}

// DomainEnum is an enumerated type and the names of its values, in the
// order the model declares them.
type DomainEnum struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// DomainEntity is a named concept, the entities it relates to, and its
// stored attributes and actions in the order the model declares them; a
// derived attribute, computed rather than stored, is left out.
type DomainEntity struct {
	Name          string               `json:"name"`
	Relationships []DomainRelationship `json:"relationships,omitempty"`
	Attributes    []string             `json:"attributes,omitempty"`
	Actions       []string             `json:"actions,omitempty"`
}

// DomainRelationship relates an entity to another, by name: an entity of the
// same model, or "scope.Entity" of an imported one. Owned marks composition.
type DomainRelationship struct {
	Entity string `json:"entity"`
	Owned  bool   `json:"owned,omitempty"`
}

// NewDomainTypes indexes the program's types for one domain rule.
func NewDomainTypes(p *Program, g *Graphs, r *DomainRule) DomainTypes {
	d := DomainTypes{
		rule: r, exists: map[string]bool{}, byName: map[string][]string{},
		byDirective: map[string][]string{},
	}
	for i := range p.Types {
		t := &p.Types[i]
		d.exists[t.ID] = true
		if r.Subsystem == "" || g.SubsystemAssignment.Of[t.Package] == r.Subsystem {
			d.byName[t.Name] = append(d.byName[t.Name], t.ID)
			if t.Entity != "" {
				d.byDirective[t.Entity] = append(d.byDirective[t.Entity], t.ID)
			}
		}
	}
	return d
}

// Match returns the types a model name stands for: its explicit mapping when
// that names a type of the program, nothing when it names none; otherwise
// the in-scope types whose "//arch:entity" directive names it, and failing
// those the in-scope types of that name.
func (d DomainTypes) Match(name string) []string {
	if id, ok := d.rule.Types[name]; ok {
		if d.exists[id] {
			return []string{id}
		}
		return nil
	}
	if ids := d.byDirective[name]; len(ids) > 0 {
		return ids
	}
	return d.byName[name]
}

// Mapped reports a name the rule maps explicitly.
func (d DomainTypes) Mapped(name string) bool {
	_, ok := d.rule.Types[name]
	return ok
}

// EntityTypes returns, for each type that expresses exactly one entity of
// the input's domain models, that entity as "path:Name". g is the graphs of
// the input's program and model.
//
// Requires: in.Domains holds one model per fitness.domains rule, in order.
func (in *Input) EntityTypes(g *Graphs) map[string]string {
	out := map[string]string{}
	for i := range in.Model.Fitness.Domains {
		d := NewDomainTypes(&in.Program, g, &in.Model.Fitness.Domains[i])
		for _, e := range in.Domains[i].Entities {
			if ids := d.Match(e.Name); len(ids) == 1 {
				out[ids[0]] = in.Domains[i].Path + ":" + e.Name
			}
		}
	}
	return out
}

// ImportScope returns the scope under which d imports the model at file, a
// path as the fitness rule names it. An import path is relative to the
// importing model's directory, as modelith resolves it.
func (d *DomainModel) ImportScope(file string) (string, bool) {
	for _, imp := range d.Imports {
		if path.Join(path.Dir(d.Path), imp.Path) == path.Clean(file) {
			return imp.Scope, true
		}
	}
	return "", false
}
