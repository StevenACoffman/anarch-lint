// Package text renders reports and gate verdicts for people, with
// text/tabwriter. Machine-readable output is arch.WriteJSON's job; this
// package only lays out what a report already contains, in a fixed order, so
// its output is as deterministic as the report.
package text

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// hashPrefix is how much of a hash the text form shows.
const hashPrefix = 12

// WriteReport writes a report section by section: inputs, MMI, ordering,
// coupling, package metrics, the types to read first, findings, judgement
// slots and assumptions.
func WriteReport(w io.Writer, r *arch.Report) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	writeInputs(tw, &r.Inputs)
	writeMMI(tw, &r.MMI)
	writeConformance(tw, &r.Conformance)
	writeOrdering(tw, &r.Ordering)
	writeCoupling(tw, &r.Coupling)
	writePackages(tw, r.Packages)
	writeRank(tw, r.ComponentRank)
	writeFindings(tw, r.Findings)
	writeJudgements(tw, r.Judgements)
	_, _ = fmt.Fprintf(
		tw,
		"\nASSUMPTIONS: %d, listed in the JSON report (--format=json)\n",
		len(r.Assumptions),
	)
	if err := tw.Flush(); err != nil {
		return &arch.Error{Op: "text.WriteReport", Err: err}
	}
	return nil
}

func writeInputs(w io.Writer, in *arch.Inputs) {
	tags := strings.Join(in.Tags, ",")
	if tags == "" {
		tags = "none"
	}
	_, _ = fmt.Fprintf(w, "anarch-lint report (%s)\n", arch.ReportSchema)
	_, _ = fmt.Fprintf(w, "patterns %s · tags %s · model %s · %s\n",
		strings.Join(in.Patterns, " "), tags, short(in.ModelHash), in.GoVersion)
}

func writeMMI(w io.Writer, m *arch.MMI) {
	pending, unmeasured := 0, 0
	for i := range m.Criteria {
		switch {
		case m.Criteria[i].Kind == arch.Unmeasured:
			unmeasured++
		case m.Criteria[i].Score == nil:
			pending++
		}
	}
	if m.Exact {
		_, _ = fmt.Fprintf(w, "\nMMI %.2f\n", m.Low)
	} else {
		_, _ = fmt.Fprintf(
			w,
			"\nMMI %.2f – %.2f  (%d judgements pending, %d criteria unmeasured)\n",
			m.Low,
			m.High,
			pending,
			unmeasured,
		)
	}
	for i := range m.Criteria {
		c := &m.Criteria[i]
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", c.ID, c.Name, c.Kind, value(c), score(c))
	}
}

func writeOrdering(w io.Writer, o *arch.Ordering) {
	_, _ = fmt.Fprintln(w, "\nORDERING (Lilienthal 2008, ch. 7)")
	rows := []struct {
		criterion, name string
		value           int
	}{
		{"Zyklenausmaß", "ClassesInCycles", o.Class.InCycles},
		{"Zyklenausmaß", "PckgInCycles", o.Package.InCycles},
		{"Zyklenausmaß", "SubsysInCycles", o.Subsystem.InCycles},
		{"Zyklenausmaß", "RefClassesInCycles", o.Class.Refs},
		{"Zyklenausmaß", "RefPckgInCycles", o.Package.Weight},
		{"Zyklenausmaß", "RefSubsysInCycles", o.Subsystem.Weight},
		{
			"Zyklenreichweite",
			"PackageCycleGroupsFromClassCycles",
			o.PackageCycleGroupsFromClassCycles,
		},
		{
			"Zyklenreichweite",
			"SubsysCycleGroupsFromClassCycles",
			o.SubsysCycleGroupsFromClassCycles,
		},
		{
			"Zyklenreichweite",
			"RefsInClassCyclesBetweenPackages",
			o.RefsInClassCyclesBetweenPackages,
		},
		{
			"Zyklenreichweite",
			"RefsInClassCyclesBetweenSubsystems",
			o.RefsInClassCyclesBetweenSubsystems,
		},
		{"Verflochtenheit", "DirectRefsInClassCycles", o.Class.Direct},
		{"Verflochtenheit", "DirectRefsInPackageCycles", o.Package.Direct},
		{"Verflochtenheit", "DirectRefsInSubsystemCycles", o.Subsystem.Direct},
		{"Zyklenumfang", "max ClassesInCycle", o.Class.MaxGroup},
		{"Zyklenumfang", "max PckgInCycle", o.Package.MaxGroup},
		{"Zyklenumfang", "max SubsysInCycle", o.Subsystem.MaxGroup},
	}
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%d\n", r.criterion, r.name, r.value)
	}
}

