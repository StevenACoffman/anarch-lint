package arch_test

import (
	"bytes"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestStarterModel: each community of packages is a subsystem named after
// its shortest path, numbered when another took the name; the order runs
// from the dependent subsystems down; and the draft is a valid model that
// claims every package once.
func TestStarterModel(t *testing.T) {
	t.Parallel()
	imp := func(from, to string) arch.Import { return arch.Import{From: from, To: to} }
	p := arch.Program{
		Packages: []arch.Package{
			{Path: "m/legacy/shop"},
			{Path: "m/legacy/shop/x"},
			{Path: "m/shop"},
			{Path: "m/shop/cart"},
			{Path: "m/shop/checkout"},
			{Path: "m/store"},
			{Path: "m/store/sql"},
			{Path: "m/tools/store"},
		},
		Imports: []arch.Import{
			imp("m/shop/cart", "m/shop"), imp("m/shop/checkout", "m/shop"),
			imp("m/shop/checkout", "m/shop/cart"), imp("m/shop", "m/store"),
			imp("m/store/sql", "m/store"), imp("m/tools/store", "m/store/sql"),
			imp("m/legacy/shop/x", "m/legacy/shop"),
		},
	}
	m := arch.StarterModel(&p)
	equals(t, []arch.Subsystem{
		{Name: "shop", Packages: []string{"m/legacy/shop", "m/legacy/shop/x"}},
		{Name: "shop-2", Packages: []string{"m/shop", "m/shop/cart", "m/shop/checkout"}},
		{Name: "store", Packages: []string{"m/store", "m/store/sql", "m/tools/store"}},
	}, m.Subsystems)
	equals(t, []string{"shop", "shop-2", "store"}, m.SubsystemOrder)
	seen := map[string]bool{}
	for _, s := range m.Subsystems {
		for _, pkg := range s.Packages {
			if seen[pkg] {
				t.Fatalf("%s is in two subsystems", pkg)
			}
			seen[pkg] = true
		}
	}
	equals(t, len(p.Packages), len(seen))
	if _, err := arch.DecodeModel(bytes.NewReader(m.Format())); err != nil {
		t.Fatalf("the draft is no valid model: %v", err)
	}
}
