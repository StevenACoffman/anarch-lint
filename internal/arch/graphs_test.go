package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// fixtureProgram is the program of mmi.md §6, as a loader would report it.
func fixtureProgram() arch.Program {
	const invoice, catalog, tax = "fx/billing/invoice", "fx/shop/catalog", "fx/billing/tax"
	types := []arch.Type{
		{ID: invoice + ".Invoice", Package: invoice, Name: "Invoice", Kind: arch.KindStruct},
		{ID: invoice + ".Line", Package: invoice, Name: "Line", Kind: arch.KindStruct},
		{ID: invoice + ".Total", Package: invoice, Name: "Total", Kind: arch.KindStruct},
		{ID: tax + ".Rate", Package: tax, Name: "Rate", Kind: arch.KindStruct},
		{ID: catalog + ".Category", Package: catalog, Name: "Category", Kind: arch.KindStruct},
		{ID: catalog + ".Item", Package: catalog, Name: "Item", Kind: arch.KindStruct},
		{ID: catalog + ".Product", Package: catalog, Name: "Product", Kind: arch.KindStruct},
	}
	var refs []arch.Ref
	for _, e := range fixtureTypes().Edges() {
		refs = append(refs, arch.Ref{From: e.From, To: e.To, Weight: e.Weight})
	}
	return arch.Program{
		Packages: []arch.Package{{Path: invoice}, {Path: tax}, {Path: catalog}},
		Files: []arch.File{
			{Path: "billing/invoice/invoice.go", Package: invoice, Lines: 6},
			{Path: "billing/tax/tax.go", Package: tax, Lines: 4},
			{Path: "shop/catalog/catalog.go", Package: catalog, Lines: 8},
		},
		Types: types,
		Refs:  refs,
		Imports: []arch.Import{
			{From: tax, To: catalog},
			{From: catalog, To: invoice},
			{From: catalog, To: "fmt"}, // outside the scope: in no graph
		},
	}
}

func fixtureModel() arch.Model {
	return arch.Model{
		Version: 1,
		Subsystems: []arch.Subsystem{
			{
				Name:      "billing",
				Packages:  []string{"fx/billing/..."},
				Interface: []string{"fx/billing/invoice"},
			},
			{Name: "shop", Packages: []string{"fx/shop/..."}},
		},
	}
}

// The ordering measures mmi.md §6 reports for its fixture.
func TestOrderingFixture(t *testing.T) {
	t.Parallel()
	o := func() arch.Ordering {
		p, m := fixtureProgram(), fixtureModel()
		g := arch.BuildGraphs(&p, &m)
		return g.Ordering()
	}()
	equals(t, 5, o.Class.InCycles)
	equals(t, 5, o.Class.Refs)
	equals(t, 1, o.Class.Direct)
	equals(t, 3, o.Class.MaxGroup)
	equals(t, 0, o.Package.InCycles)
	equals(t, 2, o.Subsystem.InCycles)
	equals(t, 2, o.Subsystem.Weight)
	equals(t, 1, o.Subsystem.Direct)
	equals(t, [][]string{{"billing", "shop"}}, o.Subsystem.Groups)
	equals(t, 0, o.RefsInClassCyclesBetweenPackages)
	equals(t, 0, o.RefsInClassCyclesBetweenSubsystems)
	equals(t, 0, o.PackageCycleGroupsFromClassCycles)
	equals(t, 0, o.SubsysCycleGroupsFromClassCycles)
}

func TestAssignment(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		subsystems []arch.Subsystem
		of         map[string]string
		unassigned []string
		ambiguous  []string
		stale      []string
	}{
		"most specific wins": {
			subsystems: []arch.Subsystem{
				{Name: "all", Packages: []string{"fx/..."}},
				{Name: "tax", Packages: []string{"fx/billing/tax"}},
			},
			of: map[string]string{
				"fx/billing/invoice": "all", "fx/billing/tax": "tax", "fx/shop/catalog": "all",
			},
		},
		"exact beats prefix of the same root": {
			subsystems: []arch.Subsystem{
				{Name: "tree", Packages: []string{"fx/billing/tax/..."}},
				{Name: "leaf", Packages: []string{"fx/billing/tax"}},
			},
			of:         map[string]string{"fx/billing/tax": "leaf"},
			unassigned: []string{"fx/billing/invoice", "fx/shop/catalog"},
		},
		"equal specificity is ambiguous": {
			subsystems: []arch.Subsystem{
				{Name: "a", Packages: []string{"fx/shop/..."}},
				{Name: "b", Packages: []string{"fx/shop/..."}},
			},
			of:         map[string]string{},
			unassigned: []string{"fx/billing/invoice", "fx/billing/tax"},
			ambiguous:  []string{"fx/shop/catalog"},
		},
		"captures match like plain patterns": {
			subsystems: []arch.Subsystem{
				{Name: "area", Packages: []string{"fx/$*/..."}},
				{Name: "tax", Packages: []string{"fx/$*/tax"}},
			},
			of: map[string]string{
				"fx/billing/invoice": "area", "fx/billing/tax": "tax", "fx/shop/catalog": "area",
			},
		},
		"stale pattern": {
			subsystems: []arch.Subsystem{{Name: "gone", Packages: []string{"fx/legacy/..."}}},
			of:         map[string]string{},
			unassigned: []string{"fx/billing/invoice", "fx/billing/tax", "fx/shop/catalog"},
			stale:      []string{"gone: fx/legacy/..."},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := arch.Model{Version: 1, Subsystems: tc.subsystems}
			p := fixtureProgram()
			a := arch.BuildGraphs(&p, &m).SubsystemAssignment
			equals(t, tc.of, a.Of)
			equals(t, tc.unassigned, a.Unassigned)
			equals(t, tc.ambiguous, a.Ambiguous)
			equals(t, tc.stale, a.Stale)
		})
	}
}
