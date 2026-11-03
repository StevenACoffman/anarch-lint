package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// shop is a program whose code expresses some of a domain model: Order
// refers to Line, Customer is declared twice, Status has a constant the
// model lacks, and Receipt plays the contract role.
func shop() arch.Program {
	typ := func(pkg, name string, kind arch.TypeKind) arch.Type {
		return arch.Type{ID: pkg + "." + name, Package: pkg, Name: name, Kind: kind}
	}
	return arch.Program{
		Packages: []arch.Package{{Path: "m/crm"}, {Path: "m/shop"}},
		Types: []arch.Type{
			typ("m/crm", "Customer", arch.KindStruct),
			typ("m/shop", "Customer", arch.KindStruct),
			typ("m/shop", "Line", arch.KindStruct),
			typ("m/shop", "Order", arch.KindStruct),
			typ("m/shop", "Receipt", arch.KindStruct),
			typ("m/shop", "State", arch.KindOther),
			typ("m/shop", "Status", arch.KindOther),
		},
		Refs: []arch.Ref{
			{
				From:   "m/shop.Order",
				To:     "m/shop.Line",
				Weight: 1,
				Kinds:  []arch.RefKind{arch.RefField},
			},
		},
		Facts: arch.Facts{Constants: []arch.Constant{
			{ID: "m/shop.Open", Type: "m/shop.Status", Value: `"open"`},
			{ID: "m/shop.Closed", Type: "m/shop.Status", Value: `"closed"`},
			{ID: "m/shop.Void", Type: "m/shop.Status", Value: `"void"`},
		}},
	}
}

func shopModel() arch.DomainModel {
	return arch.DomainModel{
		Path: "docs/shop.modelith.yaml",
		Enums: []arch.DomainEnum{
			{Name: "Status", Values: []string{"closed", "open", "refunded"}},
			{Name: "Priority", Values: []string{"high"}},
		},
		Entities: []arch.DomainEntity{
			{Name: "Customer"},
			{Name: "Invoice"},
			{Name: "Line"},
			{Name: "Order", Relationships: []arch.DomainRelationship{
				{Entity: "Line", Owned: true}, {Entity: "Receipt"}, {Entity: "payments.Method"},
			}},
			{Name: "Receipt"},
		},
	}
}

func TestDomainConformance(t *testing.T) {
	t.Parallel()
	m := arch.Model{
		Version: 1,
		Roles:   []arch.Role{{Name: "contract", Suffix: "Receipt"}},
		Fitness: arch.Fitness{Domains: []arch.DomainRule{{
			RuleOptions: arch.RuleOptions{Name: "shop"}, Path: "docs/shop.modelith.yaml",
			Types: map[string]string{"Priority": "m/shop.Missing"},
		}}},
	}
	r := fitness.Analyze(
		&arch.Input{Program: shop(), Model: m, Domains: []arch.DomainModel{shopModel()}},
	)
	const at = "docs/shop.modelith.yaml:"
	equals(t, []string{
		"domain-contract-entity " + at + "Receipt m/shop.Receipt",
		"domain-entity-ambiguous " + at + "Customer ",
		"domain-entity-missing " + at + "Invoice ",
		"domain-enum-drift " + at + "Status m/shop.Status",
		"domain-relationship-missing " + at + "Order Receipt",
		"domain-type-unmapped " + at + "Priority m/shop.Missing",
	}, rules(&r))

	var drift arch.Finding
	for _, f := range r.Findings {
		if f.Rule == "domain-enum-drift" {
			drift = f
		}
	}
	equals(t, []string{"in the model only: refunded", "in the code only: Void"}, drift.Evidence)
	equals(t, 7, r.Rules[0].Matched)
	equals(t, fitness.StatusFailed, r.Rules[0].Status)
}

// A subsystem narrows the types an entity may match.
func TestDomainConformanceWithinASubsystem(t *testing.T) {
	t.Parallel()
	m := arch.Model{
		Version:    1,
		Subsystems: []arch.Subsystem{{Name: "shop", Packages: []string{"m/shop"}}},
		Fitness: arch.Fitness{Domains: []arch.DomainRule{
			{
				RuleOptions: arch.RuleOptions{
					Name: "shop",
				},
				Path:      "docs/shop.modelith.yaml",
				Subsystem: "shop",
			},
		}},
	}
	model := arch.DomainModel{
		Path:     "docs/shop.modelith.yaml",
		Entities: []arch.DomainEntity{{Name: "Customer"}},
	}
	r := fitness.Analyze(&arch.Input{Program: shop(), Model: m, Domains: []arch.DomainModel{model}})
	equals(t, []string(nil), rules(&r))
}

