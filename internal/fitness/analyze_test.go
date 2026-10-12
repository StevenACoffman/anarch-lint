package fitness_test

import (
	"cmp"
	"reflect"
	"slices"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// program is an app of three layers and two areas: web imports store
// directly, skipping the service layer, and the two areas import each other.
func program() arch.Program {
	imp := func(from, to string, line int) arch.Import {
		return arch.Import{From: from, To: to, Pos: arch.Position{File: from + ".go", Line: line}}
	}
	pkgs := []string{"m/a/web", "m/a/service", "m/a/store", "m/b/web", "m/b/service"}
	p := arch.Program{
		Imports: []arch.Import{
			imp("m/a/service", "m/a/store", 3),
			imp("m/a/service", "m/b/service", 4),
			imp("m/a/store", "database/sql", 2),
			imp("m/a/web", "m/a/service", 2),
			imp("m/a/web", "m/a/store", 3),
			imp("m/b/service", "m/a/service", 5),
			imp("m/b/web", "m/b/service", 2),
		},
		Types: []arch.Type{
			{ID: "m/a/store.DB", Package: "m/a/store", Name: "DB", Kind: arch.KindStruct},
			{ID: "m/a/store.Repo", Package: "m/a/store", Name: "Repo", Kind: arch.KindInterface},
			{ID: "m/a/store.cache", Package: "m/a/store", Name: "cache", Kind: arch.KindStruct},
		},
		Facts: arch.Facts{
			Calls: []arch.Call{
				{
					Callee: "time.Now",
					Count:  2,
					Site: arch.Site{
						File:    "s.go",
						Line:    9,
						Package: "m/a/service",
						Func:    "m/a/service.Run",
					},
				},
				{
					Callee: "database/sql.Open",
					Count:  1,
					Site: arch.Site{
						File:    "w.go",
						Line:    4,
						Package: "m/a/web",
						Func:    "m/a/web.Serve",
					},
				},
			},
			Uses: []arch.Use{
				{
					Object:  "m/a/store.DB",
					Kind:    arch.ObjectType,
					Package: "m/a/store",
					Count:   1,
					Site: arch.Site{
						File:    "s.go",
						Line:    5,
						Package: "m/a/service",
						Func:    "m/a/service.Run",
					},
				},
				{
					Object:  "m/a/store.Repo",
					Kind:    arch.ObjectType,
					Package: "m/a/store",
					Count:   1,
					Site: arch.Site{
						File:    "s.go",
						Line:    6,
						Package: "m/a/service",
						Func:    "m/a/service.Run",
					},
				},
			},
		},
	}
	for _, path := range pkgs {
		p.Packages = append(p.Packages, arch.Package{Path: path, Module: "m"})
	}
	slices.SortFunc(p.Packages, func(a, b arch.Package) int { return cmp.Compare(a.Path, b.Path) })
	return p
}

// layered is the model with web over service over store.
func layered(f *arch.Fitness) arch.Model {
	return arch.Model{
		Version: 1,
		Layers: []arch.Layer{
			{Name: "web", Packages: []string{"m/$*/web"}},
			{Name: "service", Packages: []string{"m/$*/service"}},
			{Name: "store", Packages: []string{"m/$*/store"}},
		},
		Fitness: *f,
	}
}

func analyze(m *arch.Model) fitness.Report {
	return fitness.Analyze(&arch.Input{Program: program(), Model: *m})
}

// rules returns each finding as "rule subject object", leaving out the
// advice that a rule gives no reason, which TestUnexplainedRules covers.
func rules(r *fitness.Report) []string {
	var out []string
	for i := range r.Findings {
		if f := &r.Findings[i]; f.Rule != "rule-without-reason" {
			out = append(out, f.Rule+" "+f.Subject+" "+f.Object)
		}
	}
	return out
}

// A rule without a reason is advice, a warning, whatever its gate.
func TestUnexplainedRules(t *testing.T) {
	t.Parallel()
	m := layered(&arch.Fitness{Calls: []arch.CallRule{
		{
			RuleOptions: arch.RuleOptions{
				Name:     "explained",
				Reason:   "inject a clock",
				Optional: true,
			},
			From:    []string{"m/zzz"},
			Callees: []string{"time.Now"},
		},
		{
			RuleOptions: arch.RuleOptions{Name: "unexplained", Optional: true},
			From:        []string{"m/zzz"}, Callees: []string{"time.Now"},
		},
	}})
	r := analyze(&m)
	var advice []string
	for _, f := range r.Findings {
		if f.Rule == "rule-without-reason" {
			equals(t, arch.SeverityWarning, f.Severity)
			advice = append(advice, f.Subject)
		}
	}
	equals(t, []string{"unexplained"}, advice)
}

func TestRuleKinds(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		fitness  arch.Fitness
		findings []string
	}{
		"strict layers report the skip": {
			fitness:  arch.Fitness{StrictLayers: true},
			findings: []string{"strict-layers m/a/web strictLayers: m/a/store"},
		},
		"only service may access store": {
			fitness: arch.Fitness{LayerAccess: []arch.LayerAccess{{
				RuleOptions: arch.RuleOptions{Name: "store-access"}, Layer: "store",
				MayOnlyBeAccessedBy: []string{"service"},
			}}},
			findings: []string{"layer-access m/a/web store-access: m/a/store"},
		},
		"the onion forbids an inner ring importing an outer one": {
			fitness: arch.Fitness{Onion: arch.Onion{
				DomainModels: []string{"m/a/store"}, ApplicationServices: []string{"m/a/service"},
				Adapters: map[string][]string{"web": {"m/a/web"}},
			}},
			findings: nil, // every import points inward
		},
		"slices may not depend on each other": {
			fitness: arch.Fitness{Slices: []arch.Slices{
				{
					RuleOptions: arch.RuleOptions{
						Name: "areas",
					},
					Pattern:              "m/$*/...",
					NotDependOnEachOther: true,
				},
			}},
			findings: []string{
				"slice-dependency m/a/service areas: m/b/service",
				"slice-dependency m/b/service areas: m/a/service",
			},
		},
		"slices free of cycles": {
			fitness: arch.Fitness{Slices: []arch.Slices{
				{
					RuleOptions: arch.RuleOptions{
						Name: "areas",
					},
					Pattern:      "m/$*/...",
					FreeOfCycles: true,
				},
			}},
			findings: []string{"slice-cycle areas a → b → a"},
		},
		"only the store may import database/sql": {
			fitness: arch.Fitness{Dependents: []arch.Dependents{{
				RuleOptions: arch.RuleOptions{Name: "sql"}, Packages: []string{"database/sql"},
				OnlyFrom: []string{"m/a/store"},
			}}},
			findings: nil,
		},
		"no clock in services": {
			fitness: arch.Fitness{Calls: []arch.CallRule{{
				RuleOptions: arch.RuleOptions{Name: "clock"}, From: []string{"m/$*/service"},
				Callees: []string{"time.*"},
			}}},
			findings: []string{"forbidden-call m/a/service.Run clock: time.Now"},
		},
		"only the store may open a database": {
			fitness: arch.Fitness{Calls: []arch.CallRule{{
				RuleOptions: arch.RuleOptions{Name: "open"}, From: []string{"m/a/store"},
				Callees: []string{"database/sql.Open"}, Only: true,
			}}},
			findings: []string{"call-restriction m/a/web.Serve open: database/sql.Open"},
		},
		"services use only the store's interfaces": {
			fitness: arch.Fitness{InterfacesOnly: []arch.InterfacesOnly{{
				RuleOptions: arch.RuleOptions{Name: "ports"}, From: []string{"m/$*/service"},
				To: []string{"m/a/store"},
			}}},
			findings: []string{"interfaces-only m/a/service.Run ports: m/a/store.DB"},
		},
		"every package must have a subsystem": {
			fitness: arch.Fitness{RequireAssigned: true},
			findings: []string{
				"package-outside-architecture m/a/service subsystem",
				"package-outside-architecture m/a/store subsystem",
				"package-outside-architecture m/a/web subsystem",
				"package-outside-architecture m/b/service subsystem",
				"package-outside-architecture m/b/web subsystem",
			},
		},
		"an empty rule is a finding": {
			fitness: arch.Fitness{Dependents: []arch.Dependents{{
				RuleOptions: arch.RuleOptions{Name: "nothing"}, Packages: []string{"m/zzz/..."},
			}}},
			findings: []string{"empty-selector nothing "},
		},
		"an optional empty rule passes": {
			fitness: arch.Fitness{Dependents: []arch.Dependents{
				{
					RuleOptions: arch.RuleOptions{
						Name:     "nothing",
						Optional: true,
					},
					Packages: []string{"m/zzz/..."},
				},
			}},
			findings: nil,
		},
		"a rule that is off checks nothing": {
			fitness: arch.Fitness{Calls: []arch.CallRule{
				{
					RuleOptions: arch.RuleOptions{
						Name: "clock",
						Gate: arch.GateOff,
					},
					From:    []string{"m/$*/service"},
					Callees: []string{"time.Now"},
				},
			}},
			findings: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := layered(&tc.fitness)
			r := analyze(&m)
			equals(t, tc.findings, rules(&r))
		})
	}
}

