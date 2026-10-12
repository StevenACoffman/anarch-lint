package arch_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// analyze analyzes the fixture program under m.
func analyze(m *arch.Model, verdicts ...arch.Verdict) arch.Report {
	p := fixtureProgram()
	return arch.Analyze(
		&arch.Input{
			Program:  p,
			Model:    *m,
			Verdicts: verdicts,
			Inputs:   arch.Inputs{Tool: "test", Patterns: []string{"./..."}},
		},
	)
}

func findingsOf(r *arch.Report, rule string) []arch.Finding {
	var fs []arch.Finding
	for i := range r.Findings {
		if r.Findings[i].Rule == rule {
			fs = append(fs, r.Findings[i])
		}
	}
	return fs
}

func criterion(r *arch.Report, id string) arch.Criterion {
	for i := range r.MMI.Criteria {
		if r.MMI.Criteria[i].ID == id {
			return r.MMI.Criteria[i]
		}
	}
	return arch.Criterion{}
}

func TestScoreOfBoundaries(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		criterion string
		value     float64
		score     int
	}{
		"1.1.1 at the lowest bound scores 0":    {"1.1.1", 54, 0},
		"1.1.1 just above it scores 1":          {"1.1.1", 54.01, 1},
		"1.1.1 above the top bound scores 10":   {"1.1.1", 90.01, 10},
		"1.1.1 at the top bound scores 9":       {"1.1.1", 90, 9},
		"2.2.1 at 25% scores 0":                 {"2.2.1", 25, 0},
		"2.2.1 just below 25% scores 1":         {"2.2.1", 24.99, 1},
		"2.2.1 below 2.5% scores 10":            {"2.2.1", 2.49, 10},
		"2.2.1 at 2.5% scores 9":                {"2.2.1", 2.5, 9},
		"2.2.3 of 106 types scores 0":           {"2.2.3", 106, 0},
		"2.2.3 of 3 types scores 10":            {"2.2.3", 3, 10},
		"1.1.3 at 7.5 scores 0":                 {"1.1.3", 7.5, 0},
		"1.1.3 below 0.5 scores 10":             {"1.1.3", 0.49, 10},
		"1.3.4 with no complex functions is 10": {"1.3.4", 0, 10},
		"3.1 above 95% scores 10":               {"3.1", 95.5, 10},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			score, ok := arch.ScoreOf(tc.criterion, tc.value)
			assert(t, ok, "criterion has no Table 4-2 row")
			equals(t, tc.score, score)
		})
	}
	_, ok := arch.ScoreOf("1.1.5", 3)
	assert(t, !ok, "a reviewer criterion got a numeric score")
}

func TestAnalyzeFixtureFindings(t *testing.T) {
	t.Parallel()
	m := fixtureModel()
	r := analyze(&m)

	cycles := findingsOf(&r, arch.RuleSubsystemCycle)
	equals(t, 1, len(cycles))
	equals(t, "billing ↔ shop", cycles[0].Subject)
	assert(t, strings.Contains(cycles[0].Fix, "cut "), "cycle finding has no cut: "+cycles[0].Fix)

	// shop declares no interface, so tax's import of catalog bypasses it;
	// catalog's import of invoice uses billing's interface.
	bypass := findingsOf(&r, arch.RuleInterfaceBypass)
	equals(t, 1, len(bypass))
	equals(t, "fx/billing/tax", bypass[0].Subject)
	equals(t, "fx/shop/catalog", bypass[0].Object)
	equals(t, 1, len(findingsOf(&r, arch.RuleSubsystemWithoutIface)))

	c := criterion(&r, "1.2.1")
	equals(t, arch.Measured, c.Kind)
	equals(t, 50.0, *c.Value)
	equals(t, 0, *c.Score)
}

