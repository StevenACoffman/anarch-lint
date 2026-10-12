package arch

import (
	"cmp"
	"fmt"
	"slices"
)

// ReportSchema names the report format; it changes when a field changes
// meaning, so a consumer can refuse a report it does not understand.
const ReportSchema = "anarch-lint.report.v1"

// Report is the complete, deterministic result of one analysis.
type Report struct {
	Schema string `json:"schema"`
	Inputs Inputs `json:"inputs"`
	Sizes  Sizes  `json:"sizes"`
	// Ordering holds the dissertation's thirteen measures (mmi.md §4).
	Ordering Ordering `json:"ordering"`
	// Coupling holds von Zitzewitz's metrics at each level (mmi.md §7.5).
	Coupling CouplingLevels `json:"coupling"`
	// Cohesion lists the LCOM4 of struct types with two or more methods.
	Cohesion []Cohesion `json:"cohesion"`
	// Packages holds Martin's metrics for every package.
	Packages []PackageMetrics `json:"packages"`
	// ComponentRank lists the most depended-upon types: read these first.
	ComponentRank []RankedType `json:"componentRank"`
	// Conformance holds the model's coverage and the program's compliance.
	Conformance Conformance `json:"conformance"`
	MMI         MMI         `json:"mmi"`
	// Judgements are the slots for the five reviewer criteria.
	Judgements []JudgementSlot `json:"judgements"`
	Findings   []Finding       `json:"findings"`
	// Assumptions are the choices behind the numbers that a reader should
	// know: borrowed thresholds, definitions the sources leave open.
	Assumptions []Assumption `json:"assumptions"`
}

// CouplingLevels holds the coupling metrics of the three graphs.
type CouplingLevels struct {
	Types      Coupling `json:"types"`
	Packages   Coupling `json:"packages"`
	Subsystems Coupling `json:"subsystems"`
}

// Inputs records everything a report depends on besides the source tree, so
// that two reports can be compared only when their inputs agree. It holds no
// clock reading and no absolute path.
type Inputs struct {
	Tool       string     `json:"tool"`
	GoVersion  string     `json:"goVersion"`
	Tags       []string   `json:"tags"`
	Patterns   []string   `json:"patterns"`
	ModelHash  string     `json:"modelHash"`
	Thresholds Thresholds `json:"thresholds"`
}

// Assumption is a choice a measure rests on.
type Assumption struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Source string `json:"source"`
}

// Analyze measures in's program under its target architecture, filling the
// judgement slots from its verdicts. It is a pure function: the same input
// always gives the same report, and the input is not modified. The inputs are
// copied into the report with the model hash and the thresholds in force.
//
// Requires: in.Program is sorted, as [Program] documents.
func Analyze(input *Input) Report {
	p, m, verdicts, in := &input.Program, &input.Model, input.Verdicts, &input.Inputs
	g := BuildGraphs(p, m)
	s := &subject{p: p, m: m, g: &g}
	limits := m.Limits()
	inputs := *in
	inputs.ModelHash, inputs.Thresholds = m.Hash(), limits
	inputs.Tags = append(make([]string, 0, len(in.Tags)), in.Tags...)
	inputs.Patterns = append(make([]string, 0, len(in.Patterns)), in.Patterns...)

	sizes := sizesOf(p, &g, limits)
	ordering := g.Ordering()
	cohesion := CohesionOf(p)
	results := ruleResults{
		layers:         layerRule(s),
		subsystemOrder: subsystemOrderRule(s),
		iface:          interfaceRule(s),
		roles:          roleRule(s),
	}
	forbidden := forbiddenRule(s)
	findings := collectFindings(s, limits, cohesion, &results, forbidden)

	slots := judgementSlots(s, &sizes, cohesion, findings, verdicts)
	criteria := criteriaOf(s, &sizes, &ordering, &results)
	for i := range criteria {
		for j := range slots {
			if slots[j].Criterion == criteria[i].ID {
				slots[j].apply(&criteria[i])
			}
		}
	}
	return Report{
		Schema:   ReportSchema,
		Inputs:   inputs,
		Sizes:    sizes,
		Ordering: ordering,
		Coupling: CouplingLevels{
			Types: CouplingOf(
				g.Types,
			),
			Packages:   CouplingOf(g.Packages),
			Subsystems: CouplingOf(g.Subsystems),
		},
		Cohesion:      cohesion,
		Packages:      packageMetricsOf(p, &g),
		ComponentRank: componentRank(g.Types),
		Conformance:   conformanceOf(s, findings),
		MMI:           newMMI(criteria),
		Judgements:    slots,
		Findings:      findings,
		Assumptions:   assumptions(limits),
	}
}

