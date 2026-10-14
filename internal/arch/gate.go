package arch

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
)

// BaselineSchema names the baseline format.
const BaselineSchema = "anarch-lint.baseline.v1"

// Baseline is the accepted state of an architecture: the findings it already
// has and the system metrics it already scores. The gate fails on anything
// new or worse, so a legacy system can adopt it without first fixing
// everything ("stop the bleeding", von Zitzewitz; mmi.md §8).
type Baseline struct {
	Schema string `json:"schema"`
	// ModelHash ties the baseline to the model it was measured under: a
	// report under another model is not comparable with it.
	ModelHash string            `json:"modelHash"`
	Findings  []BaselineFinding `json:"findings"`
	Metrics   []Metric          `json:"metrics"`
}

// BaselineFinding identifies an accepted finding.
type BaselineFinding struct {
	Fingerprint string   `json:"fingerprint"`
	Rule        string   `json:"rule"`
	Severity    Severity `json:"severity"`
	Subject     string   `json:"subject"`
	Object      string   `json:"object,omitempty"`
}

// Metric is one system-level value the gate ratchets.
type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	// HigherIsBetter is true for the MMI's low bound and false for every
	// cycle and debt measure.
	HigherIsBetter bool `json:"higherIsBetter"`
}

// GateInput is what the gate compares with a baseline: every measure's
// findings and the system metrics it ratchets, measured under one model.
type GateInput struct {
	ModelHash string
	Findings  []Finding
	Metrics   []Metric
	// Conformance holds the coverage and compliance the model's floors
	// apply to.
	Conformance Conformance
}

// GateOptions narrow or relax a gate. The zero value, and a nil pointer,
// gate every finding with no waivers.
type GateOptions struct {
	// Changed, when set, limits the finding gate to findings whose location
	// intersects a changed line: the granular gate of a single change.
	Changed ChangedLines
	// Waivers accept findings until they expire.
	Waivers []Waiver
	// Today decides which waivers have expired. The shell supplies it, so
	// the domain never reads the clock.
	Today Date
}

// GateResult is the outcome of comparing a report with a baseline.
type GateResult struct {
	Passed bool `json:"passed"`
	// Comparable is false when the baseline was measured under another
	// model; such a gate fails, naming the reason, rather than comparing
	// numbers that mean different things.
	Comparable bool   `json:"comparable"`
	Reason     string `json:"reason,omitempty"`
	// New are error-severity findings the baseline does not have; they fail
	// the gate. NewWarnings are new warnings, reported as advice.
	New         []Finding `json:"new"`
	NewWarnings []Finding `json:"newWarnings"`
	// Waived are new findings an unexpired waiver accepts. ExpiredWaivers
	// no longer protect their findings, which count as new. UnusedWaivers
	// name no current finding and can be deleted.
	Waived         []Finding `json:"waived"`
	ExpiredWaivers []Waiver  `json:"expiredWaivers"`
	UnusedWaivers  []Waiver  `json:"unusedWaivers"`
	// Fixed are baseline findings the report no longer has.
	Fixed       []BaselineFinding `json:"fixed"`
	Regressions []Regression      `json:"regressions"`
	// Improved are metrics better than their baseline values. With Fixed,
	// they say the baseline can shrink, which never fails the gate.
	Improved []Regression `json:"improved"`
	// Shortfalls are the model's coverage and compliance floors the report
	// falls below; any one fails the gate, whatever the baseline says.
	Shortfalls []Shortfall `json:"shortfalls"`
	// Scoped is true when only findings on changed lines counted;
	// OutOfScope counts the new findings that were set aside.
	Scoped     bool `json:"scoped"`
	OutOfScope int  `json:"outOfScope"`
}

// Regression is a system metric that got worse than its baseline value.
type Regression struct {
	Metric   string  `json:"metric"`
	Baseline float64 `json:"baseline"`
	Current  float64 `json:"current"`
}