func TestAnalyzeRules(t *testing.T) {
	t.Parallel()
	layers := func(names ...string) []arch.Layer {
		var ls []arch.Layer
		for _, n := range names {
			ls = append(ls, arch.Layer{Name: n, Packages: []string{"fx/" + n + "/..."}})
		}
		return ls
	}
	cases := map[string]struct {
		model arch.Model
		rule  string
		count int
	}{
		"an import against the layer order": {
			model: arch.Model{Version: 1, Layers: layers("billing", "shop")},
			rule:  arch.RuleLayerOrder,
			count: 1, // billing/tax imports shop/catalog downward, shop/catalog imports billing upward
		},
		"forbidden import outside the scope": {
			model: arch.Model{Version: 1, ForbiddenImports: []arch.ForbiddenImport{
				{From: []string{"fx/shop/..."}, Imports: []string{"fmt"}, Reason: "no printing"},
			}},
			rule:  arch.RuleForbiddenImport,
			count: 1,
		},
		"forbidden import that does not occur": {
			model: arch.Model{Version: 1, ForbiddenImports: []arch.ForbiddenImport{
				{From: []string{"fx/billing/..."}, Imports: []string{"fmt"}, Reason: "no printing"},
			}},
			rule:  arch.RuleForbiddenImport,
			count: 0,
		},
		"role order": {
			model: arch.Model{
				Version: 1,
				Roles: []arch.Role{
					{Name: "entity", Suffix: "Line"},
					{Name: "aggregate", Suffix: "Invoice"},
				},
			},
			rule:  arch.RuleRoleOrder,
			count: 1, // Invoice (aggregate, lower) refers to Line (entity, upper)
		},
		"type cycle above the limit": {
			model: arch.Model{Version: 1, Thresholds: arch.Thresholds{MaxTypeCycleGroup: 2}},
			rule:  arch.RuleTypeCycleTooLarge,
			count: 1,
		},
		"isolation keeps each area to itself": {
			model: arch.Model{Version: 1, Isolations: []arch.Isolation{{
				From: "fx/$*/...", MayImport: []string{"fx/$1/..."}, Reason: "areas stay apart",
			}}},
			rule:  arch.RuleIsolation,
			count: 2, // billing/tax imports shop/catalog; shop/catalog imports billing/invoice
		},
		"isolation that allows the other area": {
			model: arch.Model{Version: 1, Isolations: []arch.Isolation{
				{
					From:      "fx/$*/...",
					MayImport: []string{"fx/$1/...", "fx/billing/invoice"},
					Reason:    "r",
				},
			}},
			rule:  arch.RuleIsolation,
			count: 1, // only billing/tax's import of shop/catalog is left
		},
		"a multi-segment capture": {
			model: arch.Model{Version: 1, Isolations: []arch.Isolation{{
				From: "$**/catalog", MayImport: []string{"$1/..."}, Reason: "r",
			}}},
			rule:  arch.RuleIsolation,
			count: 1, // fx/shop/catalog may import only fx/shop/..., not fx/billing/invoice
		},
		"files above the length limit": {
			model: arch.Model{Version: 1, Thresholds: arch.Thresholds{LargeFileLines: 5}},
			rule:  arch.RuleLargeFile,
			count: 2,
		},
		"files within the default length limit": {
			model: arch.Model{Version: 1},
			rule:  arch.RuleLargeFile,
			count: 0,
		},
		"type cycles within the default limit": {
			model: arch.Model{Version: 1},
			rule:  arch.RuleTypeCycleTooLarge,
			count: 0,
		},
		"unassigned and stale": {
			model: arch.Model{
				Version: 1,
				Subsystems: []arch.Subsystem{
					{Name: "b", Packages: []string{"fx/billing/...", "fx/gone"}},
				},
			},
			rule:  arch.RuleUnassignedPackage,
			count: 1,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := analyze(&tc.model)
			equals(t, tc.count, len(findingsOf(&r, tc.rule)))
			for _, f := range findingsOf(&r, tc.rule) {
				assert(t, f.Fix != "", "finding without a fix: "+f.Message)
				assert(t, len(f.Fingerprint) == 16, "fingerprint "+f.Fingerprint)
			}
		})
	}
}

