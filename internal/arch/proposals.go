package arch

import (
	"encoding/json"
	"fmt"
	"io"
)

// proposalsSchema names version 1 of the proposals format; a file declares
// it in its "schema" key.
const proposalsSchema = "anarch-lint.proposals.v1"

// Proposals is a file of candidate fitness rules another tool proposes for
// a person to review, such as codetospec's rules on who writes the state a
// service stores. Nothing applies a proposal by itself: check reads only
// the model, and a person adds a proposal with accept. A person rejects one
// by accepting it and setting its gate to off, so the decision stays in the
// model and the proposing tool can skip every rule the model declares.
type Proposals struct {
	Schema string `json:"schema"`
	// Source names the tool that wrote the file.
	Source    string     `json:"source"`
	Proposals []Proposal `json:"proposals"`
}

// Proposal is one candidate rule: exactly one named fitness rule of the
// kinds a rule set may hold (rules that stand on their own patterns, never
// the model's layers or subsystems), and the evidence it was proposed from.
type Proposal struct {
	// ID identifies the proposal within its file, for accept.
	ID       string         `json:"id"`
	Fitness  RuleSetFitness `json:"fitness"`
	Evidence []Evidence     `json:"evidence,omitempty"`
}

// Evidence is one thing a proposal rests on, for the person reviewing it:
// code at Path and Lines, or a record of the proposing tool named by ID.
type Evidence struct {
	// Kind says what the evidence is, in the proposing tool's terms, such
	// as "write" or "rule".
	Kind  string `json:"kind"`
	Path  string `json:"path,omitempty"`
	Lines string `json:"lines,omitempty"`
	ID    string `json:"id,omitempty"`
	Note  string `json:"note,omitempty"`
}

// Rule returns the proposal's one rule, with its kind and name.
//
// Requires: p was decoded by DecodeProposals.
func (p *Proposal) Rule() DeclaredRule {
	f := fitnessWith(&Fitness{}, &p.Fitness)
	return f.Rules()[0]
}

// DecodeProposals reads a proposals file. A file of another schema, an
// unknown field, a proposal without an id or with the id of another, or one
// that holds no rule, several rules or an unnamed rule is EINVALID.
//
// Ensures: each proposal holds exactly one named rule, and ids are unique.
func DecodeProposals(r io.Reader) (Proposals, error) {
	const op = "arch.DecodeProposals"
	invalid := func(format string, args ...any) error {
		return &Error{Op: op, Err: &Error{
			Code: EINVALID, Message: "proposals: " + fmt.Sprintf(format, args...),
		}}
	}
	var f Proposals
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return Proposals{}, invalid("%v", err)
	}
	if f.Schema != proposalsSchema {
		return Proposals{}, invalid(
			"schema %q is not supported; want %q",
			f.Schema,
			proposalsSchema,
		)
	}
	seen := map[string]bool{}
	for i := range f.Proposals {
		p := &f.Proposals[i]
		switch {
		case p.ID == "":
			return Proposals{}, invalid("proposal %d has no id", i)
		case seen[p.ID]:
			return Proposals{}, invalid("id %s is used twice", p.ID)
		}
		seen[p.ID] = true
		one := fitnessWith(&Fitness{}, &p.Fitness)
		rules := one.Rules()
		if len(rules) != 1 {
			return Proposals{}, invalid("proposal %s holds %d rules, not one", p.ID, len(rules))
		}
		if rules[0].RuleOptions.Name == "" {
			return Proposals{}, invalid("proposal %s: its rule has no name", p.ID)
		}
	}
	return f, nil
}

// Accept returns the model with the proposal's rule after the model's own
// rules of its kind; m is not modified. A name the model already declares
// is ProblemDuplicate, as for a rule set.
func (m *Model) Accept(p *Proposal) (Model, error) {
	const op = "arch.Model.Accept"
	out := *m
	out.hash = "" // the accepted rule is part of what the model means
	out.Fitness = fitnessWith(&out.Fitness, &p.Fitness)
	if err := out.validate(); err != nil {
		return Model{}, &Error{Op: op, Err: err}
	}
	return out, nil
}

// Declares reports whether the model declares a fitness rule named name.
func (m *Model) Declares(name string) bool {
	for _, r := range m.Fitness.Rules() {
		if r.Name == name {
			return true
		}
	}
	return false
}