func writeCoupling(w io.Writer, c *arch.CouplingLevels) {
	_, _ = fmt.Fprintln(w, "\nCOUPLING (von Zitzewitz)\ttypes\tpackages\tsubsystems")
	levels := []*arch.Coupling{&c.Types, &c.Packages, &c.Subsystems}
	row := func(name string, cell func(*arch.Coupling) string) {
		cells := make([]string, len(levels))
		for i, l := range levels {
			cells[i] = cell(l)
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s\n", name, strings.Join(cells, "\t"))
	}
	row("elements", func(l *arch.Coupling) string { return strconv.Itoa(l.Nodes) })
	row("CCD", func(l *arch.Coupling) string { return strconv.Itoa(l.CCD) })
	row("ACD", func(l *arch.Coupling) string { return fmt.Sprintf("%.2f", l.ACD) })
	row("propagation cost", func(l *arch.Coupling) string { return percentText(l.PropagationCost) })
	row(
		"relative cyclicity",
		func(l *arch.Coupling) string { return percentText(l.RelativeCyclicity) },
	)
	row(
		"largest cycle group",
		func(l *arch.Coupling) string { return strconv.Itoa(l.MaxCycleGroup) },
	)
	row("SDI", func(l *arch.Coupling) string { return fmt.Sprintf("%d (%s)", l.SDI, l.CutMethod) })
}

// writeConformance writes the model's coverage and the program's compliance,
// with the rules that fail.
func writeConformance(w io.Writer, c *arch.Conformance) {
	_, _ = fmt.Fprintf(
		w,
		"\nCONFORMANCE coverage %.2f%% · compliance %.2f%% (%d of %d rules pass)\n",
		c.Coverage,
		c.Compliance,
		c.Passing,
		c.Rules,
	)
	for _, rule := range c.Failing {
		_, _ = fmt.Fprintf(w, "  failing\t%s\n", rule)
	}
}

// writePackages writes Martin's metrics, the packages furthest from the main
// sequence first, at most ten of them.
func writePackages(w io.Writer, pkgs []arch.PackageMetrics) {
	const listed = 10
	sorted := slices.Clone(pkgs)
	slices.SortStableFunc(sorted, func(a, b arch.PackageMetrics) int {
		return cmp.Or(cmp.Compare(b.Distance, a.Distance), cmp.Compare(a.Package, b.Package))
	})
	_, _ = fmt.Fprintln(
		w,
		"\nPACKAGES (Martin; furthest from the main sequence first)\tCa\tCe\tI\tA\tD",
	)
	for i := range sorted[:min(len(sorted), listed)] {
		m := &sorted[i]
		_, _ = fmt.Fprintf(w, "  %s\t%d\t%d\t%.2f\t%.2f\t%.2f\n",
			m.Package, m.Ca, m.Ce, m.Instability, m.Abstractness, m.Distance)
	}
}

// writeRank writes the most depended-upon types.
func writeRank(w io.Writer, ranked []arch.RankedType) {
	_, _ = fmt.Fprintln(w, "\nREAD FIRST (Component Rank)")
	for i := range ranked {
		_, _ = fmt.Fprintf(w, "  %s\t%.4f\n", ranked[i].Type, ranked[i].Rank)
	}
}

func writeFindings(w io.Writer, fs []arch.Finding) {
	errs := 0
	for i := range fs {
		if fs[i].Severity == arch.SeverityError {
			errs++
		}
	}
	_, _ = fmt.Fprintf(w, "\nFINDINGS (%d errors, %d warnings)\n", errs, len(fs)-errs)
	for i := range fs {
		WriteFinding(w, &fs[i])
	}
}

// WriteFinding writes one finding: its rule, subject and location, then the
// message, the evidence and the fix on lines of their own.
func WriteFinding(w io.Writer, f *arch.Finding) {
	where := ""
	if f.Location != nil {
		where = fmt.Sprintf("%s:%d", f.Location.File, f.Location.Start)
	}
	_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", f.Severity, f.Rule, f.Subject, where)
	_, _ = fmt.Fprintf(w, "  \t%s\n", f.Message)
	for _, e := range f.Evidence {
		_, _ = fmt.Fprintf(w, "  \tevidence: %s\n", e)
	}
	_, _ = fmt.Fprintf(w, "  \tfix: %s\n", f.Fix)
}

func writeJudgements(w io.Writer, slots []arch.JudgementSlot) {
	_, _ = fmt.Fprintln(w, "\nJUDGEMENTS (for a human expert or an adversarial LLM; never gate)")
	for i := range slots {
		s := &slots[i]
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", s.Criterion, s.Status, s.Question)
		for _, e := range s.Evidence {
			_, _ = fmt.Fprintf(w, "  \t\tevidence: %s\n", e)
		}
		_, _ = fmt.Fprintf(w, "  \t\tevidence hash: %s\n", s.EvidenceHash)
	}
}

func value(c *arch.Criterion) string {
	if c.Value == nil {
		return "-"
	}
	if c.Unit == "%" {
		return fmt.Sprintf("%.2f%%", *c.Value)
	}
	return strconv.FormatFloat(*c.Value, 'f', -1, 64)
}

func score(c *arch.Criterion) string {
	if c.Score == nil {
		return "unscored"
	}
	return "score " + strconv.Itoa(*c.Score)
}

func percentText(ratio float64) string { return fmt.Sprintf("%.1f%%", 100*ratio) }

func short(hash string) string {
	if len(hash) > hashPrefix {
		return hash[:hashPrefix]
	}
	return hash
}

// WriteGate writes a gate result: the verdict, then what regressed, what is
// new, each new finding with its fix, and what was fixed.
func WriteGate(w io.Writer, g *arch.GateResult) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	writeVerdict(tw, g)
	writeFindingList(tw, "NEW ERRORS", g.New)
	writeFindingList(tw, "NEW WARNINGS", g.NewWarnings)
	for i := range g.Fixed {
		_, _ = fmt.Fprintf(tw, "  fixed\t%s\t%s\n", g.Fixed[i].Rule, g.Fixed[i].Subject)
	}
	writeWaivers(tw, g)
	if err := tw.Flush(); err != nil {
		return &arch.Error{Op: "text.WriteGate", Err: err}
	}
	return nil
}

