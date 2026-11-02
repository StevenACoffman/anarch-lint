// Package sarif writes the gate's result as SARIF 2.1.0, the format code
// scanning reads (TODO.md, "SARIF output"), through
// github.com/owenrumney/go-sarif. Each finding is a result: the gate's new
// findings and new warnings are "new", waived ones are "new" and suppressed,
// and a regression, a shortfall or an incomparable baseline, which has no
// place in the code, is a tool execution notification.
package sarif

import (
	"fmt"
	"io"
	"slices"

	"github.com/google/uuid"
	"github.com/owenrumney/go-sarif/v2/sarif"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// informationURI is where a reader learns what the rules mean.
const informationURI = "https://github.com/StevenACoffman/anarch-lint"

// fingerprintKey names anarch-lint's fingerprint among a result's partial
// fingerprints; the version changes if the fingerprint's recipe does.
const fingerprintKey = "anarchLint/v1"

// Options are what a SARIF log records besides the result.
type Options struct {
	// Version is the tool's version, such as "v1.2.3" or "dev".
	Version string
	// ModelPath is the model file, relative to the analysis root: where a
	// finding about the model itself points.
	ModelPath string
}

// WriteGate writes the gate's result to w as a SARIF log of one run.
func WriteGate(w io.Writer, r *arch.GateResult, opts Options) error {
	const op = "sarif.WriteGate"
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return &arch.Error{Op: op, Err: err}
	}
	run := sarif.NewRunWithInformationURI("anarch-lint", informationURI)
	run.Tool.Driver.WithVersion(opts.Version)
	findings := slices.Concat(r.New, r.NewWarnings, r.Waived)
	addRules(run, findings)
	waived := map[string]bool{}
	for i := range r.Waived {
		waived[r.Waived[i].Fingerprint] = true
	}
	for i := range findings {
		run.AddResult(result(&findings[i], waived[findings[i].Fingerprint], opts.ModelPath))
	}
	run.AddInvocations(invocation(r))
	report.AddRun(run)
	if err := report.PrettyWrite(w); err != nil {
		return &arch.Error{Op: op, Err: err}
	}
	return nil
}

// addRules declares each rule the findings break, sorted by name, with the
// catalog's summary as its description.
func addRules(run *sarif.Run, findings []arch.Finding) {
	names := make([]string, 0, len(findings))
	for i := range findings {
		names = append(names, findings[i].Rule)
	}
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		rule := run.AddRule(name).WithName(name).WithHelpURI(informationURI + "#readme").
			WithShortDescription(sarif.NewMultiformatMessageString(name))
		if info, ok := arch.RuleNamed(name); ok {
			rule.WithFullDescription(sarif.NewMultiformatMessageString(info.Summary))
		}
	}
}

// result is one finding as a SARIF result.
func result(f *arch.Finding, waived bool, modelPath string) *sarif.Result {
	text := f.Message
	if f.Fix != "" {
		text += "\nfix: " + f.Fix
	}
	res := sarif.NewRuleResult(f.Rule).
		WithLevel(level(f.Severity)).
		WithMessage(sarif.NewTextMessage(text)).
		WithBaselineState("new").
		WithPartialFingerPrints(map[string]any{fingerprintKey: f.Fingerprint})
	if props := properties(f); props != nil {
		res.AttachPropertyBag(props)
	}
	loc := sarif.NewPhysicalLocation()
	switch {
	case f.Location != nil:
		loc.WithArtifactLocation(sarif.NewSimpleArtifactLocation(f.Location.File)).
			WithRegion(sarif.NewSimpleRegion(f.Location.Start, max(f.Location.End, f.Location.Start)))
	case modelPath != "":
		loc.WithArtifactLocation(sarif.NewSimpleArtifactLocation(modelPath))
	default:
		loc = nil
	}
	if loc == nil {
		return res
	}
	res.AddLocation(sarif.NewLocationWithPhysicalLocation(loc))
	if waived {
		// go-sarif writes every field of a suppression, so each is set.
		res.AddSuppression(sarif.NewSuppression("external").WithStatus("accepted").
			WithGuid(guidOf(f.Fingerprint)).
			WithLocation(sarif.NewLocationWithPhysicalLocation(loc)).
			WithJustifcation("accepted by a waiver until it expires"))
	}
	return res
}

// properties are what the model says of the finding's rule: the rule it
// breaks and the quality attribute it protects; nil when it says neither.
func properties(f *arch.Finding) *sarif.PropertyBag {
	if f.ModelRule == "" && f.QualityAttribute == "" {
		return nil
	}
	props := sarif.NewPropertyBag()
	if f.ModelRule != "" {
		props.AddString("modelRule", f.ModelRule)
	}
	if f.QualityAttribute != "" {
		props.AddString("qualityAttribute", string(f.QualityAttribute))
	}
	return props
}

// guidOf derives a stable GUID from a fingerprint: a name-based UUID
// (version 5) in a namespace of anarch-lint's own.
func guidOf(fingerprint string) string {
	ns := uuid.NewSHA1(uuid.NameSpaceURL, []byte(informationURI))
	return uuid.NewSHA1(ns, []byte(fingerprint)).String()
}

// level is a severity as a SARIF level.
func level(s arch.Severity) string {
	if s == arch.SeverityError {
		return "error"
	}
	return "warning"
}

// invocation says whether the gate passed, with a notification for each
// failure that has no place in the code, and a note of what it did not read.
func invocation(r *arch.GateResult) *sarif.Invocation {
	inv := sarif.NewInvocation().WithExecutionSuccess(true)
	add := func(level, text string) {
		inv.AddTToolExecutionNotification(sarif.NewNotification().
			WithLevel(level).WithMessage(sarif.NewTextMessage(text)))
	}
	notify := func(text string) { add("error", text) }
	if unread := arch.DescribeUnread(r.Unread); unread != "" {
		add("note", "not read: "+unread)
	}
	if r.BaselineBuild != nil {
		add("note", arch.DescribeOtherBuild(r.BaselineBuild))
	}
	if !r.Comparable {
		notify("the baseline is not comparable: " + r.Reason)
	}
	for _, reg := range r.Regressions {
		notify(fmt.Sprintf("%s regressed from %g to %g", reg.Metric, reg.Baseline, reg.Current))
	}
	for _, s := range r.Shortfalls {
		notify(fmt.Sprintf("%s is %g, below its floor of %g", s.Gate, s.Actual, s.Minimum))
	}
	return inv
}
