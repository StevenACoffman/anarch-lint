package balance_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
)

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// fixture is three subsystems of one module. web imports auth's internal
// package, bypassing auth's interface; billing uses auth's model type Role
// and calls auth.Version, which exposes none; two binaries hold billing and
// web apart, and a clone spans them.
func fixture() *arch.Input {
	site := func(file string, line int, pkg, fn string) arch.Site {
		return arch.Site{File: file, Line: line, Package: pkg, Func: fn}
	}
	return &arch.Input{
		Program: arch.Program{
			Packages: []arch.Package{
				{Path: "m/auth", Name: "auth", Module: "m", Files: []string{"auth/a.go"}},
				{
					Path:   "m/auth/internal/store",
					Name:   "store",
					Module: "m",
					Files:  []string{"auth/internal/store/s.go"},
				},
				{Path: "m/billing", Name: "billing", Module: "m", Files: []string{"billing/b.go"}},
				{
					Path:   "m/cmd/client",
					Name:   "main",
					Module: "m",
					Files:  []string{"cmd/client/main.go"},
				},
				{
					Path:   "m/cmd/server",
					Name:   "main",
					Module: "m",
					Files:  []string{"cmd/server/main.go"},
				},
				{Path: "m/web", Name: "web", Module: "m", Files: []string{"web/w.go"}},
			},
			Types: []arch.Type{
				{
					ID: "m/auth.Role", Package: "m/auth", Name: "Role", Kind: arch.KindOther,
					Span: arch.Span{File: "auth/a.go", Start: 3, End: 3},
				},
			},
			Imports: []arch.Import{
				{
					From: "m/billing",
					To:   "m/auth",
					Pos:  arch.Position{File: "billing/b.go", Line: 2},
				},
				{
					From: "m/cmd/client",
					To:   "m/billing",
					Pos:  arch.Position{File: "cmd/client/main.go", Line: 2},
				},
				{
					From: "m/cmd/server",
					To:   "m/web",
					Pos:  arch.Position{File: "cmd/server/main.go", Line: 2},
				},
				{
					From: "m/web",
					To:   "m/auth/internal/store",
					Pos:  arch.Position{File: "web/w.go", Line: 2},
				},
			},
			Facts: arch.Facts{
				Uses: []arch.Use{
					{
						Object:   "m/auth.Role",
						Kind:     arch.ObjectType,
						Package:  "m/auth",
						Exported: true,
						Count:    1,
						Site:     site("billing/b.go", 5, "m/billing", "m/billing.Bill"),
					},
					{
						Object:   "m/auth.Version",
						Kind:     arch.ObjectFunc,
						Package:  "m/auth",
						Exported: true,
						Count:    1,
						Site:     site("billing/b.go", 6, "m/billing", "m/billing.Bill"),
					},
				},
				Signatures: []arch.Signature{
					{
						Func:     "m/auth.Version",
						Exported: true,
						Results:  []arch.TypeRef{{Type: "string", Basic: true}},
					},
				},
			},
		},
		Model: arch.Model{Version: 1, Subsystems: []arch.Subsystem{
			{
				Name:      "auth",
				Packages:  []string{"m/auth/..."},
				Interface: []string{"m/auth"},
				Owner:     "@team",
			},
			{Name: "billing", Packages: []string{"m/billing", "m/cmd/client"}, Owner: "@team"},
			{Name: "web", Packages: []string{"m/web", "m/cmd/server"}, Owner: "@web"},
		}},
	}
}

// clone is a connascence candidate spanning billing and web, which no binary
// holds together.
func clone() arch.Connascence {
	return arch.Connascence{
		Fingerprint: "c10e", Kind: arch.StrengthAlgorithm, Strength: 5, Detector: "algorithm.clone",
		Subject: "m/billing.Total", Basis: arch.BasisCandidate, Message: "two bodies of one shape",
		Sites: []arch.Site{
			{File: "billing/b.go", Line: 9, Package: "m/billing", Func: "m/billing.Total"},
			{File: "web/w.go", Line: 9, Package: "m/web", Func: "m/web.Tally"},
		},
	}
}

func analyze(in *arch.Input, extra ...arch.Connascence) balance.Report {
	return balance.Analyze(&balance.Input{Input: in, Connascence: extra})
}

func pairOf(t *testing.T, r *balance.Report, from, to string) balance.Pair {
	t.Helper()
	for i := range r.Pairs {
		if p := r.Pairs[i]; p.From == from && p.To == to {
			return p
		}
	}
	t.Fatalf("no pair %s -> %s among %d", from, to, len(r.Pairs))
	return balance.Pair{}
}

func TestLevels(t *testing.T) {
	t.Parallel()
	r := analyze(fixture(), clone())
	cases := map[string]struct {
		from, to string
		level    balance.Level
		basis    arch.Basis
	}{
		"a model type crossing": {
			"billing",
			"auth",
			balance.LevelModel,
			arch.BasisFact,
		},
		"an import bypassing the interface": {
			"web",
			"auth",
			balance.LevelIntrusive,
			arch.BasisFact,
		},
		"a clone": {
			"billing",
			"web",
			balance.LevelFunctional,
			arch.BasisCandidate,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := pairOf(t, &r, tc.from, tc.to)
			equals(t, tc.level, p.Level)
			equals(t, tc.basis, p.Basis)
		})
	}
}

