package arch_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

func TestDecodeModel(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		input   string
		message string // "" when the model is valid
	}{
		"minimal": {input: `{"version": 1}`},
		"wrong version": {
			input:   `{"version": 2}`,
			message: "model: unsupported-version: version 2 is not supported; want 1",
		},
		"unknown field": {
			input:   `{"version": 1, "layer": []}`,
			message: `model: malformed: json: unknown field "layer"`,
		},
		"duplicate layer": {
			input:   `{"version": 1, "layers": [{"name": "a"}, {"name": "a"}]}`,
			message: `model: duplicate-name: layer "a" is declared twice`,
		},
		"duplicate subsystem reported before duplicate layer": {
			input: `{"version": 1, "subsystems": [{"name": "s"}, {"name": "s"}],
				"layers": [{"name": "a"}, {"name": "a"}]}`,
			message: `model: duplicate-name: subsystem "s" is declared twice`,
		},
		"unknown subsystem in order": {
			input:   `{"version": 1, "subsystemOrder": ["x"]}`,
			message: `model: unknown-reference: subsystemOrder names unknown subsystem "x"`,
		},
		"isolation refers past its captures": {
			input: `{"version": 1, "isolations": [{"from": "x/$*/...",
				"mayImport": ["x/$2/..."], "reason": "r"}]}`,
			message: `model: unknown-reference: isolation "x/$*/...": mayImport "x/$2/..." refers to $2, but from captures 1`,
		},
		"gate above 100": {
			input:   `{"version": 1, "gates": {"minCoverage": 101}}`,
			message: "model: out-of-range: gate minCoverage 101 is outside 0–100",
		},
		"negative threshold": {
			input:   `{"version": 1, "thresholds": {"largeFuncLines": -1}}`,
			message: "model: out-of-range: threshold largeFuncLines is negative",
		},
		"a $schema key": {
			input: `{"$schema": "https://github.com/StevenACoffman/anarch-lint/schema/anarch.v1.json", "version": 1}`,
		},
		"connascence limits": {
			input: `{"version": 1, "connascence": {"maxStrength": {"system": 3},
				"maxDegree": {"system": 20}}, "balance": {"messaging": ["x/..."]}}`,
		},
		"unknown locality": {
			input:   `{"version": 1, "connascence": {"maxStrength": {"galaxy": 3}}}`,
			message: `model: unknown-value: "galaxy" is not a locality`,
		},
		"strength above 9": {
			input:   `{"version": 1, "connascence": {"maxStrength": {"package": 10}}}`,
			message: "model: out-of-range: connascence maxStrength package 10 is outside 1–9",
		},
		"degree below 1": {
			input:   `{"version": 1, "connascence": {"maxDegree": {"system": 0}}}`,
			message: "model: out-of-range: connascence maxDegree system 0 is below 1",
		},
		"a history window": {
			input: `{"version": 1, "balance": {"history": {"commits": 200, "maxHotspot": 500}}}`,
		},
		"a negative history window": {
			input:   `{"version": 1, "balance": {"history": {"commits": -1}}}`,
			message: "model: out-of-range: balance history has a negative commits or maxHotspot",
		},
		"a hotspot limit without a window": {
			input:   `{"version": 1, "balance": {"history": {"maxHotspot": 500}}}`,
			message: "model: incomplete-rule: balance history sets maxHotspot without commits",
		},
		"empty messaging pattern": {
			input:   `{"version": 1, "balance": {"messaging": [""]}}`,
			message: "model: bad-pattern: balance messaging has an empty pattern",
		},
		"fitness rules": {
			input: `{"version": 1, "layers": [{"name": "a"}, {"name": "b"}],
				"fitness": {"strictLayers": true, "layerAccess": [{"layer": "a", "mayOnlyAccess": ["b"]}],
				"slices": [{"pattern": "x/$*/...", "freeOfCycles": true}],
				"calls": [{"from": ["x/..."], "callees": ["time.Now"], "gate": "warn"}]}}`,
		},
		"fitness unknown layer": {
			input:   `{"version": 1, "fitness": {"layerAccess": [{"layer": "z"}]}}`,
			message: `model: unknown-reference: fitness layerAccess names unknown layer "z"`,
		},
		"fitness unknown gate": {
			input:   `{"version": 1, "fitness": {"calls": [{"from": ["x"], "callees": ["f"], "gate": "maybe"}]}}`,
			message: `model: unknown-value: fitness gate "maybe" is not fail, warn or off`,
		},
		"fitness slice without capture": {
			input:   `{"version": 1, "fitness": {"slices": [{"pattern": "x/..."}]}}`,
			message: `model: bad-pattern: fitness slices pattern "x/..." captures nothing; use $* or $**`,
		},
		"fitness duplicate name": {
			input: `{"version": 1, "fitness": {"calls": [{"name": "c", "from": ["x"], "callees": ["f"]}],
				"interfacesOnly": [{"name": "c", "from": ["x"], "to": ["y"]}]}}`,
			message: `model: duplicate-name: fitness rule "c" is declared twice`,
		},
		"fitness calls without from": {
			input:   `{"version": 1, "fitness": {"calls": [{"name": "c", "callees": ["f"]}]}}`,
			message: `model: incomplete-rule: fitness calls rule "c" needs callees, and from unless only is set`,
		},
		"fitness fields without fields": {
			input:   `{"version": 1, "fitness": {"fields": [{"name": "f", "from": ["x"]}]}}`,
			message: `model: incomplete-rule: fitness fields rule "f" needs fields, and from unless only is set`,
		},
		"fitness fields with a bare name": {
			input: `{"version": 1, "fitness": {"fields": [{"name": "f", "from": ["x"],
				"fields": ["conn"]}]}}`,
			message: `model: bad-pattern: fitness fields rule "f": "conn" is not pkg.Type.Field, pkg.Type.* or pkg.*`,
		},
		"fitness coChange without require": {
			input:   `{"version": 1, "fitness": {"coChange": [{"name": "c", "when": ["api/**"]}]}}`,
			message: `model: incomplete-rule: fitness coChange rule "c" needs when and require`,
		},
		"fitness coChange with a malformed glob": {
			input: `{"version": 1, "fitness": {"coChange": [{"name": "c", "when": ["api/[x"],
				"require": ["gen/**"]}]}}`,
			message: `model: bad-pattern: fitness coChange rule "c" has malformed glob "api/[x"`,
		},
		"fitness graphql without a schema": {
			input:   `{"version": 1, "fitness": {"graphql": [{"name": "g"}]}}`,
			message: `model: incomplete-rule: fitness graphql rule "g" names no schema`,
		},
		"fitness graphql with a malformed glob": {
			input: `{"version": 1, "fitness": {"graphql": [{"name": "g",
				"schema": ["graph/[x"]}]}}`,
			message: `model: bad-pattern: fitness graphql rule "g" has malformed glob "graph/[x"`,
		},
		"fitness protobuf without files": {
			input:   `{"version": 1, "fitness": {"protobuf": [{"name": "p"}]}}`,
			message: `model: incomplete-rule: fitness protobuf rule "p" names no files`,
		},
		"fitness protobuf with a malformed glob": {
			input: `{"version": 1, "fitness": {"protobuf": [{"name": "p",
				"files": ["proto/[x"]}]}}`,
			message: `model: bad-pattern: fitness protobuf rule "p" has malformed glob "proto/[x"`,
		},
		"fitness boundaries without uses": {
			input:   `{"version": 1, "fitness": {"boundaries": [{"name": "b", "files": ["web/**"]}]}}`,
			message: `model: incomplete-rule: fitness boundaries rule "b" needs files and uses`,
		},
		"fitness strict layers with one layer": {
			input:   `{"version": 1, "layers": [{"name": "a"}], "fitness": {"strictLayers": true}}`,
			message: "model: incomplete-rule: fitness strictLayers needs at least two layers",
		},
		"unknown variant": {
			input:   `{"version": 1, "thresholds": {"complexityVariant": "fancy"}}`,
			message: `model: unknown-value: complexityVariant "fancy" is not plain, modified or extended`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := arch.DecodeModel(strings.NewReader(tc.input))
			if tc.message == "" {
				equals(t, nil, err)
				return
			}
			equals(t, arch.EINVALID, arch.ErrorCode(err))
			equals(t, tc.message, arch.ErrorMessage(err))
		})
	}
}

