package arch_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestExpand: a subsystem template becomes one subsystem per captured area,
// with its interface and owner filled in; the expanded model hashes as the
// declared one, and a cycle between two areas is found with no hand-written
// subsystem list.
func TestExpand(t *testing.T) {
	t.Parallel()
	m := arch.Model{Version: 1, Subsystems: []arch.Subsystem{
		{
			Name: "$1", Packages: []string{"m/app/$*/..."},
			Interface: []string{"m/app/$1/api"}, Owner: "@team-$1",
		},
		{Name: "lib", Packages: []string{"m/lib"}},
		{Name: "gone-$1", Packages: []string{"m/gone/$*/..."}},
	}}
	pkgs := []string{"m/app/billing", "m/app/billing/api", "m/app/shop", "m/app/shop/api", "m/lib"}
	expanded := m.Expand(pkgs)
	equals(t, []arch.Subsystem{
		{
			Name: "billing", Packages: []string{"m/app/billing/..."},
			Interface: []string{"m/app/billing/api"}, Owner: "@team-billing",
		},
		{
			Name: "shop", Packages: []string{"m/app/shop/..."},
			Interface: []string{"m/app/shop/api"}, Owner: "@team-shop",
		},
		{Name: "lib", Packages: []string{"m/lib"}},
		{Name: "gone-$1", Packages: []string{"m/gone/$*/..."}},
	}, expanded.Subsystems)
	equals(t, m.Hash(), expanded.Hash())
	equals(t, 3, len(m.Subsystems)) // m is not modified

	var p arch.Program
	for _, path := range pkgs {
		p.Packages = append(p.Packages, arch.Package{Path: path})
	}
	p.Imports = []arch.Import{
		{
			From: "m/app/billing",
			To:   "m/app/shop/api",
		},
		{From: "m/app/shop", To: "m/app/billing/api"},
	}
	r := arch.Analyze(&arch.Input{Program: p, Model: expanded})
	equals(t, 1, len(findingsOf(&r, arch.RuleSubsystemCycle)))
}

func TestDecodeSubsystemTemplates(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		input   string
		message string // "" when the model is valid
	}{
		"a template": {
			input: `{"version": 1, "subsystems": [{"name": "$1", "packages": ["m/$*/..."],
				"interface": ["m/$1/api"]}]}`,
		},
		"a reference past the captures": {
			input: `{"version": 1, "subsystems": [{"name": "$2", "packages": ["m/$*/..."]}]}`,
			message: `model: unknown-reference: subsystem "$2" refers to $2, but package ` +
				`pattern "m/$*/..." captures 1`,
		},
		"a reference outside a template": {
			input: `{"version": 1, "subsystems": [{"name": "m", "packages": ["m/$*/..."],
				"interface": ["m/$1/api"]}]}`,
			message: `model: unknown-reference: subsystem "m" refers to $1 in "m/$1/api", but ` +
				`its name refers to no capture, so it is not a template`,
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
			equals(t, tc.message, arch.ErrorMessage(err))
		})
	}
}
