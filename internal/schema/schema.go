// Package schema generates the JSON Schema of anarch.json from the Go types
// that define it, so editors can offer completion and catch typos while a
// model is written. The schema is generated, not written by hand: anarch.json
// is defined by arch.Model, and a second, hand-kept copy would drift from it
// (modelith's ADR-0009 makes the same argument against an unchecked copy).
package schema

import (
	"reflect"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// ID names the schema; a model may refer to it in its $schema key.
const ID = "https://github.com/StevenACoffman/anarch-lint/schema/anarch.v1.json"

// proposalsID names the schema of a proposals file (anarch-lint.proposals.v1).
const proposalsID = "https://github.com/StevenACoffman/anarch-lint/schema/proposals.v1.json"

// generator accumulates the definitions of the struct types it meets.
type generator struct {
	defs map[string]any
}

// Generate returns the JSON Schema (draft 2020-12) of an anarch.json model,
// as a value that encodes to the same bytes on every run.
func Generate() map[string]any {
	return generate(reflect.TypeFor[arch.Model](), ID, "anarch-lint model")
}

// GenerateProposals returns the JSON Schema (draft 2020-12) of a proposals
// file, which another tool writes for anarch-lint to read: its rules are
// described by the same definitions as the model's.
func GenerateProposals() map[string]any {
	return generate(reflect.TypeFor[arch.Proposals](), proposalsID, "anarch-lint proposals")
}

// generate returns the schema of root, named id and titled title, with the
// definitions of every struct type it reaches.
func generate(root reflect.Type, id, title string) map[string]any {
	g := generator{defs: map[string]any{}}
	s := g.object(root)
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["$id"] = id
	s["title"] = title
	s["$defs"] = g.defs
	return s
}

// closedValues returns the values each closed string type accepts.
func closedValues() map[reflect.Type][]string {
	localities := make([]string, 0, int(arch.LocalityExternal)+1)
	for l := arch.LocalityFunction; l <= arch.LocalityExternal; l++ {
		localities = append(localities, l.String())
	}
	return map[reflect.Type][]string{
		reflect.TypeFor[arch.Locality](): localities,
		reflect.TypeFor[arch.RuleGate](): {
			string(arch.GateFail),
			string(arch.GateWarn),
			string(arch.GateOff),
		},
		reflect.TypeFor[arch.QualityAttribute](): stringsOf(arch.QualityAttributes()),
		reflect.TypeFor[arch.RuleName]():         stringsOf(arch.GovernableRules()),
		reflect.TypeFor[arch.CCVariant](): {
			string(arch.CCPlain),
			string(arch.CCModified),
			string(arch.CCExtended),
		},
	}
}

// stringsOf converts a closed string type's values to strings.
func stringsOf[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

// scalarTypes maps each scalar Go kind a model uses to its JSON type.
func scalarTypes() map[reflect.Kind]string {
	return map[reflect.Kind]string{
		reflect.String: "string", reflect.Bool: "boolean",
		reflect.Int: "integer", reflect.Float64: "number",
	}
}

// schemaOf returns the schema of one Go type.
func (g *generator) schemaOf(t reflect.Type) map[string]any {
	if values, ok := closedValues()[t]; ok {
		return map[string]any{"type": "string", "enum": values}
	}
	if typ, ok := scalarTypes()[t.Kind()]; ok {
		return map[string]any{"type": typ}
	}
	switch t.Kind() {
	case reflect.Slice:
		return map[string]any{"type": "array", "items": g.schemaOf(t.Elem())}
	case reflect.Map:
		return g.mapOf(t)
	case reflect.Struct:
		return g.ref(t)
	default:
		return map[string]any{}
	}
}

// mapOf returns the schema of a map: an object whose values share one
// schema, and whose keys are closed when the key type is.
func (g *generator) mapOf(t reflect.Type) map[string]any {
	m := map[string]any{"type": "object", "additionalProperties": g.schemaOf(t.Elem())}
	if values, ok := closedValues()[t.Key()]; ok {
		m["propertyNames"] = map[string]any{"enum": values}
	}
	return m
}

// ref defines a struct type once under $defs and refers to it.
func (g *generator) ref(t reflect.Type) map[string]any {
	if _, done := g.defs[t.Name()]; !done {
		g.defs[t.Name()] = map[string]any{} // reserve the name before recursing
		g.defs[t.Name()] = g.object(t)
	}
	return map[string]any{"$ref": "#/$defs/" + t.Name()}
}

// object returns the schema of a struct type: its JSON fields, embedded
// structs flattened as encoding/json flattens them, no others allowed. A
// field without omitempty or omitzero is required unless it is a struct,
// which encodes even when zero.
func (g *generator) object(t reflect.Type) map[string]any {
	props := map[string]any{}
	var required []string
	g.fields(t, props, &required)
	slices.Sort(required)
	obj := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		obj["required"] = required
	}
	return obj
}

// fields adds t's JSON fields to props, and the required ones to required.
func (g *generator) fields(t reflect.Type, props map[string]any, required *[]string) {
	for f := range t.Fields() {
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			g.fields(f.Type, props, required)
			continue
		}
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if !f.IsExported() || name == "-" || name == "" {
			continue
		}
		props[name] = g.schemaOf(f.Type)
		optional := strings.Contains(opts, "omitempty") || strings.Contains(opts, "omitzero")
		if !optional && f.Type.Kind() != reflect.Struct {
			*required = append(*required, name)
		}
	}
}