func TestModelLimitsFillDefaults(t *testing.T) {
	t.Parallel()
	m := arch.Model{Thresholds: arch.Thresholds{LargeFuncLines: 80}}
	equals(t, arch.Thresholds{
		LargeTypeLines:    400,
		LargeFuncLines:    80,
		LargePackageTypes: 40,
		HighComplexity:    15,
		ComplexityVariant: arch.CCModified,
		MaxTypeCycleGroup: 5,
		LargeFileLines:    800,
		MaxNesting:        4,
		HighCognitive:     15,
	}, m.Limits())
}

// A model written before gates existed must hash as it did, or every
// baseline recorded under it would stop being comparable.
func TestModelWithoutGatesOmitsThem(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ok(t, arch.WriteJSON(&buf, arch.Model{Version: 1}))
	assert(
		t,
		!strings.Contains(buf.String(), "gates"),
		"an unset gates field was encoded:\n"+buf.String(),
	)
}

func TestModelHashIsStable(t *testing.T) {
	t.Parallel()
	a := arch.Model{Version: 1, Layers: []arch.Layer{{Name: "x", Packages: []string{"p/..."}}}}
	b := arch.Model{Version: 1, Layers: []arch.Layer{{Name: "x", Packages: []string{"p/..."}}}}
	equals(t, a.Hash(), b.Hash())
	b.Layers[0].Name = "y"
	assert(t, a.Hash() != b.Hash(), "a changed model kept its hash")
}

