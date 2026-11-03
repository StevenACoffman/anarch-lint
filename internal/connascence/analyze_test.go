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

// eventsOf attests a verdict: by a human when approved, and otherwise only
// by a judge, whose verdict is a draft.
func eventsOf(approved bool) []arch.VerificationEvent {
	if approved {
		return []arch.VerificationEvent{{Role: arch.RoleHuman}}
	}
	return []arch.VerificationEvent{{Role: arch.RoleJudge, Config: "model-v1"}}
}

func TestCandidateVerdicts(t *testing.T) {
	t.Parallel()
	pending := analyze(model(true))
	equals(t, 1, len(pending.Judgements))
	slot := pending.Judgements[0]
	equals(t, []string{"confirmed", "rejected", arch.AnswerInsufficient}, slot.Answers)
	verdict := func(answer string, approved bool, hash string) arch.Verdict {
		return arch.Verdict{
			Criterion:    slot.Criterion,
			Answer:       answer,
			EvidenceHash: hash,
			Events:       eventsOf(approved),
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
		"insufficient evidence": {
			verdict:    verdict(arch.AnswerInsufficient, true, slot.EvidenceHash),
			findings:   1,
			candidates: 1,
			status:     arch.SlotUnresolved,
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

// A candidate under every limit asks no reviewer anything, and the report
// lists its slot as skipped, with why.
func TestCandidateUnderTheLimitsHasNoSlot(t *testing.T) {
	t.Parallel()
	m := model(true)
	m.Connascence.MaxStrength[arch.LocalitySystem] = 9
	r := analyze(m)
	equals(t, 0, len(r.Judgements))
	equals(t, []arch.SkippedSlot{{
		Criterion: "connascence/" + r.Candidates[0].Fingerprint,
		Reason:    "within the limits at system locality, so confirming it would make no finding",
	}}, r.Skipped)
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

// Two constants of a predeclared type, such as uint64, with one value are not
// an enum collision: the type belongs to no package, so to no program.
func TestPredeclaredTypesAreNoEnum(t *testing.T) {
	t.Parallel()
	p := program()
	for _, name := range []string{"a", "b"} {
		p.Facts.Constants = append(p.Facts.Constants, arch.Constant{
			ID: "fx/billing/invoice." + name, Type: "uint64", Value: "7",
			Site: arch.Site{Package: "fx/billing/invoice"},
		})
	}
	r := connascence.Analyze(&arch.Input{Program: *p, Model: *model(true)})
	for i := range r.Instances {
		if r.Instances[i].Detector == "contranascence.enum-collision" &&
			r.Instances[i].Object == "uint64" {
			t.Fatalf("predeclared type reported as an enum: %+v", r.Instances[i])
		}
	}
}

// A run of code dupl finds in two packages is a partial clone, a candidate
// of connascence of algorithm; copies that are whole functions of one body
// shape are left to the whole-body clone detector, and copies within one
// package are not between packages at all.
func TestPartialClones(t *testing.T) {
	t.Parallel()
	span := func(file string, start, end int) arch.Span {
		return arch.Span{File: file, Start: start, End: end}
	}
	cases := map[string]struct {
		group arch.CloneGroup
		want  []string
	}{
		"within functions, across packages": {
			group: arch.CloneGroup{
				Fragments: []arch.Span{span("auth/a.go", 12, 30), span("web/w.go", 5, 23)},
			},
			want: []string{"m/auth.A", "m/web.W"},
		},
		"whole bodies of one shape": {
			group: arch.CloneGroup{
				Fragments: []arch.Span{span("auth/a.go", 10, 40), span("web/w.go", 1, 30)},
			},
		},
		"within one package": {
			group: arch.CloneGroup{
				Fragments: []arch.Span{span("auth/a.go", 12, 20), span("auth/a.go", 22, 30)},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := program()
			p.Packages[0].Files = []string{"auth/a.go"}
			p.Packages[2].Files = []string{"web/w.go"}
			p.Funcs = []arch.Func{
				{ID: "m/auth.A", Package: "m/auth", Span: span("auth/a.go", 10, 40), BodyHash: "h"},
				{ID: "m/web.W", Package: "m/web", Span: span("web/w.go", 1, 30), BodyHash: "h"},
			}
			p.Facts.Clones = []arch.CloneGroup{tc.group}
			var got []string
			for _, c := range connascence.Instances(p, model(false)) {
				if c.Detector != "algorithm.partial-clone" {
					continue
				}
				for _, s := range c.Sites {
					got = append(got, s.Func)
				}
			}
			equals(t, tc.want, got)
		})
	}
}

// Two struct types in different packages whose tags give the same JSON
// names to three or more fields copy one wire format; two names are too few
// to tell from chance.
func TestWireTags(t *testing.T) {
	t.Parallel()
	typ := func(pkg, name string, wire ...string) arch.Type {
		return arch.Type{
			ID: pkg + "." + name, Package: pkg, Name: name, WireNames: wire,
			Span: arch.Span{File: name + ".go", Start: 1, End: 5},
		}
	}
	order := []string{"json:id", "json:total", "json:when"}
	p := program()
	p.Types = append(p.Types,
		typ("m/billing", "Order", order...), typ("m/web", "OrderView", order...),
		typ("m/billing", "Pair", "json:a", "json:b"), typ("m/web", "PairView", "json:a", "json:b"))
	var found []string
	for _, c := range connascence.Instances(p, model(false)) {
		if c.Detector == "meaning.wire-tags" {
			found = append(found, c.Subject+" "+c.Object)
		}
	}
	equals(t, []string{"m/billing.Order json"}, found)
}
