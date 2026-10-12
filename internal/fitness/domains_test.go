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
		Refs: []arch.Ref{{From: "m/shop.Order", To: "m/shop.Line", Weight: 1}},
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
