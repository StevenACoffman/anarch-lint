// Package modelith reads modelith domain models (*.modelith.yaml files) into
// arch.DomainModel: the entities, relationships, enums, glossary,
// invariants and imports that anarch-lint compares with the code. It wraps
// sigs.k8s.io/yaml, the YAML library modelith itself uses, and reads only
// that subset; the rest of the format is modelith's to check, with
// `modelith lint`.
package modelith

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// The kind and version of the one format this package reads.
const (
	kind    = "DomainModel"
	version = "v1"
)

// document is the subset of a domain model this package decodes.
type document struct {
	Kind       string                     `json:"kind"`
	Version    string                     `json:"version"`
	Shared     bool                       `json:"shared"`
	Imports    []json.RawMessage          `json:"imports"`
	Glossary   map[string]json.RawMessage `json:"glossary"`
	Enums      map[string]enumDoc         `json:"enums"`
	Entities   map[string]entityDoc       `json:"entities"`
	Invariants []invariantDoc             `json:"invariants"`
}

type enumDoc struct {
	Values []struct {
		Name string `json:"name"`
	} `json:"values"`
}

type entityDoc struct {
	Relationships []struct {
		Entity    string `json:"entity"`
		Ownership string `json:"ownership"`
	} `json:"relationships"`
	Invariants []invariantDoc `json:"invariants"`
}

type invariantDoc struct {
	ID string `json:"id"`
}

// Parse reads the domain model src, the contents of the file at file. A file
// that is not YAML, or not a DomainModel of version v1, is EINVALID naming
// the file.
func Parse(file string, src []byte) (arch.DomainModel, error) {
	invalid := func(format string, args ...any) error {
		return &arch.Error{Code: arch.EINVALID, Message: file + ": " + fmt.Sprintf(format, args...)}
	}
	js, err := yaml.YAMLToJSON(src)
	if err != nil {
		return arch.DomainModel{}, invalid("not YAML: %v", err)
	}
	var doc document
	if err := json.Unmarshal(js, &doc); err != nil {
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
		Invariants: invariantsOf(&doc),
	}, nil
}

// importsOf reads the imports, each a path or {scope, path}. A bare path
// binds the scope modelith derives: the file's base name without
// .modelith.yaml or .modelith.yml.
func importsOf(raw []json.RawMessage) ([]arch.DomainImport, error) {
	out := make([]arch.DomainImport, 0, len(raw))
	for _, r := range raw {
		var bare string
		if json.Unmarshal(r, &bare) == nil {
			out = append(out, arch.DomainImport{Scope: scopeOf(bare), Path: bare})
			continue
		}
		var explicit arch.DomainImport
		if err := json.Unmarshal(r, &explicit); err != nil || explicit.Path == "" {
			return nil, errors.New("an import is neither a path nor {scope, path}")
		}
		if explicit.Scope == "" {
			explicit.Scope = scopeOf(explicit.Path)
		}
		out = append(out, explicit)
	}
	slices.SortFunc(out, func(a, b arch.DomainImport) int { return cmp.Compare(a.Scope, b.Scope) })
	return out, nil
}

// scopeOf is the scope a bare import path binds.
func scopeOf(p string) string {
	base := path.Base(p)
	for _, suffix := range []string{".modelith.yaml", ".modelith.yml"} {
		if s, ok := strings.CutSuffix(base, suffix); ok {
			return s
		}
	}
	return base
}

func enumsOf(enums map[string]enumDoc) []arch.DomainEnum {
	out := make([]arch.DomainEnum, 0, len(enums))
	for _, name := range slices.Sorted(maps.Keys(enums)) {
		e := arch.DomainEnum{Name: name, Values: make([]string, 0, len(enums[name].Values))}
		for _, v := range enums[name].Values {
			e.Values = append(e.Values, v.Name)
		}
		slices.Sort(e.Values)
		out = append(out, e)
	}
	return out
}

func entitiesOf(entities map[string]entityDoc) []arch.DomainEntity {
	out := make([]arch.DomainEntity, 0, len(entities))
	for _, name := range slices.Sorted(maps.Keys(entities)) {
		e := arch.DomainEntity{Name: name}
		for _, r := range entities[name].Relationships {
			e.Relationships = append(e.Relationships, arch.DomainRelationship{
				Entity: r.Entity, Owned: r.Ownership == "owned",
			})
		}
		slices.SortFunc(e.Relationships, func(a, b arch.DomainRelationship) int {
			return cmp.Compare(a.Entity, b.Entity)
		})
		out = append(out, e)
	}
	return out
}

// invariantsOf returns every invariant ID, the entities' and the model's.
func invariantsOf(doc *document) []string {
	var ids []string
	for _, inv := range doc.Invariants {
		ids = append(ids, inv.ID)
	}
	for _, e := range doc.Entities {
		for _, inv := range e.Invariants {
			ids = append(ids, inv.ID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}
