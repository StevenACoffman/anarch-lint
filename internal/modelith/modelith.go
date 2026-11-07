// Package modelith reads modelith domain models (*.modelith.yaml files) into
// arch.DomainModel: the entities with their relationships, attributes and
// actions, the enums with their values in declared order, the glossary,
// the invariants and the imports that anarch-lint compares with the code. It
// parses through github.com/StevenACoffman/indomain's model package, the
// reader codetospec uses too, so both read one format one way; decoding is
// strict, so a field the format does not have is an error. Checking the
// rest of the format is modelith's job, with `modelith lint`.
package modelith

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/indomain/pkg/model"
)

// lintHint ends the message of an invalid model: anarch-lint decodes a
// model strictly but does not lint it, and modelith's linter names every
// problem of the file, not only the first one decoding meets.
const lintHint = " (check the file with `modelith lint`)"

// The kind and version of the one format this package reads.
const (
	kind    = "DomainModel"
	version = "v1"
)

// Parse reads the domain model src, the contents of the file at file. A file
// that is not YAML, has a field the format lacks, or is not a DomainModel of
// version v1, is EINVALID naming the file and pointing to `modelith lint`,
// which checks what this strict decoding does not.
func Parse(file string, src []byte) (arch.DomainModel, error) {
	invalid := func(format string, args ...any) error {
		return &arch.Error{
			Code:    arch.EINVALID,
			Message: file + ": " + fmt.Sprintf(format, args...) + lintHint,
		}
	}
	doc, err := model.Parse(src)
	if err != nil {
		return arch.DomainModel{}, invalid("not a domain model: %v", err)
	}
	if doc.Kind != kind || doc.Version != version {
		return arch.DomainModel{}, invalid(
			"kind %q version %q is not %s %s",
			doc.Kind,
			doc.Version,
			kind,
			version,
		)
	}
	imports, err := importsOf(doc.Imports)
	if err != nil {
		return arch.DomainModel{}, invalid("%v", err)
	}
	return arch.DomainModel{
		Path:       file,
		Shared:     doc.Shared,
		Imports:    imports,
		Glossary:   slices.Sorted(maps.Keys(doc.Glossary)),
		Enums:      enumsOf(doc.Enums),
		Entities:   entitiesOf(doc.Entities),
		Invariants: invariantsOf(doc),
	}, nil
}

// importsOf returns the imports sorted by scope. A bare path binds the scope
// indomain derives from it: the base name without .modelith.yaml,
// .modelith.yml or its extension. An import without a path is an error.
func importsOf(imports []model.Import) ([]arch.DomainImport, error) {
	out := make([]arch.DomainImport, 0, len(imports))
	for _, imp := range imports {
		if imp.Path == "" {
			return nil, errors.New("an import names no path")
		}
		scope := imp.Scope
		if scope == "" {
			scope = model.ScopeFromPath(imp.Path)
		}
		out = append(out, arch.DomainImport{Scope: scope, Path: imp.Path})
	}
	slices.SortFunc(out, func(a, b arch.DomainImport) int { return cmp.Compare(a.Scope, b.Scope) })
	return out, nil
}

// enumsOf returns the enums by name, each with its values in the order the
// model declares them: an order such as Low, Mid, High can mean something.
func enumsOf(enums map[string]model.Enum) []arch.DomainEnum {
	out := make([]arch.DomainEnum, 0, len(enums))
	for _, name := range slices.Sorted(maps.Keys(enums)) {
		e := arch.DomainEnum{Name: name, Values: make([]string, 0, len(enums[name].Values))}
		for _, v := range enums[name].Values {
			e.Values = append(e.Values, v.Name)
		}
		out = append(out, e)
	}
	return out
}

// entitiesOf returns the entities by name, each with its relationships
// sorted by entity, and its attributes and actions in declared order.
func entitiesOf(entities map[string]model.Entity) []arch.DomainEntity {
	out := make([]arch.DomainEntity, 0, len(entities))
	for _, name := range slices.Sorted(maps.Keys(entities)) {
		doc := entities[name]
		e := arch.DomainEntity{Name: name}
		for _, r := range doc.Relationships {
			e.Relationships = append(e.Relationships, arch.DomainRelationship{
				Entity: r.Entity, Owned: r.Ownership == "owned",
			})
		}
		slices.SortFunc(e.Relationships, func(a, b arch.DomainRelationship) int {
			return cmp.Compare(a.Entity, b.Entity)
		})
		for _, a := range doc.Attributes {
			if !a.Derived {
				e.Attributes = append(e.Attributes, a.Name)
			}
		}
		for _, a := range doc.Actions {
			e.Actions = append(e.Actions, a.Name)
		}
		out = append(out, e)
	}
	return out
}

// invariantsOf returns every invariant, the entities' and the model's, by
// ID, each with the actions that preserve it. modelith lint rejects a
// duplicate ID; should one get through, the first by entity name is kept.
func invariantsOf(doc *model.Model) []arch.DomainInvariant {
	preservedBy := map[string][]string{}
	var out []arch.DomainInvariant
	for _, inv := range doc.Invariants {
		out = append(out, arch.DomainInvariant{ID: inv.ID, Statement: inv.Statement})
	}
	for _, name := range slices.Sorted(maps.Keys(doc.Entities)) {
		e := doc.Entities[name]
		for _, inv := range e.Invariants {
			out = append(
				out,
				arch.DomainInvariant{ID: inv.ID, Statement: inv.Statement, Entity: name},
			)
		}
		for _, a := range e.Actions {
			for _, id := range a.Preserves {
				preservedBy[id] = append(preservedBy[id], name+"."+a.Name)
			}
		}
	}
	slices.SortStableFunc(
		out,
		func(a, b arch.DomainInvariant) int { return cmp.Compare(a.ID, b.ID) },
	)
	out = slices.CompactFunc(out, func(a, b arch.DomainInvariant) bool { return a.ID == b.ID })
	for i := range out {
		out[i].PreservedBy = slices.Sorted(slices.Values(preservedBy[out[i].ID]))
	}
	return out
}
