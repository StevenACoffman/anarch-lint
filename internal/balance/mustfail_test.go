package balance_test

import (
	"cmp"
	"fmt"
	"slices"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
)

// judged analyzes in with a human verdict for each slot answers names,
// taken from a first run that opens the slots.
func judged(t *testing.T, in *balance.Input, answers map[string]string) balance.Report {
	t.Helper()
	first := balance.Analyze(in)
	var verdicts []arch.Verdict
	for i := range first.Judgements {
		if s := &first.Judgements[i]; answers[s.Criterion] != "" {
			answer := answers[s.Criterion]
			verdicts = append(verdicts, arch.Verdict{
				Criterion: s.Criterion, Answer: answer, EvidenceHash: s.EvidenceHash,
				Events: []arch.VerificationEvent{{Role: arch.RoleHuman}},
			})
		}
	}
	if len(verdicts) != len(answers) {
		t.Fatalf("want %d slots open, found %d", len(answers), len(verdicts))
	}
	withVerdicts := *in.Input
	withVerdicts.Verdicts = verdicts
	in.Input = &withVerdicts
	return balance.Analyze(in)
}

// rulesOf names the rules of a report's findings and candidates.
func rulesOf(r *balance.Report) map[string]bool {
	out := map[string]bool{}
	for i := range r.Findings {
		out[r.Findings[i].Rule] = true
	}
	for i := range r.Candidates {
		out[r.Candidates[i].Rule] = true
	}
	return out
}

// A subsystem judged supporting that changed often may change for accidental
// reasons, and one judged core that never changed may be frozen
// (connascence.md §9.4).
func TestChurnCandidates(t *testing.T) {
	t.Parallel()
	commits := make([]string, 20)
	for i := range commits {
		commits[i] = fmt.Sprintf("c%02d", i)
	}
	in := balance.Input{Input: fixture(), Churn: &arch.Churn{
		Since: "a", Head: "b", Commits: map[string][]string{"auth/a.go": commits},
	}}
	r := judged(t, &in, map[string]string{
		"subdomain/auth": "supporting", "subdomain/billing": "core",
	})
	rules := rulesOf(&r)
	equals(t, true, rules[arch.RuleAccidentalVolatility])
	equals(t, true, rules[arch.RuleFrozenCore])
}

// Two bounded contexts that share one's model, with no integration pattern
// that shares it on purpose, leak the model (connascence.md §9.7).
func TestLeakedModel(t *testing.T) {
	t.Parallel()
	in := balance.Input{Input: fixture()}
	r := judged(t, &in, map[string]string{
		"bounded-context/billing": "yes", "bounded-context/auth": "yes",
	})
	equals(t, true, rulesOf(&r)[arch.RuleLeakedModel])
}

// A package ten others import, whose own upstream dependency is intrusive,
// exposes them all to a change behind it (connascence.md §9.6).
func TestUpstreamExposure(t *testing.T) {
	t.Parallel()
	in := fixture()
	for i := range 10 {
		path := fmt.Sprintf("m/x%d", i)
		in.Program.Packages = append(in.Program.Packages, arch.Package{
			Path: path, Name: "x", Module: "m", Files: []string{path + "/x.go"},
		})
		in.Program.Imports = append(in.Program.Imports, arch.Import{
			From: path, To: "m/web", Pos: arch.Position{File: path + "/x.go", Line: 2},
		})
	}
	slices.SortFunc(in.Program.Packages, func(a, b arch.Package) int {
		return cmp.Compare(a.Path, b.Path)
	})
	slices.SortFunc(in.Program.Imports, func(a, b arch.Import) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
	r := balance.Analyze(&balance.Input{Input: in})
	var exposed []string
	for _, f := range r.Findings {
		if f.Rule == arch.RuleUpstreamExposure {
			exposed = append(exposed, f.Subject)
		}
	}
	equals(t, []string{"m/web"}, exposed)
}

// A reflect lookup of an unexported name in an unknown type is a candidate;
// one into a known type of another subsystem is intrusive evidence instead.
func TestReflection(t *testing.T) {
	t.Parallel()
	in := fixture()
	at := arch.Site{File: "web/w.go", Line: 9, Package: "m/web", Func: "m/web.Peek"}
	in.Program.Facts.ReflectByNames = []arch.ReflectByName{
		{Method: "(reflect.Value).FieldByName", Name: "secret", Site: at},
		{Method: "(reflect.Value).FieldByName", Name: "role", Type: "m/auth.Role", Site: at},
	}
	r := balance.Analyze(&balance.Input{Input: in})
	var candidates []string
	for _, c := range r.Candidates {
		if c.Rule == arch.RuleIntrusiveReflection {
			candidates = append(candidates, c.Object)
		}
	}
	equals(t, []string{"secret"}, candidates)
	equals(t, balance.LevelIntrusive, pairOf(t, &r, "web", "auth").Level)
}

// Two subsystems that name one topic to a messaging client the model names
// integrate asynchronously, a fact the pair records.
func TestTopicsAreAsync(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.Model.Balance.Messaging = []string{"example.com/bus"}
	topic := func(pkg string) arch.ConstExpr {
		return arch.ConstExpr{
			Type: "string", Value: `"invoices"`, Context: arch.ContextArgument,
			Callee: "example.com/bus.Publish", Site: arch.Site{File: "x.go", Line: 1, Package: pkg},
		}
	}
	in.Program.Facts.ConstExprs = []arch.ConstExpr{topic("m/billing"), topic("m/web")}
	r := balance.Analyze(&balance.Input{Input: in})
	equals(t, true, pairOf(t, &r, "billing", "web").Async)
}

// With balance.packages, the packages within one subsystem are paired too,
// at the low distance of one subsystem; the pairs across subsystems are
// unchanged.
func TestPackagePairs(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.Model.Balance.Packages = true
	in.Program.Facts.Uses = append(in.Program.Facts.Uses, arch.Use{
		Object: "m/billing.Bill", Kind: arch.ObjectFunc, Package: "m/billing", Exported: true,
		Count: 1, Site: arch.Site{
			File: "cmd/client/main.go", Line: 5, Package: "m/cmd/client",
			Func: "m/cmd/client.main",
		},
	})
	r := balance.Analyze(&balance.Input{Input: in})
	if len(r.PackagePairs) != 1 {
		t.Fatalf("want one pair within billing, got %+v", r.PackagePairs)
	}
	p := r.PackagePairs[0]
	equals(t, "m/cmd/client -> m/billing", p.From+" -> "+p.To)
	equals(t, arch.LocalitySubsystem, p.Locality)
	equals(t, 2, p.Distance)
	without := balance.Analyze(&balance.Input{Input: fixture()})
	equals(t, len(without.Pairs), len(r.Pairs))
}
