package fitness

import (
	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// ProposalsSchema names version 1 of the proposals report.
const ProposalsSchema = "anarch-lint.proposals-report.v1"

// ProposalsReport says what each proposed rule would do if a person
// accepted it: whether the model already declares a rule of its name, and
// otherwise its outcome on the code as it is.
type ProposalsReport struct {
	Schema string      `json:"schema"`
	Inputs arch.Inputs `json:"inputs"`
	// Source is the tool that wrote the proposals.
	Source    string           `json:"source"`
	Proposals []ProposalStatus `json:"proposals"`
	// Findings are the violations of the proposals not yet declared.
	Findings []arch.Finding `json:"findings"`
}

// ProposalStatus is one proposal's outcome.
type ProposalStatus struct {
	ID   string `json:"id"`
	Rule string `json:"rule"`
	Kind string `json:"kind"`
	// Declared is true when the model already declares a rule of this
	// name, accepted or rejected (gate off): the proposal is not analyzed.
	Declared bool `json:"declared,omitempty"`
	// Result is the rule's outcome as fitness reports it; nil when
	// declared.
	Result   *RuleResult `json:"result,omitempty"`
	Evidence int         `json:"evidence"`
}

// AnalyzeProposals measures the proposals against the loaded code: every
// proposal the model does not declare is added to a copy of the model, as
// accept would add it, and the copy is analyzed. A proposal whose rule the
// model cannot hold (an incomplete rule, a name another proposal also
// uses) is EINVALID.
//
// Requires: in was loaded with every fact (pipeline.AllFacts).
// Ensures:  one status per proposal, in the file's order; findings are
// those of undeclared proposals, as Analyze orders them.
func AnalyzeProposals(in *arch.Input, props *arch.Proposals) (ProposalsReport, error) {
	const op = "fitness.AnalyzeProposals"
	measured := *in
	statuses := make([]ProposalStatus, len(props.Proposals))
	proposed := map[string]bool{}
	for i := range props.Proposals {
		p := &props.Proposals[i]
		rule := p.Rule()
		statuses[i] = ProposalStatus{
			ID: p.ID, Rule: rule.Name, Kind: rule.Kind, Evidence: len(p.Evidence),
			Declared: in.Model.Declares(rule.Name),
		}
		if statuses[i].Declared {
			continue
		}
		m, err := measured.Model.Accept(p)
		if err != nil {
			return ProposalsReport{}, &arch.Error{Op: op + " " + p.ID, Err: err}
		}
		measured.Model = m
		proposed["fitness."+rule.Kind+" "+rule.Name] = true
	}
	r := Analyze(&measured)
	results := map[string]RuleResult{}
	for _, res := range r.Rules {
		results[res.Name] = res
	}
	for i := range statuses {
		if res, ok := results[statuses[i].Rule]; ok && !statuses[i].Declared {
			statuses[i].Result = &res
		}
	}
	findings := make([]arch.Finding, 0)
	for i := range r.Findings {
		if proposed[r.Findings[i].ModelRule] {
			findings = append(findings, r.Findings[i])
		}
	}
	inputs := r.Inputs
	inputs.ModelHash = in.Model.Hash() // the model as written, not the measured copy
	return ProposalsReport{
		Schema: ProposalsSchema, Inputs: inputs, Source: props.Source,
		Proposals: statuses, Findings: findings,
	}, nil
}