func TestRuleTableAndGates(t *testing.T) {
	t.Parallel()
	m := layered(&arch.Fitness{Calls: []arch.CallRule{
		{
			RuleOptions: arch.RuleOptions{
				Name:   "clock",
				Gate:   arch.GateWarn,
				Reason: "inject a clock",
			},
			From:    []string{"m/$*/service"},
			Callees: []string{"time.Now"},
		},
		{
			RuleOptions: arch.RuleOptions{
				Name: "off",
				Gate: arch.GateOff,
			},
			From:    []string{"m/a/web"},
			Callees: []string{"fmt.*"},
		},
	}})
	r := analyze(&m)
	equals(t, []fitness.RuleResult{
		{
			Name: "clock", Kind: "calls", Reason: "inject a clock", Gate: arch.GateWarn, Matched: 2,
			Violations: 1, Status: fitness.StatusFailed,
		},
		{Name: "off", Kind: "calls", Gate: arch.GateOff, Matched: 1, Status: fitness.StatusOff},
	}, r.Rules)
	equals(t, arch.SeverityWarning, r.Findings[0].Severity)
	equals(t, "m/a/service.Run calls time.Now 2 times: inject a clock", r.Findings[0].Message)
}

func TestEmptyExistingRules(t *testing.T) {
	t.Parallel()
	m := layered(&arch.Fitness{})
	m.ForbiddenImports = []arch.ForbiddenImport{
		{From: []string{"m/a/..."}, Imports: []string{"net/http"}, Reason: "r"},
	}
	r := analyze(&m)
	equals(t, []string{"empty-selector forbiddenImports[0] imports"}, rules(&r))
	equals(t, arch.SeverityWarning, r.Findings[0].Severity) // the model declares no fitness rule
}

