package pipeline

import (
	"bytes"
	"path/filepath"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/indomain/pkg/provenance"
)

// includeRuleSets returns m with the rules of the rule sets it names,
// relative to the model file at path, each read with read and its
// provenance checked (docs/adr/0018). read lets the bars read the sets at a
// revision as the work tree has them now.
func includeRuleSets(
	m *arch.Model, path string, read func(string) ([]byte, error),
) (arch.Model, error) {
	const op = "pipeline.includeRuleSets"
	if len(m.RuleSets) == 0 {
		return *m, nil
	}
	sets := make([]arch.RuleSet, 0, len(m.RuleSets))
	for _, rel := range m.RuleSets {
		file := filepath.Join(filepath.Dir(path), rel)
		src, err := read(file)
		if err != nil {
			return arch.Model{}, &arch.Error{Op: op + " " + rel, Err: err}
		}
		if err := checkProvenance(rel, src); err != nil {
			return arch.Model{}, &arch.Error{Op: op, Err: err}
		}
		set, err := decodeRuleSetFile(rel, src)
		if err != nil {
			return arch.Model{}, &arch.Error{Op: op + " " + rel, Err: err}
		}
		sets = append(sets, set)
	}
	out, err := m.WithRuleSets(sets)
	if err != nil {
		return arch.Model{}, &arch.Error{Op: op, Err: err}
	}
	return out, nil
}

// checkProvenance accepts a rule set with no provenance header, which this
// repository owns, and a vendored copy whose header is sound and whose
// content still hashes to the digest it records: a copy edited here has
// drifted from its origin. It reads nothing but src, as modelith checks its
// vendored models offline (its ADR-0011 and ADR-0015).
func checkProvenance(path string, src []byte) error {
	const op = "pipeline.checkProvenance"
	h, problems := provenance.Parse(src)
	if len(problems) > 0 {
		return &arch.Error{Op: op, Err: arch.InvalidModel(arch.ProblemProvenance,
			"rule set %s: provenance header line %d: %s",
			path, problems[0].Line, problems[0].Message)}
	}
	if h == nil {
		return nil
	}
	if ok, got := h.Verify(src); !ok {
		return &arch.Error{Op: op, Err: arch.InvalidModel(arch.ProblemProvenance,
			"rule set %s no longer matches the digest its provenance header records "+
				"(recorded %s, computed %s): restore it from %s, or delete the header if the "+
				"change is a deliberate fork, which makes this repository its home",
			path, h.Digest, got, h.Origin)}
	}
	return nil
}

// decodeRuleSetFile decodes a rule set's contents: JSON, or YAML when the
// file is named *.yaml or *.yml, read as the JSON it stands for, as a model
// file is. Only a YAML rule set can carry a provenance header, a comment.
func decodeRuleSetFile(path string, src []byte) (arch.RuleSet, error) {
	const op = "pipeline.decodeRuleSetFile"
	js, err := modelJSON(path, src)
	if err != nil {
		return arch.RuleSet{}, &arch.Error{Op: op, Err: err}
	}
	set, err := arch.DecodeRuleSet(bytes.NewReader(js), path)
	if err != nil {
		return arch.RuleSet{}, &arch.Error{Op: op, Err: err}
	}
	return set, nil
}