func TestDistance(t *testing.T) {
	t.Parallel()
	r := analyze(fixture(), clone())
	sameTeam := pairOf(t, &r, "billing", "auth")
	equals(t, false, sameTeam.OwnerBoundary)
	equals(t, 4, sameTeam.Distance)
	equals(t, balance.QuadrantLowCohesion, sameTeam.Quadrant)

	otherTeam := pairOf(t, &r, "web", "auth")
	equals(t, true, otherTeam.OwnerBoundary)
	equals(t, false, otherTeam.DeployBoundary) // an import cannot cross one
	equals(t, balance.QuadrantTightCoupling, otherTeam.Quadrant)

	clones := pairOf(t, &r, "billing", "web")
	equals(t, true, clones.DeployBoundary)
	equals(t, true, clones.Symmetric)
	equals(t, 9, clones.Distance)
	equals(t, 1, clones.Score) // max(|9−9|, 10−10) + 1: the distributed monolith
}

func TestTightCouplingGatesOnFactsAndVerdicts(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.Model.Balance.Gate = true
	r := analyze(in, clone())
	rules := map[string]arch.Severity{}
	for _, f := range r.Findings {
		rules[f.Rule+" "+f.Subject+"->"+f.Object] = f.Severity
	}
	equals(
		t,
		map[string]arch.Severity{"tight-coupling-intrusive web->auth": arch.SeverityError},
		rules,
	)

	var slot arch.JudgementSlot
	for _, s := range r.Judgements {
		if strings.HasPrefix(s.Criterion, "balance/") {
			slot = s
		}
	}
	confirm := arch.Verdict{
		Criterion: slot.Criterion, Answer: "confirmed",
		EvidenceHash: slot.EvidenceHash, Events: []arch.VerificationEvent{{Role: arch.RoleHuman}},
	}
	in.Verdicts = []arch.Verdict{confirm}
	confirmed := analyze(in, clone())
	equals(t, 2, len(confirmed.Findings))
	equals(t, "tight-coupling-functional", confirmed.Findings[0].Rule)
}

// A supporting subdomain may be tightly coupled: its low volatility balances
// the strength and the distance (connascence.md §9.5).
func TestLowVolatilitySuppresses(t *testing.T) {
	t.Parallel()
	in := fixture()
	r := analyze(in)
	var slot arch.JudgementSlot
	for _, s := range r.Judgements {
		if s.Criterion == "subdomain/auth" {
			slot = s
		}
	}
	equals(t, []string{"core", "generic", "supporting", arch.AnswerInsufficient}, slot.Answers)
	in.Verdicts = []arch.Verdict{
		{
			Criterion:    "subdomain/auth",
			Answer:       "supporting",
			EvidenceHash: slot.EvidenceHash,
			Events:       []arch.VerificationEvent{{Role: arch.RoleHuman}},
		},
	}
	judged := analyze(in)
	equals(t, 0, len(judged.Findings))
	equals(t, "supporting", pairOf(t, &judged, "web", "auth").Volatility)
}

func TestRejectedConnascenceIsLeftOut(t *testing.T) {
	t.Parallel()
	r := balance.Analyze(&balance.Input{
		Input: fixture(), Connascence: []arch.Connascence{clone()},
		Rejected: []string{"c10e"},
	})
	for _, p := range r.Pairs {
		if p.From == "billing" && p.To == "web" {
			t.Fatalf("a rejected clone still couples billing and web: %+v", p)
		}
	}
}

func TestOwnersDiffer(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.Model.Subsystems[1].Owner = ""
	r := balance.Analyze(&balance.Input{Input: in, Owners: map[string][]string{
		"m/billing": {"@money"}, "m/cmd/client": {"@platform"},
	}})
	equals(t, "subsystem-owners-differ", r.Findings[0].Rule)
	equals(t, []string{"owners: @money", "owners: @platform"}, r.Findings[0].Evidence)
}

// A slot opens only where its answer can change something, and the report
// says which were skipped and why. Without the clone, only auth is
// upstream, and only billing and auth share a model.
func TestSlotsOpenOnlyWhereTheyDecide(t *testing.T) {
	t.Parallel()
	r := analyze(fixture())
	var opened []string
	for _, s := range r.Judgements {
		if strings.HasPrefix(s.Criterion, "subdomain/") ||
			strings.HasPrefix(s.Criterion, "bounded-context/") {
			opened = append(opened, s.Criterion)
		}
	}
	equals(t, []string{"bounded-context/auth", "bounded-context/billing", "subdomain/auth"}, opened)
	var skipped []string
	for _, s := range r.Skipped {
		equals(t, true, s.Reason != "")
		skipped = append(skipped, s.Criterion)
	}
	equals(t, []string{"subdomain/billing", "subdomain/web", "bounded-context/web"}, skipped)

	// A revision range gives every subsystem's churn, whose candidates read
	// the subdomain, so every subdomain slot opens.
	churned := balance.Input{Input: fixture(), Churn: &arch.Churn{Since: "a", Head: "b"}}
	withChurn := balance.Analyze(&churned)
	for _, s := range withChurn.Skipped {
		equals(t, false, strings.HasPrefix(s.Criterion, "subdomain/"))
	}
}
