package connascence_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
)

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// program is two subsystems of one module: billing and web each compile the
// same pattern, a high-confidence candidate of algorithm, and both write
// auth's exported variable, a fact of identity.
func program() *arch.Program {
	site := func(file string, line int, pkg, fn string) arch.Site {
		return arch.Site{File: file, Line: line, Package: pkg, Func: fn}
	}
	const pattern = `"^[a-z]+$"`
	return &arch.Program{
		Packages: []arch.Package{
			{Path: "m/auth", Name: "auth", Module: "m"},
			{Path: "m/billing", Name: "billing", Module: "m"},
			{Path: "m/web", Name: "web", Module: "m"},
		},
		Facts: arch.Facts{
			ConstExprs: []arch.ConstExpr{
				{
					Type: "string", Value: pattern, Literal: true, Context: arch.ContextArgument,
					Callee: "regexp.MustCompile", Site: site("billing/b.go", 3, "m/billing", ""),
				},
				{
					Type: "string", Value: pattern, Literal: true, Context: arch.ContextArgument,
					Callee: "regexp.MustCompile", Site: site("web/w.go", 4, "m/web", ""),
				},
			},
			GlobalUses: []arch.GlobalUse{
				{
					Var: "m/auth.Count", Package: "m/auth", Exported: true, Write: true,
					Site: site("billing/b.go", 9, "m/billing", "m/billing.Add"),
				},
				{
					Var: "m/auth.Count", Package: "m/auth", Exported: true, Write: true,
					Site: site("web/w.go", 7, "m/web", "m/web.Reset"),
				},
			},
		},
	}
}

func model(limits bool) *arch.Model {
	m := &arch.Model{Version: 1, Subsystems: []arch.Subsystem{
		{Name: "auth", Packages: []string{"m/auth"}},
		{Name: "billing", Packages: []string{"m/billing"}},
		{Name: "web", Packages: []string{"m/web"}},
	}}
	if limits {
		m.Connascence.MaxStrength = map[arch.Locality]int{arch.LocalitySystem: 3}
	}
	return m
}

func analyze(m *arch.Model, verdicts ...arch.Verdict) connascence.Report {
	return connascence.Analyze(
		&arch.Input{
			Program:  *program(),
			Model:    *m,
			Verdicts: verdicts,
			Inputs:   arch.Inputs{Tool: "test"},
		},
	)
}

func TestFactsOverTheLimitAreFindings(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		limits   bool
		severity arch.Severity
	}{
		"model opts in":  {limits: true, severity: arch.SeverityError},
		"model opts out": {limits: false, severity: arch.SeverityWarning},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := analyze(model(tc.limits))
			equals(t, 1, len(r.Findings))
			f := r.Findings[0]
			equals(t, "connascence-identity", f.Rule)
			equals(t, "m/auth.Count", f.Subject)
			equals(t, tc.severity, f.Severity)
			equals(
				t,
				"strength 9 (identity), locality system, degree 2: above maxStrength.system 3",
				f.Evidence[0],
			)
		})
	}
}

func TestCandidateVerdicts(t *testing.T) {
	t.Parallel()
	pending := analyze(model(true))
	equals(t, 1, len(pending.Judgements))
	slot := pending.Judgements[0]
	equals(t, []string{"confirmed", "rejected"}, slot.Answers)
	verdict := func(answer string, approved bool, hash string) arch.Verdict {
		return arch.Verdict{
			Criterion: slot.Criterion, Answer: answer, Provenance: arch.ProvenanceLLM,
			EvidenceHash: hash, Approved: approved,
		}
	}
	cases := map[string]struct {
		verdict              arch.Verdict
		findings, candidates int
		rejected             int
		status               arch.SlotStatus
	}{
		"no verdict": {
			verdict:    arch.Verdict{},
			findings:   1,
			candidates: 1,
			status:     arch.SlotPending,
		},
		"confirmed": {
			verdict:    verdict("confirmed", true, slot.EvidenceHash),
			findings:   2,
			candidates: 1,
			status:     arch.SlotSupplied,
		},
		"rejected": {
			verdict:  verdict("rejected", true, slot.EvidenceHash),
			findings: 1,
			rejected: 1,
			status:   arch.SlotSupplied,
		},
		"unapproved": {
			verdict:    verdict("confirmed", false, slot.EvidenceHash),
			findings:   1,
			candidates: 1,
			status:     arch.SlotUnapproved,
		},
		"stale confirmed": {
			verdict:    verdict("confirmed", true, "old"),
			findings:   1,
			candidates: 1,
			status:     arch.SlotStale,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := analyze(model(true), tc.verdict)
			equals(t, tc.findings, len(r.Findings))
			equals(t, tc.candidates, len(r.Candidates))
			equals(t, tc.rejected, len(r.Rejected))
			equals(t, tc.status, r.Judgements[0].Status)
		})
	}
}

// A candidate under every limit asks no reviewer anything.
func TestCandidateUnderTheLimitsHasNoSlot(t *testing.T) {
	t.Parallel()
	m := model(true)
	m.Connascence.MaxStrength[arch.LocalitySystem] = 9
	r := analyze(m)
	equals(t, 0, len(r.Judgements))
	equals(t, 0, len(r.Findings))
	equals(t, "algorithm.same-pattern", r.Candidates[0].Detector)
	equals(t, arch.ConfidenceHigh, r.Candidates[0].Confidence)
}

// Reversing every input slice changes nothing: the report sorts what the
// detectors find.
func TestAnalyzeIgnoresInputOrder(t *testing.T) {
	t.Parallel()
	p := program()
	reversed := program()
	slices.Reverse(reversed.Facts.ConstExprs)
	slices.Reverse(reversed.Facts.GlobalUses)
	a := connascence.Analyze(&arch.Input{Program: *p, Model: *model(true)})
	b := connascence.Analyze(&arch.Input{Program: *reversed, Model: *model(true)})
	equals(t, a, b)
}
