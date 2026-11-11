package schema_test

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/schema"
)

// validate checks v against the subset of JSON Schema that Generate emits:
// type, properties, required, additionalProperties, items, enum,
// propertyNames and $ref. It returns every violation, as JSON pointers.
func validate(root, s map[string]any, v any, at string) []string {
	if ref, ok := s["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/$defs/")
		def, _ := root["$defs"].(map[string]any)[name].(map[string]any)
		return validate(root, def, v, at)
	}
	if enum, ok := s["enum"].([]any); ok && !slices.Contains(enum, v) {
		return []string{fmt.Sprintf("%s: %v is not one of %v", at, v, enum)}
	}
	switch s["type"] {
	case "object":
		return validateObject(root, s, v, at)
	case "array":
		items, ok := v.([]any)
		if !ok {
			return []string{at + ": not an array"}
		}
		var out []string
		for i, item := range items {
			out = append(
				out,
				validate(root, s["items"].(map[string]any), item, fmt.Sprintf("%s/%d", at, i))...)
		}
		return out
	}
	return nil
}

func validateObject(root, s map[string]any, v any, at string) []string {
	obj, ok := v.(map[string]any)
	if !ok {
		return []string{at + ": not an object"}
	}
	var out []string
	for _, r := range asStrings(s["required"]) {
		if _, ok := obj[r]; !ok {
			out = append(out, at+": missing "+r)
		}
	}
	props, _ := s["properties"].(map[string]any)
	names, _ := s["propertyNames"].(map[string]any)
	for k, val := range obj {
		if names != nil {
			out = append(out, validate(root, names, k, at+"/"+k)...)
		}
		switch extra := s["additionalProperties"].(type) {
		case map[string]any:
			out = append(out, validate(root, extra, val, at+"/"+k)...)
			continue
		case bool:
			if p, ok := props[k]; ok {
				out = append(out, validate(root, p.(map[string]any), val, at+"/"+k)...)
			} else if !extra {
				out = append(out, at+": unknown key "+k)
			}
		}
	}
	return out
}

func asStrings(v any) []string {
	var out []string
	for _, x := range anySlice(v) {
		out = append(out, x.(string))
	}
	return out
}

func anySlice(v any) []any {
	s, _ := v.([]any)
	return s
}

// generated returns the schema as it encodes, a generic JSON value.
func generated(t *testing.T) map[string]any {
	t.Helper()
	return decoded(t, schema.Generate())
}

// decoded is s as a JSON decoder returns it, so validate reads plain maps.
func decoded(t *testing.T, s map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// The schema accepts every model the repository uses.
func TestSchemaAcceptsTheModels(t *testing.T) {
	t.Parallel()
	s := generated(t)
	for _, path := range []string{
		"../../anarch.json", "../../testdata/fx/anarch.json", "../../testdata/cx/anarch.json",
		"../../testdata/cx/fitness.json", "../../testdata/corpus/go-ddd.json",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var v any
			if err := json.Unmarshal(src, &v); err != nil {
				t.Fatal(err)
			}
			if errs := validate(s, s, v, ""); len(errs) > 0 {
				t.Fatalf("the schema rejects %s:\n%s", path, strings.Join(errs, "\n"))
			}
		})
	}
}

func TestSchemaRejects(t *testing.T) {
	t.Parallel()
	s := generated(t)
	cases := map[string]string{
		"an unknown key":               `{"version": 1, "layer": []}`,
		"a missing version":            `{"subsystems": []}`,
		"an unknown locality":          `{"version": 1, "connascence": {"maxStrength": {"galaxy": 3}}}`,
		"an unknown gate":              `{"version": 1, "fitness": {"calls": [{"from": ["x"], "callees": ["f"], "gate": "maybe"}]}}`,
		"a subsystem without packages": `{"version": 1, "subsystems": [{"name": "a"}]}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var v any
			if err := json.Unmarshal([]byte(src), &v); err != nil {
				t.Fatal(err)
			}
			if errs := validate(s, s, v, ""); len(errs) == 0 {
				t.Fatalf("the schema accepts %s", src)
			}
		})
	}
}

// The proposals schema describes a proposal's rule with the model's
// definitions, and holds only the rules a rule set may.
func TestProposalsSchema(t *testing.T) {
	t.Parallel()
	s := decoded(t, schema.GenerateProposals())
	src, err := os.ReadFile("../../testdata/cx/proposals.json")
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(src, &v); err != nil {
		t.Fatal(err)
	}
	if errs := validate(s, s, v, ""); len(errs) > 0 {
		t.Fatalf("the schema rejects testdata/cx/proposals.json:\n%s", strings.Join(errs, "\n"))
	}
	for name, src := range map[string]string{
		"a layer rule": `{"schema": "anarch-lint.proposals.v1", "source": "x", "proposals": [
			{"id": "p", "fitness": {"layerAccess": [{"layer": "a"}]}}]}`,
		"a proposal without an id": `{"schema": "anarch-lint.proposals.v1", "source": "x",
			"proposals": [{"fitness": {}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var v any
			if err := json.Unmarshal([]byte(src), &v); err != nil {
				t.Fatal(err)
			}
			if errs := validate(s, s, v, ""); len(errs) == 0 {
				t.Fatalf("the schema accepts %s", src)
			}
		})
	}
}
