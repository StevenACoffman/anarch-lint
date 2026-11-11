package arch_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// A field rule proposal, as codetospec writes one.
const fieldProposal = `{"id": "p1", "fitness": {"fields": [{"name": "models.Run.Status writers",
	"fields": ["example.com/m/models.Run.Status"], "writes": true, "only": true,
	"from": ["example.com/m/emails"]}]},
	"evidence": [{"kind": "write", "path": "emails/send.go", "lines": "10-20"}]}`

func proposalsFile(proposals ...string) string {
	return `{"schema": "anarch-lint.proposals.v1", "source": "codetospec", "proposals": [` +
		strings.Join(proposals, ",") + `]}`
}

// A proposal holds exactly one named rule, under a unique id, in a file of
// the supported schema.
func TestDecodeProposals(t *testing.T) {
	t.Parallel()
	f, err := arch.DecodeProposals(strings.NewReader(proposalsFile(fieldProposal)))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Proposals) != 1 || f.Source != "codetospec" {
		t.Fatalf("proposals = %+v", f)
	}
	r := f.Proposals[0].Rule()
	if r.Kind != "fields" || r.Name != "models.Run.Status writers" {
		t.Errorf("rule = %s %q, want fields %q", r.Kind, r.Name, "models.Run.Status writers")
	}
	if got := f.Proposals[0].Evidence; len(got) != 1 || got[0].Lines != "10-20" {
		t.Errorf("evidence = %+v", got)
	}

	for name, tc := range map[string]struct{ file, want string }{
		"another schema": {
			`{"schema": "anarch-lint.proposals.v2", "proposals": []}`, "not supported",
		},
		"unknown field":     {`{"schema": "anarch-lint.proposals.v1", "extra": 1}`, "unknown field"},
		"no id":             {proposalsFile(strings.Replace(fieldProposal, `"p1"`, `""`, 1)), "no id"},
		"id used twice":     {proposalsFile(fieldProposal, fieldProposal), "used twice"},
		"no rule":           {proposalsFile(`{"id": "p", "fitness": {}}`), "0 rules"},
		"unnamed rule":      {proposalsFile(`{"id": "p", "fitness": {"fields": [{"fields": ["x.T.F"]}]}}`), "no name"},
		"two rules":         {proposalsFile(`{"id": "p", "fitness": {"fields": [{"name": "a", "fields": ["x.T.F"]}, {"name": "b", "fields": ["x.T.G"]}]}}`), "2 rules"},
		"a layer rule kind": {proposalsFile(`{"id": "p", "fitness": {"layerAccess": [{"name": "a", "layer": "x"}]}}`), "unknown field"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := arch.DecodeProposals(strings.NewReader(tc.file))
			if arch.ErrorCode(err) != arch.EINVALID || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want EINVALID containing %q", err, tc.want)
			}
		})
	}
}

// Accepting a proposal appends its rule to the model's rules of its kind
// and changes the model's hash; a name the model declares is refused.
func TestAccept(t *testing.T) {
	t.Parallel()
	f, err := arch.DecodeProposals(strings.NewReader(proposalsFile(fieldProposal)))
	if err != nil {
		t.Fatal(err)
	}
	p := &f.Proposals[0]
	m := arch.Model{Version: 1, Fitness: arch.Fitness{Fields: []arch.FieldRule{{
		RuleOptions: arch.RuleOptions{Name: "existing"}, Fields: []string{"example.com/m.T.F"},
		From: []string{"example.com/m"},
	}}}}
	if m.Declares(p.Rule().Name) {
		t.Fatal("the model declares the proposal before accepting it")
	}
	got, err := m.Accept(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Fitness.Fields) != 2 || got.Fitness.Fields[1].Name != p.Rule().Name {
		t.Errorf("fields = %+v, want existing then the proposal's", got.Fitness.Fields)
	}
	if !got.Declares(p.Rule().Name) || len(m.Fitness.Fields) != 1 {
		t.Error("Accept should return a new model declaring the rule, leaving m alone")
	}
	if got.Hash() == m.Hash() {
		t.Error("the accepted rule should change the model's hash")
	}

	_, err = got.Accept(p)
	if arch.ErrorCode(err) != arch.EINVALID ||
		!strings.Contains(err.Error(), string(arch.ProblemDuplicate)) {
		t.Errorf("accepting twice: err = %v, want %s", err, arch.ProblemDuplicate)
	}
}