// collectFindings runs every rule and returns the findings, sorted.
func collectFindings(
	s *subject, limits Thresholds, cohesion []Cohesion, r *ruleResults, forbidden ruleResult,
) []Finding {
	lcom := map[string]int{}
	for _, c := range cohesion {
		lcom[c.Type] = c.LCOM4
	}
	fs := make([]Finding, 0)
	fs = append(fs, assignmentRules(s)...)
	fs = append(fs, unknownRoleRule(s)...)
	fs = append(fs, isolationRule(s)...)
	fs = append(fs, transitiveForbiddenRule(s)...)
	fs = append(fs, hiddenDependencyRule(s)...)
	for _, rr := range []ruleResult{r.layers, r.subsystemOrder, r.iface, r.roles, forbidden} {
		fs = append(fs, rr.findings...)
	}
	fs = append(fs, subsystemCycleRule(s)...)
	fs = append(fs, typeCycleRule(s, limits.MaxTypeCycleGroup)...)
	fs = append(fs, sizeRules(s, limits, lcom)...)
	fs = append(fs, lengthAndDepthRules(s, limits)...)
	SortFindings(fs)
	return fs
}

// judgementSlots gathers the deterministic evidence for each reviewer
// criterion (mmi.md §7.6) and fills the slots from the verdicts.
func judgementSlots(
	s *subject, z *Sizes, cohesion []Cohesion, findings []Finding, verdicts []Verdict,
) []JudgementSlot {
	return []JudgementSlot{
		NewSlot("1.1.5", responsibilityEvidence(s, z, cohesion), verdicts),
		NewSlot("1.1.6", mappingEvidence(s), verdicts),
		NewSlot("1.2.2", interfaceEvidence(s, findings), verdicts),
		NewSlot("3.3", roleEvidence(s), verdicts),
		NewSlot("3.4", separationEvidence(s, findings), verdicts),
	}
}

// responsibilityEvidence is the evidence for 1.1.5: cohesion through coupling,
// LCOM4 above 1, and the balance between the largest and smallest package.
func responsibilityEvidence(s *subject, z *Sizes, cohesion []Cohesion) []string {
	var ev []string
	for _, pkg := range s.p.Packages {
		inside, outside := s.couplingOf(pkg.Path)
		if outside > inside {
			ev = append(
				ev,
				fmt.Sprintf(
					"coupling: %s imports %d packages of other subsystems and %d of its own",
					pkg.Path,
					outside,
					inside,
				),
			)
		}
	}
	for _, c := range cohesion {
		if c.LCOM4 > 1 {
			ev = append(
				ev,
				fmt.Sprintf(
					"lcom4: %s has %d independent groups of methods and fields",
					c.Type,
					c.LCOM4,
				),
			)
		}
	}
	if len(s.p.Packages) > 0 {
		pkgs := slices.Clone(s.p.Packages)
		slices.SortStableFunc(pkgs, func(a, b Package) int { return cmp.Compare(b.Lines, a.Lines) })
		ev = append(
			ev,
			fmt.Sprintf(
				"balance: largest package %s has %d of %d lines; smallest %s has %d",
				pkgs[0].Path,
				pkgs[0].Lines,
				z.Lines,
				pkgs[len(pkgs)-1].Path,
				pkgs[len(pkgs)-1].Lines,
			),
		)
	}
	return ev
}

// mappingEvidence is the evidence for 1.1.6: what the model's patterns leave
// unassigned, assign ambiguously, or no longer match.
func mappingEvidence(s *subject) []string {
	sa, la := s.g.SubsystemAssignment, s.g.LayerAssignment
	ev := []string{
		fmt.Sprintf("subsystems: %d declared; %d packages unassigned, %d ambiguous",
			len(s.m.Subsystems), len(sa.Unassigned), len(sa.Ambiguous)),
		fmt.Sprintf("layers: %d declared; %d packages unassigned, %d ambiguous",
			len(s.m.Layers), len(la.Unassigned), len(la.Ambiguous)),
	}
	for _, pat := range append(slices.Clone(sa.Stale), la.Stale...) {
		ev = append(ev, "stale pattern: "+pat)
	}
	return ev
}