// FuzzDecodeModel checks that no input crashes the decoder, and that whatever
// it accepts re-encodes and decodes to the same model.
func FuzzDecodeModel(f *testing.F) {
	f.Add(`{"version": 1}`)
	f.Add(`{"version": 1, "subsystems": [{"name": "a", "packages": ["x/..."]}]}`)
	f.Add(`{"version": 1, "thresholds": {"complexityVariant": "plain"}}`)
	f.Fuzz(func(t *testing.T, input string) {
		m, err := arch.DecodeModel(strings.NewReader(input))
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err := arch.WriteJSON(&buf, m); err != nil {
			t.Fatalf("encode: %v", err)
		}
		again, err := arch.DecodeModel(&buf)
		if err != nil {
			t.Fatalf("re-decode: %v", err)
		}
		if m.Hash() != again.Hash() {
			t.Fatalf("round trip changed the model")
		}
	})
}

// The new sections must not disturb existing baselines, and a map's insertion
// order must not reach the hash.
func TestModelHashOfSections(t *testing.T) {
	t.Parallel()
	plain := arch.Model{Version: 1}
	empty := arch.Model{Version: 1, Connascence: arch.ConnascenceLimits{}, Fitness: arch.Fitness{}}
	equals(t, plain.Hash(), empty.Hash())

	a := arch.Model{
		Version:     1,
		Connascence: arch.ConnascenceLimits{MaxStrength: map[arch.Locality]int{}},
	}
	a.Connascence.MaxStrength[arch.LocalitySystem] = 3
	a.Connascence.MaxStrength[arch.LocalityPackage] = 9
	b := arch.Model{
		Version:     1,
		Connascence: arch.ConnascenceLimits{MaxStrength: map[arch.Locality]int{}},
	}
	b.Connascence.MaxStrength[arch.LocalityPackage] = 9
	b.Connascence.MaxStrength[arch.LocalitySystem] = 3
	equals(t, a.Hash(), b.Hash())
	assert(t, a.Hash() != plain.Hash(), "a connascence section changes the hash")

	var buf bytes.Buffer
	ok(t, arch.WriteJSON(&buf, &a))
	assert(t, strings.Contains(buf.String(), `"maxStrength": {
      "package": 9,
      "system": 3
    }`), "localities encode by name, sorted: "+buf.String())
}

func TestConnascenceLimitDefaults(t *testing.T) {
	t.Parallel()
	var none arch.ConnascenceLimits
	cases := map[arch.Locality]int{
		arch.LocalityFunction:  9,
		arch.LocalityPackage:   9,
		arch.LocalitySubsystem: 5,
		arch.LocalitySystem:    3,
		arch.LocalityExternal:  2,
	}
	for l, want := range cases {
		t.Run(l.String(), func(t *testing.T) {
			t.Parallel()
			equals(t, want, none.StrengthLimit(l))
		})
	}
	set := arch.ConnascenceLimits{MaxStrength: map[arch.Locality]int{arch.LocalitySystem: 1}}
	equals(t, 1, set.StrengthLimit(arch.LocalitySystem))
	_, limited := none.DegreeLimit(arch.LocalitySystem)
	equals(t, false, limited)
}

// The hash ignores what keeps the model's meaning: the order of a set, the
// $schema key and a threshold at its default; it sees the order of the
// layers, which is the meaning.
func TestHashIgnoresOrderOfSets(t *testing.T) {
	t.Parallel()
	decode := func(src string) arch.Model {
		t.Helper()
		m, err := arch.DecodeModel(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	base := decode(`{"version": 1,
		"subsystems": [{"name": "a", "packages": ["m/a", "m/x"]}, {"name": "b", "packages": ["m/b"]}],
		"layers": [{"name": "top", "packages": ["m/a"]}, {"name": "bottom", "packages": ["m/b"]}],
		"forbiddenImports": [{"from": ["m/a"], "imports": ["os", "time"], "reason": "r"},
			{"from": ["m/b"], "imports": ["net"], "reason": "s"}]}`)
	same := decode(`{"$schema": "x", "version": 1, "thresholds": {"largeFuncLines": 60},
		"subsystems": [{"name": "b", "packages": ["m/b"]}, {"name": "a", "packages": ["m/x", "m/a"]}],
		"layers": [{"name": "top", "packages": ["m/a"]}, {"name": "bottom", "packages": ["m/b"]}],
		"forbiddenImports": [{"from": ["m/b"], "imports": ["net"], "reason": "s"},
			{"from": ["m/a"], "imports": ["time", "os"], "reason": "r"}]}`)
	reordered := decode(`{"version": 1,
		"subsystems": [{"name": "a", "packages": ["m/a", "m/x"]}, {"name": "b", "packages": ["m/b"]}],
		"layers": [{"name": "bottom", "packages": ["m/b"]}, {"name": "top", "packages": ["m/a"]}],
		"forbiddenImports": [{"from": ["m/a"], "imports": ["os", "time"], "reason": "r"},
			{"from": ["m/b"], "imports": ["net"], "reason": "s"}]}`)
	equals(t, base.Hash(), same.Hash())
	assert(t, base.Hash() != reordered.Hash(), "the order of the layers is not hashed")
}
