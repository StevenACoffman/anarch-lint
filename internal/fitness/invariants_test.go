package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// park is a program of a parking garage whose types hold their state in
// fields named by the case: m/park declares the entities' types, and m/web
// writes Ticket.Paid from another package. Nothing enforces an invariant.
func park(visit, ticket []string) arch.Program {
	typ := func(name string, fields []string) arch.Type {
		return arch.Type{
			ID: "m/park." + name, Package: "m/park", Name: name, Kind: arch.KindStruct,
			Fields: fields,
		}
	}
	write := func(pkg string, line int) arch.Use {
		return arch.Use{
			Object: "m/park.Ticket.Paid", Kind: arch.ObjectField, Package: "m/park",
			Exported: true, Owner: "m/park.Ticket", Count: 1, Writes: 1,
			Site: arch.Site{File: pkg + "/pay.go", Line: line, Package: "m/" + pkg},
		}
	}
	return arch.Program{
		Packages: []arch.Package{{Path: "m/park"}, {Path: "m/web"}},
		Types: []arch.Type{
			typ("Attendant", []string{"Name"}),
			typ("Ticket", ticket),
			typ("Visit", visit),
		},
		Facts: arch.Facts{Uses: []arch.Use{write("park", 3), write("web", 7)}},
	}
}

// An invariant is exposed when an entity it governs has a type with an
// exported field: the entity that declares it, or one its statement names in
// backticks. A glossary term in backticks is no entity, and a type whose
// fields are all unexported keeps its state behind its methods.
func TestInvariantExposed(t *testing.T) {
	t.Parallel()
	const at = "docs/park.modelith.yaml:"
	cases := map[string]struct {
		visit, ticket []string
		invariant     arch.DomainInvariant
		expected      []string
	}{
		"unexported fields keep it": {
			visit: []string{"car", "open"}, ticket: []string{"Paid"},
			invariant: arch.DomainInvariant{
				ID: "one-open-visit", Statement: "At most one open visit per car.", Entity: "Visit",
			},
		},
		"an exported field of the declaring entity exposes it": {
			visit: []string{"Car", "open"},
			invariant: arch.DomainInvariant{
				ID: "one-open-visit", Statement: "At most one open visit per car.", Entity: "Visit",
			},
			expected: []string{"invariant-exposed " + at + "one-open-visit m/park.Visit"},
		},
		"a model-level invariant governs the entities it names": {
			visit: []string{"car"}, ticket: []string{"Paid"},
			invariant: arch.DomainInvariant{
				ID: "paid-before-exit", Statement: "A `Visit` ends only once its `Ticket` is paid.",
			},
			expected: []string{"invariant-exposed " + at + "paid-before-exit m/park.Ticket"},
		},
		"a glossary term is no entity": {
			invariant: arch.DomainInvariant{
				ID: "attended", Statement: "An `Attendant` is on duty while the gate is open.",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			model := arch.DomainModel{
				Path:       "docs/park.modelith.yaml",
				Entities:   []arch.DomainEntity{{Name: "Ticket"}, {Name: "Visit"}},
				Invariants: []arch.DomainInvariant{tc.invariant},
			}
			m := arch.Model{Version: 1, Fitness: arch.Fitness{Domains: []arch.DomainRule{{
				RuleOptions: arch.RuleOptions{Name: "park", Reason: "r"}, Path: model.Path,
			}}}}
			p := park(tc.visit, tc.ticket)
			p.Enforcers = []arch.Enforcer{
				{Invariant: tc.invariant.ID, Func: "m/park.Visit.Open", Package: "m/park"},
			}
			r := fitness.Analyze(
				&arch.Input{Program: p, Model: m, Domains: []arch.DomainModel{model}},
			)
			equals(t, tc.expected, rules(&r))
		})
	}
}

// The writes from other packages are the evidence of an exposed invariant;
// the type's own package may write its fields, as its constructors do.
func TestInvariantExposedEvidence(t *testing.T) {
	t.Parallel()
	model := arch.DomainModel{
		Path:     "docs/park.modelith.yaml",
		Entities: []arch.DomainEntity{{Name: "Ticket"}, {Name: "Visit"}},
		Invariants: []arch.DomainInvariant{{
			ID: "paid-before-exit", Statement: "A ticket is paid before the car leaves.",
			Entity: "Ticket",
		}},
	}
	m := arch.Model{Version: 1, Fitness: arch.Fitness{Domains: []arch.DomainRule{{
		RuleOptions: arch.RuleOptions{Name: "park", Reason: "r"}, Path: model.Path,
	}}}}
	p := park(nil, []string{"Paid", "issued"})
	p.Enforcers = []arch.Enforcer{
		{Invariant: "paid-before-exit", Func: "m/park.TestExit", Package: "m/park", Test: true},
	}
	r := fitness.Analyze(&arch.Input{Program: p, Model: m, Domains: []arch.DomainModel{model}})
	equals(t, 1, len(r.Findings))
	equals(t, []string{
		"exported fields: Paid",
		"m/park.Ticket.Paid written at web/pay.go:7",
	}, r.Findings[0].Evidence)
	equals(t, arch.SeverityError, r.Findings[0].Severity)
}