// interfaceEvidence is the evidence for 1.2.2: subsystems without an
// interface, and imports that bypass one.
func interfaceEvidence(s *subject, findings []Finding) []string {
	ev := []string{fmt.Sprintf("subsystems: %d declared", len(s.m.Subsystems))}
	for i := range findings {
		if f := &findings[i]; f.Rule == RuleSubsystemWithoutIface || f.Rule == RuleInterfaceBypass {
			ev = append(ev, f.Rule+": "+f.Message)
		}
	}
	return ev
}

// separationEvidence is the evidence for 3.4: the forbidden imports found,
// which decide one necessary condition of the separation.
func separationEvidence(s *subject, findings []Finding) []string {
	ev := []string{fmt.Sprintf("forbidden-import rules: %d declared", len(s.m.ForbiddenImports))}
	for i := range findings {
		if f := &findings[i]; f.Rule == RuleForbiddenImport {
			ev = append(ev, "forbidden import: "+f.Subject+" -> "+f.Object)
		}
	}
	return ev
}

// couplingOf counts the in-scope packages pkg imports within its own subsystem
// and outside it: Lilienthal's test of cohesion through coupling.
func (s *subject) couplingOf(pkg string) (inside, outside int) {
	sub, ok := s.g.SubsystemAssignment.Of[pkg]
	if !ok {
		return 0, 0
	}
	for _, imp := range s.inScopeImports() {
		if imp.From != pkg {
			continue
		}
		if s.g.SubsystemAssignment.Of[imp.To] == sub {
			inside++
		} else {
			outside++
		}
	}
	return inside, outside
}

// assumptions lists the choices behind the measures, with the thresholds in
// force.
func assumptions(t Thresholds) []Assumption {
	return []Assumption{
		{"go-acyclic-imports", "Go forbids import cycles, so 2.2.2 and 2.2.4 always score 10; " +
			"read hierarchy problems from subsystem cycles and 2.1.x instead.", "mmi.md §4"},
		{
			"type-granularity",
			"A Go package-level named type stands in for the dissertation's class; " +
				"the MMI's class thresholds were calibrated on Java, C# and PHP.",
			"mmi.md §3",
		},
		{"loc", "A line of code is a line on which a token other than a comment starts; " +
			"test files and generated files are left out of every measure.", "mmi.md §7.3"},
		{"statements", "Number of Statements counts every statement except blocks, labels, empty " +
			"statements and case clauses, including a for loop's init and post.", "mmi.md §7.3"},
		{"complexity-variant", fmt.Sprintf("Criterion 1.3.4 uses the %s cyclomatic complexity, "+
			"above %d.", t.ComplexityVariant, t.HighComplexity), "mmi.md §7.3; von Zitzewitz"},
		{
			"thresholds",
			fmt.Sprintf("large type > %d lines, large function > %d lines, large package "+
				"> %d types, largest tolerable type cycle group %d, large file > %d lines, "+
				"deepest nesting %d: borrowed guidance, not the chapter's.", t.LargeTypeLines,
				t.LargeFuncLines, t.LargePackageTypes, t.MaxTypeCycleGroup, t.LargeFileLines,
				t.MaxNesting),
			"mmi.md §7.3",
		},
		{
			"denominators",
			"1.2.1, 2.1.1, 2.1.2 and 3.2 divide violating imports or references by " +
				"all candidates between different groups; the chapter gives no denominators.",
			"mmi.md §7.2",
		},
		{"largest-cycle", "Criteria 2.2.3 and 2.2.4 use the largest cycle group.", "mmi.md §4"},
		{"mmi-aggregation", "MMI = Σ weight(group) × mean(scores); unscored criteria count 0 " +
			"toward the low bound and 10 toward the high bound.", "mmi.md §2"},
		{"lcom4", "LCOM4 links methods to the fields and methods they select through the " +
			"receiver; a field no method uses is left out.", "mmi.md §7.5"},
		{"decoupling-level", "Decoupling Level (Mo et al., ICSE 2016) is not computed: US patent " +
			"11,422,800 covers its algorithm.", "mmi-ref.md A.1"},
		{
			"martin",
			"Martin's metrics count in-scope imports only; abstractness is the share of " +
				"named types that are interfaces, and instability is 0 for an isolated package.",
			"mmi-ref.md A.1",
		},
		{
			"component-rank",
			"Component Rank is PageRank over type dependencies with damping 0.8, " +
				"run for a fixed 50 iterations; types that depend on nothing spread their rank evenly.",
			"mmi-ref.md A.1; von Zitzewitz",
		},
		{"sdi-cut", "SDI cuts are exact for cycle groups of up to 12 elements and use the " +
			"Eades–Lin–Smyth ordering above that; Coupling.CutMethod says which.", "mmi.md §7.5"},
	}
}