// TestEntityDirectives: an "//arch:entity" directive settles which of two
// same-named types expresses an entity, and maps one whose name differs; a
// directive naming no entity of a covering model is a warning.
func TestEntityDirectives(t *testing.T) {
	t.Parallel()
	p := shop()
	for i := range p.Types {
		switch p.Types[i].ID {
		case "m/shop.Customer":
			p.Types[i].Entity = "Customer"
		case "m/shop.State":
			p.Types[i].Entity = "Invoice"
		case "m/shop.Line":
			p.Types[i].Entity = "Lien"
		}
	}
	m := arch.Model{Version: 1, Fitness: arch.Fitness{Domains: []arch.DomainRule{{
		RuleOptions: arch.RuleOptions{Name: "shop"}, Path: "docs/shop.modelith.yaml",
	}}}}
	model := arch.DomainModel{
		Path:     "docs/shop.modelith.yaml",
		Entities: []arch.DomainEntity{{Name: "Customer"}, {Name: "Invoice"}, {Name: "Line"}},
	}
	r := fitness.Analyze(&arch.Input{Program: p, Model: m, Domains: []arch.DomainModel{model}})
	equals(t, []string{"unknown-entity-directive m/shop.Line Lien"}, rules(&r))
}

// shape is a program whose Cart owns Item, though Wishlist holds an Item
// too, and whose Wishlist uses a Cart only in a method body; whose Cart has a field total and a
// method Checkout, with a function
// Empty that takes a Cart; and whose Size constants are iota values in
// another order than the model's, with the type's name as their prefix.
func shape() arch.Program {
	typ := func(name string, kind arch.TypeKind, fields ...string) arch.Type {
		return arch.Type{
			ID: "m/shop." + name, Package: "m/shop", Name: name, Kind: kind,
			Fields: fields,
		}
	}
	field := []arch.RefKind{arch.RefField}
	return arch.Program{
		Packages: []arch.Package{{Path: "m/shop"}},
		Types: []arch.Type{
			typ("Cart", arch.KindStruct, "Items", "Total"), typ("Item", arch.KindStruct),
			typ("Size", arch.KindOther), typ("Wishlist", arch.KindStruct, "Items"),
		},
		Funcs: []arch.Func{
			{
				ID:       "m/shop.Cart.Checkout",
				Package:  "m/shop",
				Receiver: "m/shop.Cart",
				Name:     "Checkout",
			},
		},
		Refs: []arch.Ref{
			{From: "m/shop.Cart", To: "m/shop.Item", Weight: 1, Kinds: field},
			{
				From:   "m/shop.Wishlist",
				To:     "m/shop.Cart",
				Weight: 1,
				Kinds:  []arch.RefKind{arch.RefBody},
			},
			{From: "m/shop.Wishlist", To: "m/shop.Item", Weight: 1, Kinds: field},
		},
		Facts: arch.Facts{
			Constants: []arch.Constant{
				{ID: "m/shop.SizeLarge", Type: "m/shop.Size", Value: "0"},
				{ID: "m/shop.SizeSmall", Type: "m/shop.Size", Value: "1"},
			},
			Signatures: []arch.Signature{
				{
					Func: "m/shop.Empty",
					Site: arch.Site{Package: "m/shop"},
					Params: []arch.TypeRef{
						{Type: "*m/shop.Cart", Named: "m/shop.Cart", Package: "m/shop"},
					},
				},
			},
		},
	}
}

// The model's ownership, attributes, actions and enum order each conform or
// drift: Item is shared, Wishlist holds no Cart it owns, total is a field and
// discount is not, checkout and
// empty exist and pay does not, and Size's constants run large before small.
func TestDomainShape(t *testing.T) {
	t.Parallel()
	model := arch.DomainModel{
		Path:  "docs/shop.modelith.yaml",
		Enums: []arch.DomainEnum{{Name: "Size", Values: []string{"small", "large"}}},
		Entities: []arch.DomainEntity{
			{
				Name:          "Cart",
				Relationships: []arch.DomainRelationship{{Entity: "Item", Owned: true}},
				Attributes:    []string{"total", "discount"},
				Actions:       []string{"checkout", "empty", "pay"},
			},
			{Name: "Item"},
			{Name: "Wishlist", Relationships: []arch.DomainRelationship{
				{Entity: "Cart", Owned: true}, {Entity: "Item"},
			}},
		},
	}
	m := arch.Model{Version: 1, Fitness: arch.Fitness{Domains: []arch.DomainRule{{
		RuleOptions: arch.RuleOptions{Name: "shop", Reason: "r"}, Path: model.Path,
	}}}}
	r := fitness.Analyze(
		&arch.Input{Program: shape(), Model: m, Domains: []arch.DomainModel{model}},
	)
	const at = "docs/shop.modelith.yaml:"
	equals(t, []string{
		"domain-action-missing " + at + "Cart pay",
		"domain-attribute-missing " + at + "Cart discount",
		"domain-enum-order " + at + "Size m/shop.Size",
		"domain-ownership-drift " + at + "Cart Item shared by Wishlist",
		"domain-ownership-drift " + at + "Wishlist Cart",
	}, rules(&r))
}