func TestUnmeasuredWithoutModel(t *testing.T) {
	t.Parallel()
	r := analyze(&arch.Model{Version: 1})
	for _, id := range []string{"1.1.1", "1.1.2", "1.1.3", "1.1.4", "1.2.1", "2.1.1", "2.1.2", "3.1", "3.2"} {
		c := criterion(&r, id)
		equals(t, arch.Unmeasured, c.Kind)
		assert(t, c.Score == nil, id+" has a score without a model")
		assert(t, c.Reason != "", id+" has no reason")
	}
	equals(t, 22, len(r.MMI.Criteria))
	assert(t, r.MMI.Low < r.MMI.High, "the interval is closed with criteria unmeasured")
	assert(t, !r.MMI.Exact, "an MMI with pending judgements is exact")
}

func TestJudgementSlots(t *testing.T) {
	t.Parallel()
	m := fixtureModel()
	first := analyze(&m)
	var hash string
	for _, s := range first.Judgements {
		equals(t, arch.SlotPending, s.Status)
		if s.Criterion == "3.4" {
			hash = s.EvidenceHash
		}
	}
	cases := map[string]struct {
		verdict arch.Verdict
		status  arch.SlotStatus
		scored  bool
	}{
		"approved verdict for the current evidence": {
			verdict: arch.Verdict{
				Criterion:    "3.4",
				Score:        7,
				Provenance:   arch.ProvenanceHuman,
				EvidenceHash: hash,
				Approved:     true,
			},
			status: arch.SlotSupplied,
			scored: true,
		},
		"verdict for other evidence is stale": {
			verdict: arch.Verdict{
				Criterion:    "3.4",
				Score:        7,
				Provenance:   arch.ProvenanceHuman,
				EvidenceHash: "old",
				Approved:     true,
			},
			status: arch.SlotStale,
		},
		"draft verdict awaits approval": {
			verdict: arch.Verdict{
				Criterion:    "3.4",
				Score:        7,
				Provenance:   arch.ProvenanceLLM,
				EvidenceHash: hash,
			},
			status: arch.SlotUnapproved,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := analyze(&m, tc.verdict)
			for _, s := range r.Judgements {
				if s.Criterion == "3.4" {
					equals(t, tc.status, s.Status)
				}
			}
			equals(t, tc.scored, criterion(&r, "3.4").Score != nil)
			if tc.scored {
				assert(
					t,
					r.MMI.Low > first.MMI.Low,
					"a supplied judgement did not raise the low bound",
				)
			}
		})
	}
}

