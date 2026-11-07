package modelith_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/modelith"
)

const garage = `# yaml-language-server: $schema=https://modelith.sh/schema/domain-model/v1.json
kind: DomainModel
version: v1
shared: true
imports:
  - ./payments.modelith.yaml
  - scope: billing
    path: ./legacy/pay-v2.modelith.yaml
glossary:
  Owner: "A User with full control."
  Attendant: "Works the gate."
enums:
  VisitStatus:
    values:
      - name: open
      - name: closed
entities:
  Visit:
    definition: One car's stay.
    attributes:
      - name: plate
        type: string
      - name: hours
        type: int
        derived: true
        derivation: From entry to exit.
    actions:
      - open
      - name: close
        actor: Attendant
        preserves: [one-open-visit, paid-before-exit]
    relationships:
      - entity: Ticket
        cardinality: "1:1"
        ownership: owned
      - entity: payments.PaymentMethod
        cardinality: "n:1"
    invariants:
      - id: one-open-visit
        statement: "At most one open Visit per car."
  Ticket:
    definition: Proof of entry.
invariants:
  - id: paid-before-exit
    statement: "A Visit is paid before the car leaves."
scenarios:
  - name: a normal stay
    steps: [enter, pay, leave]
`

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	m, err := modelith.Parse("docs/garage.modelith.yaml", []byte(garage))
	if err != nil {
		t.Fatal(err)
	}
	equals(t, arch.DomainModel{
		Path:   "docs/garage.modelith.yaml",
		Shared: true,
		Imports: []arch.DomainImport{
			{Scope: "billing", Path: "./legacy/pay-v2.modelith.yaml"},
			{Scope: "payments", Path: "./payments.modelith.yaml"},
		},
		Glossary: []string{"Attendant", "Owner"},
		Enums:    []arch.DomainEnum{{Name: "VisitStatus", Values: []string{"open", "closed"}}},
		Entities: []arch.DomainEntity{
			{Name: "Ticket"},
			{
				Name: "Visit", Relationships: []arch.DomainRelationship{
					{Entity: "Ticket", Owned: true}, {Entity: "payments.PaymentMethod"},
				},
				Attributes: []string{"plate"}, Actions: []string{"open", "close"},
			},
		},
		Invariants: []arch.DomainInvariant{
			{
				ID: "one-open-visit", Statement: "At most one open Visit per car.",
				Entity: "Visit", PreservedBy: []string{"Visit.close"},
			},
			{
				ID: "paid-before-exit", Statement: "A Visit is paid before the car leaves.",
				PreservedBy: []string{"Visit.close"},
			},
		},
	}, m)
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"not YAML":           "kind: [",
		"another kind":       "kind: Something\nversion: v1\n",
		"another version":    "kind: DomainModel\nversion: v2\n",
		"a malformed import": "kind: DomainModel\nversion: v1\nimports:\n  - {scope: x}\n",
		"an unknown field":   "kind: DomainModel\nversion: v1\nentities:\n  A:\n    definition: a\n    colour: red\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := modelith.Parse("m.modelith.yaml", []byte(src))
			equals(t, arch.EINVALID, arch.ErrorCode(err))
			if msg := arch.ErrorMessage(err); !strings.Contains(msg, "modelith lint") {
				t.Errorf("message %q does not point to modelith lint", msg)
			}
		})
	}
}