func TestMetrics(t *testing.T) {
	t.Parallel()
	m := layered(&arch.Fitness{})
	r := analyze(&m)
	packages := r.Lakos[1]
	equals(t, "packages", packages.Level)
	equals(t, 5, packages.Nodes)
	// Depends Upon: a/web 4 (itself, service, store, b/service), a/service 3,
	// a/store 1, b/service 3, b/web 4: CCD 15; a tree of 5 has 1+2+2+3+3 = 11.
	equals(t, 15, packages.CCD)
	equals(t, 1.363636, packages.NCCD)

	store := r.Visibility.Packages[1]
	equals(t, "m/a/store", store.Package)
	equals(
		t,
		fitness.PackageVisibility{Package: "m/a/store", Elements: 3, Visible: 2, RV: 0.666667},
		store,
	)

	var web fitness.ComponentDependency
	for _, c := range r.ComponentDependencyMetrics {
		if c.Package == "m/b/web" {
			web = c
		}
	}
	equals(
		t,
		fitness.ComponentDependency{
			Package:            "m/b/web",
			Ce:                 1,
			Instability:        1,
			NormalizedDistance: 0,
		},
		web,
	)
}

// Reversing the facts changes nothing: the report sorts what it finds.
func TestAnalyzeIgnoresInputOrder(t *testing.T) {
	t.Parallel()
	m := layered(&arch.Fitness{StrictLayers: true, Calls: []arch.CallRule{
		{
			RuleOptions: arch.RuleOptions{
				Name: "clock",
			},
			From:    []string{"m/$*/..."},
			Callees: []string{"time.*", "database/sql.*"},
		},
	}})
	p := program()
	reversed := program()
	slices.Reverse(reversed.Facts.Calls)
	a := fitness.Analyze(&arch.Input{Program: p, Model: m})
	b := fitness.Analyze(&arch.Input{Program: reversed, Model: m})
	equals(t, a, b)
}
