package arch

import (
	"cmp"
	"encoding/json"
	"io"
	"slices"
)

// RuleSet is a file of rules a model includes through RuleSets, so that
// several repositories can keep one copy of the rules they share; a vendored
// copy carries modelith's provenance header (docs/adr/0018). It holds only
// rules that name packages, callees and files themselves, never the
// including model's layers, subsystems or contract files, and each of its
// fitness rules is named, so including it never renames a rule.
type RuleSet struct {
	// Path is the file the set was read from, for messages.
	Path             string            `json:"-"`
	ForbiddenImports []ForbiddenImport `json:"forbiddenImports,omitempty"`
	Isolations       []Isolation       `json:"isolations,omitempty"`
	Fitness          RuleSetFitness    `json:"fitness,omitzero"`
}

// RuleSetFitness are the fitness rules a rule set may hold: those that stand
// on their own patterns.
type RuleSetFitness struct {
	Slices         []Slices         `json:"slices,omitempty"`
	Dependents     []Dependents     `json:"dependents,omitempty"`
	Libraries      []LibraryRule    `json:"libraries,omitempty"`
	Calls          []CallRule       `json:"calls,omitempty"`
	Fields         []FieldRule      `json:"fields,omitempty"`
	UnusedExports  []UnusedExports  `json:"unusedExports,omitempty"`
	InterfacesOnly []InterfacesOnly `json:"interfacesOnly,omitempty"`
	CoChange       []CoChangeRule   `json:"coChange,omitempty"`
	Boundaries     []BoundaryRule   `json:"boundaries,omitempty"`
	FanOut         []FanOutRule     `json:"fanOut,omitempty"`
	Expects        []ExpectRule     `json:"expects,omitempty"`
}

// DecodeRuleSet reads a rule set file. An unknown field, such as a layer
// rule, which names the including model's layers, is ProblemMalformed.
func DecodeRuleSet(r io.Reader, path string) (RuleSet, error) {
	const op = "arch.DecodeRuleSet"
	var set RuleSet
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&set); err != nil {
		return RuleSet{}, &Error{Op: op, Err: InvalidModel(ProblemMalformed,
			"rule set %s: %s", path, err.Error())}
	}
	set.Path = path
	return set, nil
}

// WithRuleSets returns the model with the sets' rules after its own, the
// sets in path order, as the hash reads ruleSets; m is not modified. The
// result is validated as a whole, so a rule set's rule named like one of
// the model's is ProblemDuplicate, and an unnamed fitness rule in a set is
// ProblemIncomplete.
func (m *Model) WithRuleSets(sets []RuleSet) (Model, error) {
	const op = "arch.Model.WithRuleSets"
	out := *m
	out.hash = "" // the merged rules are part of what the model means
	sets = slices.SortedFunc(slices.Values(sets), func(a, b RuleSet) int {
		return cmp.Compare(a.Path, b.Path)
	})
	for i := range sets {
		set := &sets[i]
		if name := set.Fitness.unnamed(); name != "" {
			return Model{}, &Error{Op: op, Err: InvalidModel(ProblemIncomplete,
				"rule set %s: rule %s has no name, which a shared rule needs", set.Path, name)}
		}
		out.ForbiddenImports = slices.Concat(out.ForbiddenImports, set.ForbiddenImports)
		out.Isolations = slices.Concat(out.Isolations, set.Isolations)
		out.Fitness = fitnessWith(&out.Fitness, &set.Fitness)
	}
	if err := out.validate(); err != nil {
		return Model{}, &Error{Op: op, Err: err}
	}
	return out, nil
}

// unnamed returns the default name of the first rule with no name, as
// Rules names it, or "" when every rule is named.
func (f *RuleSetFitness) unnamed() string {
	rules := fitnessWith(&Fitness{}, f)
	for _, r := range rules.Rules() {
		if r.RuleOptions.Name == "" {
			return r.Name
		}
	}
	return ""
}

// fitnessWith returns f with the set's rules appended to its lists; f is
// not modified.
func fitnessWith(f *Fitness, set *RuleSetFitness) Fitness {
	out := *f
	out.Slices = slices.Concat(f.Slices, set.Slices)
	out.Dependents = slices.Concat(f.Dependents, set.Dependents)
	out.Libraries = slices.Concat(f.Libraries, set.Libraries)
	out.Calls = slices.Concat(f.Calls, set.Calls)
	out.Fields = slices.Concat(f.Fields, set.Fields)
	out.UnusedExports = slices.Concat(f.UnusedExports, set.UnusedExports)
	out.InterfacesOnly = slices.Concat(f.InterfacesOnly, set.InterfacesOnly)
	out.CoChange = slices.Concat(f.CoChange, set.CoChange)
	out.Boundaries = slices.Concat(f.Boundaries, set.Boundaries)
	out.FanOut = slices.Concat(f.FanOut, set.FanOut)
	out.Expects = slices.Concat(f.Expects, set.Expects)
	return out
}
