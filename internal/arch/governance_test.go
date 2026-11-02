package arch_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestGovern: a forbidden import's entry and a findingRules entry each set
// the gate and quality attribute of their own rule's findings, and no other
// rule's; an off gate drops the finding, and an empty one keeps the rule's
// severity.
func TestGovern(t *testing.T) {
	t.Parallel()
	m := arch.Model{
		ForbiddenImports: []arch.ForbiddenImport{{
			From: []string{"m/a"}, Imports: []string{"m/b"}, Reason: "r",
			Gate: arch.GateWarn, QualityAttribute: arch.QualitySecurity,
		}},
		Isolations: []arch.Isolation{{From: "m/$*", Reason: "r", Gate: arch.GateOff}},
		FindingRules: map[arch.RuleName]arch.FindingRule{
			arch.RuleLargeFunc: {
				Gate: arch.GateFail, Reason: "small functions review faster",
				QualityAttribute: arch.QualityMaintainability,
			},
			arch.RuleDeepNesting: {QualityAttribute: arch.QualityMaintainability},
			arch.RuleLargeFile:   {Gate: arch.GateOff},
		},
	}
	finding := func(rule string, sev arch.Severity, modelRule string) arch.Finding {
		f := arch.NewFinding(arch.FindingID{Rule: rule, Subject: "s"}, sev)
		f.Message, f.ModelRule = "m", modelRule
		return f
	}
	type got struct {
		rule     string
		severity arch.Severity
		quality  arch.QualityAttribute
		message  string
	}
	var out []got
	for _, f := range m.Govern([]arch.Finding{
		finding(arch.RuleForbiddenImport, arch.SeverityError, "forbiddenImports[0]"),
		finding(arch.RuleRedundant, arch.SeverityWarning, "forbiddenImports[0]"),
		finding(arch.RuleIsolation, arch.SeverityError, "isolations[0]"),
		finding(arch.RuleLargeFunc, arch.SeverityWarning, ""),
		finding(arch.RuleDeepNesting, arch.SeverityWarning, ""),
		finding(arch.RuleLargeFile, arch.SeverityWarning, ""),
		finding(arch.RuleLargeType, arch.SeverityWarning, ""),
	}) {
		out = append(out, got{f.Rule, f.Severity, f.QualityAttribute, f.Message})
	}
	equals(t, []got{
		{arch.RuleForbiddenImport, arch.SeverityWarning, arch.QualitySecurity, "m"},
		{arch.RuleRedundant, arch.SeverityWarning, "", "m"},
		{
			arch.RuleLargeFunc, arch.SeverityError, arch.QualityMaintainability,
			"m: small functions review faster",
		},
		{arch.RuleDeepNesting, arch.SeverityWarning, arch.QualityMaintainability, "m"},
		{arch.RuleLargeType, arch.SeverityWarning, "", "m"},
	}, out)
}

// TestReasonedRules: every declared rule is listed with its reason, the
// structural ones under the findingRules entry that holds it.
func TestReasonedRules(t *testing.T) {
	t.Parallel()
	m := arch.Model{
		Layers: []arch.Layer{{Name: "a"}, {Name: "b"}},
		ForbiddenImports: []arch.ForbiddenImport{
			{From: []string{"x"}, Imports: []string{"y"}, Reason: "r"},
		},
		FindingRules: map[arch.RuleName]arch.FindingRule{
			arch.RuleLayerOrder: {Reason: "layers", QualityAttribute: arch.QualityMaintainability},
			arch.RuleLargeFunc:  {Gate: arch.GateFail},
		},
		Fitness: arch.Fitness{Calls: []arch.CallRule{{
			RuleOptions: arch.RuleOptions{Name: "c", Reason: "clock"},
			From:        []string{"x"}, Callees: []string{"time.Now"},
		}}},
	}
	equals(t, []arch.ReasonedRule{
		{
			Name: "layers", Reason: "layers", QualityAttribute: arch.QualityMaintainability,
			FindingRule: arch.RuleLayerOrder,
		},
		{Name: "forbiddenImports[0]", Reason: "r"},
		{Name: "fitness.calls c", Reason: "clock"},
		{Name: "findingRules.large-func", FindingRule: arch.RuleLargeFunc},
	}, m.ReasonedRules())
}

func TestDecodeFindingRules(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		input   string
		message string // "" when the model is valid
	}{
		"a structural rule": {
			input: `{"version": 1, "findingRules": {"layer-order-violation":
				{"gate": "warn", "reason": "r", "qualityAttribute": "maintainability"}}}`,
		},
		"an unknown rule": {
			input:   `{"version": 1, "findingRules": {"no-such-rule": {}}}`,
			message: `model: unknown-reference: findingRules names unknown rule "no-such-rule"`,
		},
		"a rule its entry governs": {
			input: `{"version": 1, "findingRules": {"forbidden-import": {}}}`,
			message: `model: unknown-reference: findingRules names "forbidden-import", ` +
				`which its forbiddenImports entry governs`,
		},
		"an unknown gate": {
			input:   `{"version": 1, "findingRules": {"large-func": {"gate": "maybe"}}}`,
			message: `model: unknown-value: findingRules "large-func" gate "maybe" is not fail, warn or off`,
		},
		"an unknown quality attribute": {
			input: `{"version": 1, "forbiddenImports": [{"from": ["x"], "imports": ["y"],
				"reason": "r", "qualityAttribute": "speed"}]}`,
			message: "model: unknown-value: speed is not a quality attribute",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := arch.DecodeModel(strings.NewReader(tc.input))
			if tc.message == "" {
				equals(t, nil, err)
				return
			}
			equals(t, arch.EINVALID, arch.ErrorCode(err))
			equals(t, tc.message, arch.ErrorMessage(err))
		})
	}
}