func TestDecodeJudgements(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		input   string
		message string
	}{
		"valid": {input: `{"version": 1, "verdicts": [{"criterion": "3.3", "score": 5,
			"provenance": "llm", "rationale": "r", "evidenceHash": "h", "approved": false}]}`},
		"score out of range": {
			input:   `{"version": 1, "verdicts": [{"criterion": "3.3", "score": 11, "provenance": "human"}]}`,
			message: "judgements: criterion 3.3: score 11 is outside 0–10",
		},
		"not a judgement criterion": {
			input:   `{"version": 1, "verdicts": [{"criterion": "2.2.1", "score": 1, "provenance": "human"}]}`,
			message: "judgements: criterion 2.2.1: not a judgement criterion or slot family",
		},
		"valid family answer": {
			input: `{"version": 1, "verdicts": [{"criterion": "subdomain/billing",
				"answer": "core", "provenance": "human"}]}`,
		},
		"unknown family": {
			input:   `{"version": 1, "verdicts": [{"criterion": "mood/billing", "answer": "x", "provenance": "human"}]}`,
			message: "judgements: criterion mood/billing: not a judgement criterion or slot family",
		},
		"family without subject": {
			input:   `{"version": 1, "verdicts": [{"criterion": "subdomain/", "answer": "core", "provenance": "human"}]}`,
			message: "judgements: criterion subdomain/: not a judgement criterion or slot family",
		},
		"answer not in family": {
			input: `{"version": 1, "verdicts": [{"criterion": "subdomain/billing",
				"answer": "boring", "provenance": "human"}]}`,
			message: `judgements: criterion subdomain/billing: answer "boring" is not one of ` +
				"core, generic, supporting",
		},
		"score on a family": {
			input: `{"version": 1, "verdicts": [{"criterion": "subdomain/billing",
				"answer": "core", "score": 3, "provenance": "human"}]}`,
			message: "judgements: criterion subdomain/billing: a slot family takes an answer, not a score",
		},
		"answer on an MMI criterion": {
			input: `{"version": 1, "verdicts": [{"criterion": "3.3", "score": 3,
				"answer": "core", "provenance": "human"}]}`,
			message: "judgements: criterion 3.3: an MMI criterion takes a score, not an answer",
		},
		"unknown provenance": {
			input:   `{"version": 1, "verdicts": [{"criterion": "3.3", "score": 1, "provenance": "oracle"}]}`,
			message: `judgements: criterion 3.3: provenance "oracle" is not human or llm`,
		},
		"two verdicts": {
			input: `{"version": 1, "verdicts": [{"criterion": "3.3", "score": 1, "provenance": "human"},
				{"criterion": "3.3", "score": 2, "provenance": "human"}]}`,
			message: "judgements: criterion 3.3 has two verdicts",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := arch.DecodeJudgements(strings.NewReader(tc.input))
			if tc.message == "" {
				equals(t, nil, err)
				return
			}
			equals(t, arch.EINVALID, arch.ErrorCode(err))
			equals(t, tc.message, arch.ErrorMessage(err))
		})
	}
}

// The same inputs must give the same bytes: mmi-plan.md §2.3.
func TestAnalyzeIsByteIdentical(t *testing.T) {
	t.Parallel()
	m := fixtureModel()
	var a, b bytes.Buffer
	if err := arch.WriteJSON(&a, analyze(&m)); err != nil {
		t.Fatal(err)
	}
	if err := arch.WriteJSON(&b, analyze(&m)); err != nil {
		t.Fatal(err)
	}
	assert(t, bytes.Equal(a.Bytes(), b.Bytes()), "two analyses of the same inputs differ")
}

func TestRoleMarkers(t *testing.T) {
	t.Parallel()
	const pkg = "fx/roles"
	p := arch.Program{
		Packages: []arch.Package{{Path: pkg, Lines: 50}},
		Types: []arch.Type{
			// The directive wins even though the suffix names another role.
			{ID: pkg + ".AHandler", Package: pkg, Name: "AHandler", Directive: "store", Lines: 10},
			{
				ID:      pkg + ".BCustomer",
				Package: pkg,
				Name:    "BCustomer",
				Embeds:  []string{pkg + ".Entity"},
				Lines:   10,
			},
			{ID: pkg + ".CTypo", Package: pkg, Name: "CTypo", Directive: "stroe", Lines: 10},
			{
				ID:         pkg + ".DStore",
				Package:    pkg,
				Name:       "DStore",
				Implements: []string{pkg + ".Repo"},
				Lines:      10,
			},
		},
	}
	m := arch.Model{Version: 1, Roles: []arch.Role{
		{Name: "handler", Suffix: "Handler"},
		{Name: "entity", Embeds: []string{pkg + ".Entity"}},
		{Name: "store", Implements: []string{pkg + ".Repo"}},
	}}
	r := arch.Analyze(&arch.Input{Program: p, Model: m})

	unknown := findingsOf(&r, arch.RuleUnknownRole)
	equals(t, 1, len(unknown))
	equals(t, pkg+".CTypo", unknown[0].Subject)
	equals(t, "stroe", unknown[0].Object)

	// Three of the four types play a role: 30 of 50 lines.
	equals(t, 60.0, *criterion(&r, "3.1").Value)
	for _, s := range r.Judgements {
		if s.Criterion == "3.3" {
			equals(t, []string{"roles: 3 declared; of 4 types, 1 have a role by directive, " +
				"1 by embedding a marker, 1 by implementing an interface, 0 by name suffix, " +
				"0 only by package, 1 none"}, s.Evidence)
		}
	}
}

