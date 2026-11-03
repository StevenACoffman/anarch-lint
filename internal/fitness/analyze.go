// Package fitness measures a Go program the way ArchUnit tests a Java one:
// Lakos, visibility and component dependency metrics, elementary cycles, and
// the model's fitness rules (layer access, an onion architecture, slices,
// dependents, calls and interface-only use), each with a reason, a gate and
// a result. Unlike ArchUnit, every list is in a fixed order, so the report
// is byte-identical across runs. It is a pure functional core over
// [arch.Program]; the shell loads it.
package fitness

import (
	"cmp"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Schema names the fitness report format.
const Schema = "anarch-lint.fitness.v1"

// Caps on cycles, after ArchUnit's cycles.maxNumberToDetect and
// cycles.maxNumberOfDependenciesPerEdge; the first ones in sorted order are
// kept, so the caps are deterministic.
const (
	maxCycles       = 100
	maxEdgeEvidence = 20
)

// Statuses of a rule, as ArchUnit's report gives them.
const (
	StatusPassed RuleStatus = "passed"
	StatusFailed RuleStatus = "failed"
	StatusEmpty  RuleStatus = "empty"
	StatusOff    RuleStatus = "off"
	// StatusNoChange is a rule on changes measured without a change set: it
	// had nothing to check.
	StatusNoChange RuleStatus = "no-change"
)

// RuleStatus is a fitness rule's outcome.
type RuleStatus string

// Report is the complete, deterministic result of the fitness measures.
type Report struct {
	Schema string      `json:"schema"`
	Inputs arch.Inputs `json:"inputs"`
	// Rules lists every declared fitness rule with its outcome, first, as
	// ArchUnit leads with its rules.
	Rules    []RuleResult   `json:"rules"`
	Findings []arch.Finding `json:"findings"`
	// Lakos holds the metrics of the type, package and subsystem graphs.
	Lakos                      []Lakos               `json:"lakos"`
	Visibility                 Visibility            `json:"visibility"`
	ComponentDependencyMetrics []ComponentDependency `json:"componentDependencyMetrics"`
	// Cycles lists the elementary cycles of the type and subsystem graphs.
	Cycles []CycleSet `json:"cycles"`
	// Verification says whether the import rules were checked against each
	// other for redundancy and unusable allowances.
	Verification Verification      `json:"verification"`
	Assumptions  []arch.Assumption `json:"assumptions"`
}

// RuleResult is one fitness rule's outcome: how many packages or calls its
// subject selectors matched, and how many violations it found.
type RuleResult struct {
	Name       string        `json:"name"`
	Kind       string        `json:"kind"`
	Reason     string        `json:"reason,omitempty"`
	Gate       arch.RuleGate `json:"gate"`
	Optional   bool          `json:"optional,omitempty"`
	Matched    int           `json:"matched"`
	Violations int           `json:"violations"`
	Status     RuleStatus    `json:"status"`
	// Truncated marks a rule that stopped at a cap, the first maxCycles
	// cycles in sorted order: its violations are a lower bound.
	Truncated bool `json:"truncated,omitempty"`
}

// CycleSet is the elementary cycles of one graph, at most maxCycles of them.
type CycleSet struct {
	Level     string     `json:"level"`
	Cycles    [][]string `json:"cycles"`
	Truncated bool       `json:"truncated"`
}

// subject is what the checks read, and the rules and findings they add to.
type subject struct {
	p     *arch.Program
	m     *arch.Model
	g     arch.Graphs
	types map[string]*arch.Type
	*ledger
	// slices holds the rule record of each slices rule, by position.
	slices map[int]*rule
}

// ledger keeps the rules in declaration order and the findings of their
// violations.
type ledger struct {
	rules    []*rule
	findings []arch.Finding
}

// rule is one fitness rule while it is checked.
type rule struct {
	result RuleResult
	// off rules check nothing; severity is the severity of a violation.
	off      bool
	severity arch.Severity
	// quality is the quality attribute the rule protects.
	quality arch.QualityAttribute
	// quiet is the status of a rule without violations that need not match
	// anything, such as a rule on changes; "" for every other rule.
	quiet RuleStatus
}

// Analyze measures in's program under its model's fitness rules. It is a
// pure function: the same input always gives the same report, and the input
// is not modified.
//
// Requires: in.Program is sorted, as [arch.Program] documents, and carries
// its [arch.Facts] when the model declares calls or interfacesOnly rules.
func Analyze(in *arch.Input) Report {
	s := newSubject(in)
	constraints := s.importConstraints()
	s.checkImports(constraints)
	verification, overlaps := verify(constraints, s.m.ForbiddenImports, s.packagePaths(),
		outsideTargets(s.p, &s.g))
	s.findings = append(s.findings, overlaps...)
	s.checkSlices()
	s.checkCalls()
	checkFields(s.ledger, s.m.Fitness.Fields, s.p.Facts.Uses, s.packagePaths())
	checkUnusedExports(s.ledger, s.m.Fitness.UnusedExports, s.p)
	s.checkInterfacesOnly()
	s.checkAssigned()
	s.checkDomains(in.Domains)
	checkOpenAPI(
		s.ledger,
		s.m.Fitness.OpenAPI,
		in.APIs,
		&s.p.Facts,
		s.packagePaths(),
	)
	checkGraphQL(s.ledger, s.m.Fitness.GraphQL, in.GraphQL, s.p.Funcs, s.packagePaths())
	checkProtobuf(s.ledger, s.m.Fitness.Protobuf, in.Protos, &s.p.Facts, s.packagePaths())
	checkCases(s.ledger, in, s.packagePaths())
	checkCoChange(s.ledger, s.m.Fitness.CoChange, in.Changed)
	checkBoundaries(s.ledger, s.m.Fitness.Boundaries, in.Changed, &s.p.Facts, s.p.Imports)
	checkFanOut(s.ledger, s.m.Fitness.FanOut, s.packagePaths(), importsInScope(s.p, &s.g))
	(&existing{p: s.p, m: s.m, types: s.types, packages: s.packagePaths(), ledger: s.ledger}).check()
	s.unexplained(s.m)
	rules := s.results() // closes every rule, adding the empty-rule findings

	inputs := in.Inputs
	inputs.ModelHash, inputs.Thresholds = s.m.Hash(), s.m.Limits()
	inputs.Tags = append(make([]string, 0, len(in.Inputs.Tags)), in.Inputs.Tags...)
	inputs.Patterns = append(make([]string, 0, len(in.Inputs.Patterns)), in.Inputs.Patterns...)
	return Report{
		Schema:   Schema,
		Inputs:   inputs,
		Rules:    rules,
		Findings: s.m.Govern(s.findings),
		Lakos: []Lakos{
			lakosOf("types", s.g.Types), lakosOf("packages", s.g.Packages),
			lakosOf("subsystems", s.g.Subsystems),
		},
		Visibility:                 visibilityOf(s.p),
		ComponentDependencyMetrics: componentDependencyOf(s.p, &s.g),
		Cycles: []CycleSet{
			cyclesOf("types", s.g.Types),
			cyclesOf("subsystems", s.g.Subsystems),
		},
		Verification: verification,
		Assumptions:  assumptions(),
	}
}

// AddTo adds the report's findings to the gate, and ratchets NCCD at the
// package and subsystem levels and the number of elementary cycles at the
// type and subsystem levels: lower is better for each.
func (r *Report) AddTo(in *arch.GateInput) {
	var metrics []arch.Metric
	for _, l := range r.Lakos {
		if l.Level != "types" {
			metrics = append(metrics, arch.Metric{Name: "fitness.nccd." + l.Level, Value: l.NCCD})
		}
	}
	for _, c := range r.Cycles {
		metrics = append(
			metrics,
			arch.Metric{Name: "fitness.cycles." + c.Level, Value: float64(len(c.Cycles))},
		)
	}
	in.Add(r.Findings, metrics)
}

// newSubject builds the graphs and lookups the checks share.
func newSubject(in *arch.Input) *subject {
	p, m := &in.Program, &in.Model
	s := &subject{
		p:      p,
		m:      m,
		g:      arch.BuildGraphs(p, m),
		types:  map[string]*arch.Type{},
		slices: map[int]*rule{},
		ledger: &ledger{},
	}
	for i := range p.Types {
		s.types[p.Types[i].ID] = &p.Types[i]
	}
	return s
}

// addRule registers a rule; its matched count and violations are filled in
// as the checks run.
func (s *ledger) addRule(kind string, opts arch.RuleOptions, name string) *rule {
	r := &rule{result: RuleResult{
		Name: name, Kind: kind, Reason: opts.Reason, Gate: cmp.Or(opts.Gate, arch.GateFail),
		Optional: opts.Optional,
	}}
	severity, on := opts.Gate.Severity()
	r.severity, r.off, r.quality = severity, !on, opts.QualityAttribute
	s.rules = append(s.rules, r)
	return r
}

// violation records one violation of r as a finding, unless r is off.
func (s *ledger) violation(r *rule, f *arch.Finding) {
	if r.off {
		return
	}
	r.result.Violations++
	f.Severity = r.severity
	f.ModelRule = r.modelRule()
	f.QualityAttribute = r.quality
	if r.result.Reason != "" {
		f.Message += ": " + r.result.Reason
	}
	s.findings = append(s.findings, *f)
}

// modelRule names the rule as a finding's ModelRule: its kind under fitness,
// and its name when that differs.
func (r *rule) modelRule() string {
	return (&arch.DeclaredRule{Kind: r.result.Kind, Name: r.result.Name}).ModelRule()
}

// results closes every rule: an empty rule that is not optional is a
// finding of its own, and each rule gets its status.
func (s *ledger) results() []RuleResult {
	out := make([]RuleResult, 0, len(s.rules))
	for _, r := range s.rules {
		switch {
		case r.off:
			r.result.Status = StatusOff
		case r.quiet != "" && r.result.Violations == 0:
			r.result.Status = r.quiet
		case r.result.Matched == 0 && !r.result.Optional:
			r.result.Status = StatusEmpty
			s.emptyRule(r)
		case r.result.Violations > 0:
			r.result.Status = StatusFailed
		default:
			r.result.Status = StatusPassed
		}
		out = append(out, r.result)
	}
	arch.SortFindings(s.findings)
	return out
}

// emptyRule reports a rule whose subject selectors match nothing, which
// ArchUnit fails by default (failOnEmptyShould): a rule that checks nothing
// is usually a typo.
func (s *ledger) emptyRule(r *rule) {
	f := arch.NewFinding(
		arch.FindingID{Rule: arch.RuleEmptySelector, Subject: r.result.Name},
		r.severity,
	)
	f.ModelRule = r.modelRule()
	f.Message = "fitness rule " + r.result.Name + " (" + r.result.Kind + ") matches nothing"
	f.Fix = "correct the rule's patterns, or set optional if it may match nothing"
	s.findings = append(s.findings, f)
}

// unexplained warns of each declared rule that gives no reason: a rule
// nobody can explain is hard to keep and hard to drop, as modelith's
// completeness layer has it. The warning is advice, so it never gates. A
// fitness rule is named by its own name, as its findings are; any other rule
// by where the model declares it.
func (s *ledger) unexplained(m *arch.Model) {
	const fix = "add a reason saying what the rule protects and why"
	warn := func(subject, message, fix string) {
		f := arch.NewFinding(
			arch.FindingID{Rule: arch.RuleWithoutReason, Subject: subject},
			arch.SeverityWarning,
		)
		f.Message, f.Fix = message, fix
		s.findings = append(s.findings, f)
	}
	fitness := map[string]bool{}
	for _, r := range m.Fitness.Rules() {
		fitness[r.ModelRule()] = true
		if r.Reason == "" {
			warn(r.Name, "fitness rule "+r.Name+" ("+r.Kind+") gives no reason", fix)
		}
	}
	for _, r := range m.ReasonedRules() {
		switch {
		case r.Reason != "" || fitness[r.Name]:
		case r.FindingRule != "":
			warn(r.Name, "rule "+r.Name+" gives no reason",
				fix+", under findingRules."+string(r.FindingRule))
		default:
			warn(r.Name, "rule "+r.Name+" gives no reason", fix)
		}
	}
}

// cyclesOf lists a graph's elementary cycles, capped.
func cyclesOf(level string, g arch.Graph) CycleSet {
	cycles, truncated := g.ElementaryCycles(maxCycles)
	return CycleSet{
		Level: level, Cycles: append(make([][]string, 0, len(cycles)), cycles...),
		Truncated: truncated,
	}
}

// matchingPackages counts the in-scope packages any of the patterns match.
func (s *subject) matchingPackages(patterns []string) int {
	return countMatching(s.packagePaths(), patterns)
}

// countMatching counts the packages any of the patterns match.
func countMatching(packages, patterns []string) int {
	n := 0
	for _, pkg := range packages {
		if arch.MatchesAny(patterns, pkg) {
			n++
		}
	}
	return n
}

// outsideTargets returns the packages outside the analysis that the import
// rules see imported, sorted.
func outsideTargets(p *arch.Program, g *arch.Graphs) []string {
	var out []string
	for _, imp := range p.RuleImports() {
		if _, in := g.SubsystemOf[imp.To]; !in {
			out = append(out, imp.To)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// packagePaths returns the in-scope packages, sorted.
func (s *subject) packagePaths() []string {
	out := make([]string, len(s.p.Packages))
	for i, pkg := range s.p.Packages {
		out[i] = pkg.Path
	}
	return out
}
