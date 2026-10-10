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
			message: "model: version 2 is not supported; want 1",
		},
		"unknown field": {
			input:   `{"version": 1, "layer": []}`,
			message: `model: json: unknown field "layer"`,
		},
		"duplicate layer": {
			input:   `{"version": 1, "layers": [{"name": "a"}, {"name": "a"}]}`,
			message: `model: layer "a" is declared twice`,
		},
		"duplicate subsystem reported before duplicate layer": {
			input: `{"version": 1, "subsystems": [{"name": "s"}, {"name": "s"}],
				"layers": [{"name": "a"}, {"name": "a"}]}`,
			message: `model: subsystem "s" is declared twice`,
		},
		"unknown subsystem in order": {
			input:   `{"version": 1, "subsystemOrder": ["x"]}`,
			message: `model: subsystemOrder names unknown subsystem "x"`,
		},
		"isolation refers past its captures": {
			input: `{"version": 1, "isolations": [{"from": "x/$*/...",
				"mayImport": ["x/$2/..."], "reason": "r"}]}`,
			message: `model: isolation "x/$*/...": mayImport "x/$2/..." refers to $2, but from captures 1`,
		},
		"gate above 100": {
			input:   `{"version": 1, "gates": {"minCoverage": 101}}`,
			message: "model: gate minCoverage 101 is outside 0–100",
		},
		"negative threshold": {
			input:   `{"version": 1, "thresholds": {"largeFuncLines": -1}}`,
			message: "model: threshold largeFuncLines is negative",
		},
		"unknown variant": {
			input:   `{"version": 1, "thresholds": {"complexityVariant": "fancy"}}`,
			message: `model: complexityVariant "fancy" is not plain, modified or extended`,
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