func TestTransitiveForbiddenImport(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		imports []string
		path    string
	}{
		"in scope, through catalog": {
			imports: []string{"fx/billing/invoice"},
			path:    "import path: fx/billing/tax → fx/shop/catalog → fx/billing/invoice",
		},
		"outside the scope, through catalog": {
			imports: []string{"fmt"},
			path:    "import path: fx/billing/tax → fx/shop/catalog → fmt",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := arch.Model{Version: 1, ForbiddenImports: []arch.ForbiddenImport{
				{
					From: []string{
						"fx/billing/tax",
					},
					Imports:    tc.imports,
					Reason:     "r",
					Transitive: true,
				},
			}}
			r := analyze(&m)
			fs := findingsOf(&r, arch.RuleForbiddenImport)
			equals(t, 1, len(fs))
			equals(t, []string{tc.path}, fs[0].Evidence)
			assert(
				t,
				strings.Contains(fs[0].Fix, "passes through fx/shop/catalog"),
				"no dominator in "+fs[0].Fix,
			)
			equals(
				t,
				&arch.Span{File: "", Start: 0, End: 0},
				fs[0].Location,
			) // the fixture has no positions
		})
	}

	// Without transitive, the indirect path is not a violation.
	m := arch.Model{Version: 1, ForbiddenImports: []arch.ForbiddenImport{{
		From: []string{"fx/billing/tax"}, Imports: []string{"fmt"}, Reason: "r",
	}}}
	r := analyze(&m)
	equals(t, 0, len(findingsOf(&r, arch.RuleForbiddenImport)))
}

func TestHiddenDependency(t *testing.T) {
	t.Parallel()
	p := arch.Program{
		Packages: []arch.Package{{Path: "fx/app"}, {Path: "fx/roles"}, {Path: "fx/service"}},
		Types:    []arch.Type{{ID: "fx/roles.SQLStore", Package: "fx/roles", Name: "SQLStore"}},
		Injections: []arch.Injection{
			{
				Caller: "fx/app", Callee: "fx/service.Lookup", CalleePackage: "fx/service",
				Concrete: "fx/roles.SQLStore", Pos: arch.Position{File: "app/app.go", Line: 11},
			},
			{
				Caller: "fx/app", Callee: "fx/service.Other", CalleePackage: "fx/service",
				Concrete: "fx/roles.SQLStore", Pos: arch.Position{File: "app/app.go", Line: 12},
			},
		},
	}
	cases := map[string]struct {
		injected bool
		count    int
	}{
		"an injected rule sees through the interface": {injected: true, count: 1},
		"an import rule alone does not":               {injected: false, count: 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := arch.Model{Version: 1, ForbiddenImports: []arch.ForbiddenImport{
				{
					From: []string{
						"fx/service",
					},
					Imports:  []string{"fx/roles"},
					Reason:   "r",
					Injected: tc.injected,
				},
			}}
			r := arch.Analyze(&arch.Input{Program: p, Model: m})
			fs := findingsOf(&r, arch.RuleHiddenDependency)
			equals(t, tc.count, len(fs)) // two calls, one dependency: one finding
			if tc.count > 0 {
				equals(t, &arch.Span{File: "app/app.go", Start: 11, End: 11}, fs[0].Location)
			}
		})
	}
}