// An invariant is enforced when a function or test of the rule's packages
// carries its directive. One elsewhere does not count for the rule, and a
// directive naming no invariant of a covering model is a typo to correct.
func TestInvariantEnforcers(t *testing.T) {
	t.Parallel()
	const at = "docs/park.modelith.yaml:"
	enforcer := func(invariant, fn, pkg string, test bool) arch.Enforcer {
		return arch.Enforcer{
			Invariant: invariant, Func: fn, Package: pkg, Test: test,
			Pos: arch.Position{File: "park/visit.go", Line: 9},
		}
	}
	cases := map[string]struct {
		enforcers []arch.Enforcer
		expected  []string
	}{
		"a function enforces it": {
			enforcers: []arch.Enforcer{
				enforcer("one-open-visit", "m/park.Visit.Open", "m/park", false),
			},
		},
		"a test enforces it": {
			enforcers: []arch.Enforcer{
				enforcer("one-open-visit", "m/park.TestOneOpenVisit", "m/park", true),
			},
		},
		"nothing enforces it": {
			expected: []string{"invariant-unenforced " + at + "one-open-visit "},
		},
		"an enforcer outside the rule's subsystem does not count": {
			enforcers: []arch.Enforcer{
				enforcer("one-open-visit", "m/web.Open", "m/web", false),
			},
			expected: []string{
				"invariant-unenforced " + at + "one-open-visit ",
				"unknown-invariant-directive m/web.Open one-open-visit",
			},
		},
		"a misspelt directive enforces nothing": {
			enforcers: []arch.Enforcer{
				enforcer("one-open-vist", "m/park.Visit.Open", "m/park", false),
			},
			expected: []string{
				"invariant-unenforced " + at + "one-open-visit ",
				"unknown-invariant-directive m/park.Visit.Open one-open-vist",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			model := arch.DomainModel{
				Path:     "docs/park.modelith.yaml",
				Entities: []arch.DomainEntity{{Name: "Visit"}},
				Invariants: []arch.DomainInvariant{{
					ID: "one-open-visit", Statement: "At most one open visit per car.",
					Entity: "Visit",
				}},
			}
			m := arch.Model{
				Version: 1,
				Subsystems: []arch.Subsystem{
					{Name: "park", Packages: []string{"m/park"}},
					{Name: "web", Packages: []string{"m/web"}},
				},
				Fitness: arch.Fitness{Domains: []arch.DomainRule{{
					RuleOptions: arch.RuleOptions{Name: "park", Reason: "r"}, Path: model.Path,
					Subsystem: "park",
				}}},
			}
			p := park([]string{"car"}, nil)
			p.Enforcers = tc.enforcers
			r := fitness.Analyze(
				&arch.Input{Program: p, Model: m, Domains: []arch.DomainModel{model}},
			)
			equals(t, tc.expected, rules(&r))
		})
	}
}

// With a coverage profile, an invariant enforced only by functions that no
// test runs is untested; a function the profile does not instrument did not
// run, and a test enforcer is run by go test. Without a profile nothing is
// known, so nothing is reported.
func TestInvariantUntested(t *testing.T) {
	t.Parallel()
	const at = "docs/park.modelith.yaml:"
	open := arch.Enforcer{Invariant: "one-open-visit", Func: "m/park.Visit.Open", Package: "m/park"}
	audit := arch.Enforcer{Invariant: "one-open-visit", Func: "m/park.Audit", Package: "m/park"}
	test := arch.Enforcer{
		Invariant: "one-open-visit", Func: "m/park.TestOneOpenVisit", Package: "m/park", Test: true,
	}
	cases := map[string]struct {
		enforcers []arch.Enforcer
		coverage  *arch.Coverage
		expected  []string
	}{
		"no profile": {enforcers: []arch.Enforcer{open}},
		"a test runs the enforcer": {
			enforcers: []arch.Enforcer{open},
			coverage: &arch.Coverage{Funcs: []arch.FuncCoverage{
				{Func: "m/park.Visit.Open", Statements: 4, Covered: 1},
			}},
		},
		"no test runs the enforcer": {
			enforcers: []arch.Enforcer{open},
			coverage: &arch.Coverage{Funcs: []arch.FuncCoverage{
				{Func: "m/park.Visit.Open", Statements: 4},
			}},
			expected: []string{"invariant-untested " + at + "one-open-visit "},
		},
		"the profile does not instrument the enforcer": {
			enforcers: []arch.Enforcer{open},
			coverage:  &arch.Coverage{},
			expected:  []string{"invariant-untested " + at + "one-open-visit "},
		},
		"one of two enforcers runs": {
			enforcers: []arch.Enforcer{audit, open},
			coverage: &arch.Coverage{Funcs: []arch.FuncCoverage{
				{Func: "m/park.Visit.Open", Statements: 4, Covered: 4},
			}},
		},
		"a test enforces it": {
			enforcers: []arch.Enforcer{open, test},
			coverage:  &arch.Coverage{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			model := arch.DomainModel{
				Path:     "docs/park.modelith.yaml",
				Entities: []arch.DomainEntity{{Name: "Visit"}},
				Invariants: []arch.DomainInvariant{{
					ID: "one-open-visit", Statement: "At most one open visit per car.",
					Entity: "Visit",
				}},
			}
			m := arch.Model{Version: 1, Fitness: arch.Fitness{Domains: []arch.DomainRule{{
				RuleOptions: arch.RuleOptions{Name: "park", Reason: "r"}, Path: model.Path,
			}}}}
			p := park([]string{"car"}, nil)
			p.Enforcers = tc.enforcers
			r := fitness.Analyze(&arch.Input{
				Program: p, Model: m, Domains: []arch.DomainModel{model}, Coverage: tc.coverage,
			})
			equals(t, tc.expected, rules(&r))
		})
	}
}