// writeVerdict writes whether the gate passed, why, its scope, and every
// metric regression.
func writeVerdict(w io.Writer, g *arch.GateResult) {
	verdict := "PASSED"
	if !g.Passed {
		verdict = "FAILED"
	}
	_, _ = fmt.Fprintf(w, "GATE %s", verdict)
	if g.Reason != "" {
		_, _ = fmt.Fprintf(w, ": %s", g.Reason)
	}
	_, _ = fmt.Fprintln(w)
	if g.Scoped {
		_, _ = fmt.Fprintf(
			w,
			"scoped to changed lines; %d new findings elsewhere set aside\n",
			g.OutOfScope,
		)
	}
	for i := range g.Regressions {
		r := &g.Regressions[i]
		_, _ = fmt.Fprintf(w, "  regression\t%s\t%g -> %g\n", r.Metric, r.Baseline, r.Current)
	}
	for i := range g.Shortfalls {
		sf := &g.Shortfalls[i]
		_, _ = fmt.Fprintf(w, "  below floor\t%s\t%g < %g\n", sf.Gate, sf.Actual, sf.Minimum)
	}
}

// writeWaivers writes what waivers did: the findings they accepted, the
// waivers that expired, and the ones that name no current finding.
func writeWaivers(w io.Writer, g *arch.GateResult) {
	for i := range g.Waived {
		_, _ = fmt.Fprintf(
			w,
			"  waived\t%s\t%s\t%s\n",
			g.Waived[i].Rule,
			g.Waived[i].Subject,
			g.Waived[i].Fingerprint,
		)
	}
	for i := range g.ExpiredWaivers {
		wv := &g.ExpiredWaivers[i]
		_, _ = fmt.Fprintf(
			w,
			"  expired waiver\t%s\texpired %s, approved by %s\n",
			wv.Fingerprint,
			wv.Expires,
			wv.ApprovedBy,
		)
	}
	for i := range g.UnusedWaivers {
		_, _ = fmt.Fprintf(
			w,
			"  unused waiver\t%s\tnames no current finding; delete it\n",
			g.UnusedWaivers[i].Fingerprint,
		)
	}
}

// writeFindingList writes a titled list of findings, or nothing when empty.
func writeFindingList(w io.Writer, title string, fs []arch.Finding) {
	if len(fs) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "\n%s (%d)\n", title, len(fs))
	for i := range fs {
		WriteFinding(w, &fs[i])
	}
}