// GateInput returns the MMI report's part of the gate: its findings, its
// system metrics and its conformance.
func (r *Report) GateInput() GateInput {
	return GateInput{
		ModelHash:   r.Inputs.ModelHash,
		Findings:    append(make([]Finding, 0, len(r.Findings)), r.Findings...),
		Metrics:     gateMetrics(r),
		Conformance: r.Conformance,
	}
}

// Add adds another measure's findings and metrics to the gate, keeping the
// findings sorted and the metrics in name order after the MMI's.
func (in *GateInput) Add(findings []Finding, metrics []Metric) {
	in.Findings = append(in.Findings, findings...)
	SortFindings(in.Findings)
	extra := slices.Clone(metrics)
	slices.SortFunc(extra, func(a, b Metric) int { return cmp.Compare(a.Name, b.Name) })
	in.Metrics = append(in.Metrics, extra...)
}

// NewBaseline records the findings and metrics of the gate's input.
func NewBaseline(r *GateInput) Baseline {
	b := Baseline{
		Schema:    BaselineSchema,
		ModelHash: r.ModelHash,
		Findings:  make([]BaselineFinding, 0, len(r.Findings)),
		Metrics:   append(make([]Metric, 0, len(r.Metrics)), r.Metrics...),
	}
	for i := range r.Findings {
		f := &r.Findings[i]
		b.Findings = append(b.Findings, BaselineFinding{
			Fingerprint: f.Fingerprint,
			Rule:        f.Rule,
			Severity:    f.Severity,
			Subject:     f.Subject,
			Object:      f.Object,
		})
	}
	return b
}

// DecodeBaseline reads a baseline file; an unknown schema is EINVALID.
func DecodeBaseline(r io.Reader) (Baseline, error) {
	const op = "arch.DecodeBaseline"
	var b Baseline
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return Baseline{}, &Error{
			Op:  op,
			Err: &Error{Code: EINVALID, Message: "baseline: " + err.Error()},
		}
	}
	if b.Schema != BaselineSchema {
		return Baseline{}, &Error{Op: op, Err: &Error{
			Code:    EINVALID,
			Message: fmt.Sprintf("baseline: schema %q is not %q", b.Schema, BaselineSchema),
		}}
	}
	return b, nil
}

// Gate compares a report with its baseline. System metrics and conformance
// floors gate whatever opts says; only the finding gate is scoped and waived.
//
// Requires: opts.Today is set whenever opts.Waivers is not empty.
// Ensures: Passed is true exactly when the two are comparable, nothing new
// and unwaived has error severity, no metric regressed, and no coverage or
// compliance floor is missed.
func Gate(r *GateInput, b *Baseline, opts *GateOptions) GateResult {
	if opts == nil {
		opts = &GateOptions{}
	}
	g := newGateResult(r, b, opts)
	if !g.Comparable {
		g.Reason = "the baseline was recorded under a different model; run baseline again " +
			"after reviewing the model change"
		return g
	}
	g.compare(r, b, opts)
	g.Regressions, g.Improved = metricChanges(b.Metrics, r.Metrics)
	g.Shortfalls = r.Conformance.shortfalls()
	g.decide()
	return g
}

// compare sorts the report's findings against the baseline and the waivers,
// and records what the baseline had that the report no longer has.
func (g *GateResult) compare(r *GateInput, b *Baseline, opts *GateOptions) {
	known := map[string]bool{}
	for _, f := range b.Findings {
		known[f.Fingerprint] = true
	}
	waivers := map[string]Waiver{}
	for _, w := range opts.Waivers {
		waivers[w.Fingerprint] = w
	}
	current := map[string]bool{}
	for i := range r.Findings {
		f := &r.Findings[i]
		current[f.Fingerprint] = true
		if !known[f.Fingerprint] {
			g.classify(f, opts, waivers)
		}
	}
	for _, f := range b.Findings {
		if !current[f.Fingerprint] {
			g.Fixed = append(g.Fixed, f)
		}
	}
	for _, w := range opts.Waivers {
		if !current[w.Fingerprint] {
			g.UnusedWaivers = append(g.UnusedWaivers, w)
		}
	}
}

