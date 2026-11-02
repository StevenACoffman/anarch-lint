package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// An exported identifier is used when another package or an external test
// names it, or when it is part of an API another package uses: a used
// type's field type, a used function's result, a used type's enum member.
// What remains is reported; package main and methods never are.
func TestUnusedExports(t *testing.T) {
	t.Parallel()
	at := arch.Span{File: "lib/lib.go", Start: 1, End: 1}
	typ := func(name string) arch.Type {
		return arch.Type{ID: "m/lib." + name, Package: "m/lib", Name: name, Span: at}
	}
	p := arch.Program{
		Packages: []arch.Package{
			{Path: "m/app", Name: "app"},
			{Path: "m/lib", Name: "lib"},
			{Path: "m/cmd", Name: "main"},
		},
		Types: []arch.Type{
			typ("Client"), typ("Options"), typ("Level"), typ("Result"),
			typ("Tested"), typ("Unused"),
			{ID: "m/cmd.Config", Package: "m/cmd", Name: "Config"},
		},
		Funcs: []arch.Func{
			{ID: "m/lib.Dial", Package: "m/lib", Name: "Dial", Span: at},
			{ID: "m/lib.Client.Close", Package: "m/lib", Receiver: "m/lib.Client", Name: "Close"},
		},
		Refs: []arch.Ref{
			{From: "m/lib.Client", To: "m/lib.Options", Kinds: []arch.RefKind{arch.RefField}},
		},
		Facts: arch.Facts{
			Uses: []arch.Use{{
				Object: "m/lib.Dial", Kind: arch.ObjectFunc, Package: "m/lib",
				Site: arch.Site{Package: "m/app"},
			}},
			Signatures: []arch.Signature{{
				Func:    "m/lib.Dial",
				Results: []arch.TypeRef{{Named: "m/lib.Client"}, {Named: "m/lib.Level"}},
			}},
			Constants: []arch.Constant{
				{ID: "m/lib.Debug", Type: "m/lib.Level", Site: arch.Site{Package: "m/lib"}},
				{ID: "m/lib.Version", Type: "string", Site: arch.Site{Package: "m/lib"}},
			},
			TestUses: []string{"m/lib.Tested"},
		},
	}
	m := arch.Model{Version: 1, Fitness: arch.Fitness{UnusedExports: []arch.UnusedExports{{
		RuleOptions: arch.RuleOptions{Name: "api", Reason: "r"}, Packages: []string{"m/..."},
	}}}}
	r := fitness.Analyze(&arch.Input{Program: p, Model: m})
	equals(t, []string{
		"unused-export m/lib.Result api",
		"unused-export m/lib.Unused api",
		"unused-export m/lib.Version api",
	}, rules(&r))
}