// newGateResult returns an empty result, every list empty rather than nil.
func newGateResult(r *GateInput, b *Baseline, opts *GateOptions) GateResult {
	return GateResult{
		Comparable:     r.ModelHash == b.ModelHash,
		Scoped:         opts.Changed != nil,
		New:            make([]Finding, 0),
		NewWarnings:    make([]Finding, 0),
		Waived:         make([]Finding, 0),
		ExpiredWaivers: make([]Waiver, 0),
		UnusedWaivers:  make([]Waiver, 0),
		Fixed:          make([]BaselineFinding, 0),
		Regressions:    make([]Regression, 0),
		Improved:       make([]Regression, 0),
		Shortfalls:     make([]Shortfall, 0),
	}
}

// classify sorts one finding the baseline does not have into the result. An
// expired waiver no longer protects its finding, and is reported.
func (g *GateResult) classify(f *Finding, opts *GateOptions, waivers map[string]Waiver) {
	if opts.Changed != nil && !f.touchedBy(opts.Changed) {
		g.OutOfScope++
		return
	}
	if w, ok := waivers[f.Fingerprint]; ok {
		if opts.Today <= w.Expires {
			g.Waived = append(g.Waived, *f)
			return
		}
		g.ExpiredWaivers = append(g.ExpiredWaivers, w)
	}
	if f.Severity == SeverityError {
		g.New = append(g.New, *f)
		return
	}
	g.NewWarnings = append(g.NewWarnings, *f)
}

// decide sets Passed and Reason from what the comparison found.
func (g *GateResult) decide() {
	g.Passed = len(g.New) == 0 && len(g.Regressions) == 0 && len(g.Shortfalls) == 0
	if !g.Passed {
		g.Reason = fmt.Sprintf("%d new error findings, %d metric regressions, %d floors not met",
			len(g.New), len(g.Regressions), len(g.Shortfalls))
	}
}

// gateMetrics returns the system metrics the gate ratchets, in a fixed order.
// Propagation cost and ACD are reported but not ratcheted: they fall as a
// system grows even when its coupling worsens (von Zitzewitz; mmi.md §7.5).
func gateMetrics(r *Report) []Metric {
	return []Metric{
		{Name: "mmi.low", Value: r.MMI.Low, HigherIsBetter: true},
		{Name: "types.inCycles", Value: float64(r.Ordering.Class.InCycles)},
		{Name: "types.maxCycleGroup", Value: float64(r.Ordering.Class.MaxGroup)},
		{Name: "types.relativeCyclicity", Value: r.Coupling.Types.RelativeCyclicity},
		{Name: "types.sdi", Value: float64(r.Coupling.Types.SDI)},
		{Name: "subsystems.inCycles", Value: float64(r.Ordering.Subsystem.InCycles)},
		{Name: "subsystems.sdi", Value: float64(r.Coupling.Subsystems.SDI)},
	}
}

// metricChanges returns every metric worse now than in the baseline, and
// every one better, each sorted by name. A metric the baseline does not
// record is new and changes neither way.
func metricChanges(baseline, current []Metric) (worse, better []Regression) {
	old := map[string]Metric{}
	for _, m := range baseline {
		old[m.Name] = m
	}
	worse, better = make([]Regression, 0), make([]Regression, 0)
	for _, m := range current {
		b, ok := old[m.Name]
		if !ok || math.Abs(m.Value-b.Value) <= 1e-9 {
			continue
		}
		change := Regression{Metric: m.Name, Baseline: b.Value, Current: m.Value}
		if (m.Value > b.Value) != m.HigherIsBetter {
			worse = append(worse, change)
		} else {
			better = append(better, change)
		}
	}
	byName := func(a, b Regression) int { return cmp.Compare(a.Metric, b.Metric) }
	slices.SortFunc(worse, byName)
	slices.SortFunc(better, byName)
	return worse, better
}
