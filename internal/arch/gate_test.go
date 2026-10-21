package arch_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

func TestGate(t *testing.T) {
	t.Parallel()
	m := fixtureModel()
	before := analyze(&m)
	baseline := arch.NewBaseline(gateInput(&before))

	cases := map[string]struct {
		model       arch.Model
		changed     arch.ChangedLines
		passed      bool
		comparable  bool
		newFindings int
		regressions int
	}{
		"the same state passes": {
			model: fixtureModel(), passed: true, comparable: true,
		},
		"a changed model makes the baseline not comparable": {
			model: func() arch.Model {
				m := fixtureModel()
				m.ForbiddenImports = []arch.ForbiddenImport{
					{
						From:    []string{"fx/shop/..."},
						Imports: []string{"fmt"},
						Reason:  "no printing",
					},
				}
				return m
			}(),
			comparable: false, // the model changed, so the baseline no longer applies
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			after := analyze(&tc.model)
			g := arch.Gate(gateInput(&after), &baseline, &arch.GateOptions{Changed: tc.changed})
			equals(t, tc.passed, g.Passed)
			equals(t, tc.comparable, g.Comparable)
			equals(t, tc.newFindings, len(g.New))
			equals(t, tc.regressions, len(g.Regressions))
		})
	}
}

func TestGateNewFindingAndRegression(t *testing.T) {
	t.Parallel()
	// Record a baseline with no subsystems, then measure a state whose model
	// is the same but whose code gained a subsystem cycle: here simulated by
	// dropping the accepted findings and lowering the recorded metrics.
	m := fixtureModel()
	r := analyze(&m)
	baseline := arch.NewBaseline(gateInput(&r))
	baseline.Findings = nil
	for i := range baseline.Metrics {
		if baseline.Metrics[i].Name == "subsystems.inCycles" {
			baseline.Metrics[i].Value = 0
		}
	}
	g := arch.Gate(gateInput(&r), &baseline, nil)
	assert(t, !g.Passed, "the gate passed a new cycle")
	assert(t, len(g.New) > 0, "no new error findings")
	equals(
		t,
		[]arch.Regression{{Metric: "subsystems.inCycles", Baseline: 0, Current: 2}},
		g.Regressions,
	)

	// Scoped to a change that touches no finding's lines, only the metric
	// regression still fails the gate.
	scoped := arch.Gate(gateInput(&r), &baseline, &arch.GateOptions{
		Changed: arch.ChangedLines{"elsewhere.go": {{Start: 1, End: 9}}},
	})
	equals(t, 0, len(scoped.New))
	assert(t, scoped.OutOfScope > 0, "nothing was set aside")
	assert(t, !scoped.Passed, "a metric regression passed a scoped gate")
}

func TestChangedLinesTouches(t *testing.T) {
	t.Parallel()
	c := arch.ChangedLines{"a.go": {{Start: 3, End: 5}, {Start: 10, End: 10}}}
	cases := map[string]struct {
		span    *arch.Span
		touches bool
	}{
		"no location":         {span: nil},
		"other file":          {span: &arch.Span{File: "b.go", Start: 1, End: 99}},
		"before the change":   {span: &arch.Span{File: "a.go", Start: 1, End: 2}},
		"overlapping the end": {span: &arch.Span{File: "a.go", Start: 5, End: 8}, touches: true},
		"between ranges":      {span: &arch.Span{File: "a.go", Start: 6, End: 9}},
		"spanning a range":    {span: &arch.Span{File: "a.go", Start: 1, End: 20}, touches: true},
		"on a one-line range": {span: &arch.Span{File: "a.go", Start: 10, End: 10}, touches: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			equals(t, tc.touches, c.Touches(tc.span))
		})
	}
}

func TestBaselineRoundTrip(t *testing.T) {
	t.Parallel()
	m := fixtureModel()
	r := analyze(&m)
	b := arch.NewBaseline(gateInput(&r))
	var buf bytes.Buffer
	ok(t, arch.WriteJSON(&buf, b))
	again, err := arch.DecodeBaseline(&buf)
	ok(t, err)
	equals(t, b, again)

	_, err = arch.DecodeBaseline(strings.NewReader(`{"schema": "other"}`))
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}

func TestConformanceAndFloors(t *testing.T) {
	t.Parallel()
	m := fixtureModel()
	m.ForbiddenImports = []arch.ForbiddenImport{
		{From: []string{"fx/shop/..."}, Imports: []string{"fmt"}, Reason: "violated"},
		{From: []string{"fx/billing/..."}, Imports: []string{"fmt"}, Reason: "kept"},
	}
	r := analyze(&m)
	c := r.Conformance
	equals(t, 100.0, c.Coverage) // both subsystems claim every package
	// interfaces fails (tax bypasses shop's), forbiddenImports[0] fails.
	equals(t, 3, c.Rules)
	equals(t, 1, c.Passing)
	equals(t, []string{"forbiddenImports[0]", "interfaces"}, c.Failing)
	equals(t, 33.333333, c.Compliance)

	baseline := arch.NewBaseline(gateInput(&r))
	equals(t, true, arch.Gate(gateInput(&r), &baseline, nil).Passed)

	m.Gates = arch.Gates{MinCompliance: 50}
	floored := analyze(&m)
	floorBaseline := arch.NewBaseline(gateInput(&floored))
	g := arch.Gate(gateInput(&floored), &floorBaseline, nil)
	assert(t, !g.Passed, "a compliance below the floor passed the gate")
	equals(
		t,
		[]arch.Shortfall{{Gate: "minCompliance", Minimum: 50, Actual: 33.333333}},
		g.Shortfalls,
	)
}

func TestShrink(t *testing.T) {
	t.Parallel()
	b := arch.Baseline{
		Schema: arch.BaselineSchema, ModelHash: "m",
		Findings: []arch.BaselineFinding{{Fingerprint: "kept"}, {Fingerprint: "fixed"}},
		Metrics: []arch.Metric{
			{Name: "cycles", Value: 4},
			{Name: "mmi.low", Value: 5, HigherIsBetter: true},
			{Name: "sdi", Value: 3},
		},
	}
	in := arch.GateInput{
		ModelHash: "m",
		Findings:  []arch.Finding{{Fingerprint: "kept"}, {Fingerprint: "new"}},
		Metrics: []arch.Metric{
			{Name: "cycles", Value: 2},
			{Name: "mmi.low", Value: 4, HigherIsBetter: true},
			{Name: "sdi", Value: 3},
			{Name: "brand.new", Value: 1},
		},
	}
	got := arch.Shrink(&b, &in)
	equals(
		t,
		[]arch.BaselineFinding{{Fingerprint: "kept"}},
		got.Findings,
	) // nothing new is accepted
	equals(t, []arch.Metric{
		{Name: "cycles", Value: 2},                        // improved
		{Name: "mmi.low", Value: 5, HigherIsBetter: true}, // worse: keeps the baseline's
		{Name: "sdi", Value: 3},
	}, got.Metrics)

	other := in
	other.ModelHash = "another"
	equals(t, b.Findings, arch.Shrink(&b, &other).Findings)
}

func TestGateReportsImprovements(t *testing.T) {
	t.Parallel()
	b := arch.Baseline{ModelHash: "m", Metrics: []arch.Metric{{Name: "cycles", Value: 4}}}
	g := arch.Gate(
		&arch.GateInput{ModelHash: "m", Metrics: []arch.Metric{{Name: "cycles", Value: 1}}},
		&b,
		nil,
	)
	equals(t, true, g.Passed)
	equals(t, []arch.Regression{{Metric: "cycles", Baseline: 4, Current: 1}}, g.Improved)
}

// A gate counts what it did not read, by reason, whatever its verdict.
func TestGateCountsUnread(t *testing.T) {
	t.Parallel()
	in := arch.GateInput{ModelHash: "m", Excluded: []arch.ExcludedFile{
		{Path: "a/gen.go", Reason: arch.ExcludedGenerated},
		{Path: "a/x_windows.go", Reason: arch.ExcludedByBuild},
		{Path: "b/gen.go", Reason: arch.ExcludedGenerated},
		{Path: "sub/go.mod", Reason: arch.ExcludedOtherModule},
	}}
	for name, b := range map[string]arch.Baseline{
		"comparable":     {ModelHash: "m"},
		"not comparable": {ModelHash: "other"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := arch.Gate(&in, &b, nil)
			equals(t, []arch.UnreadCount{
				{Reason: arch.ExcludedByBuild, Files: 1},
				{Reason: arch.ExcludedGenerated, Files: 2},
				{Reason: arch.ExcludedOtherModule, Files: 1},
			}, g.Unread)
		})
	}
	equals(t, "1 file the build constraints exclude, 2 generated files, 1 nested module",
		arch.DescribeUnread(arch.Gate(&in, &arch.Baseline{ModelHash: "m"}, nil).Unread))
	equals(t, "", arch.DescribeUnread(nil))
}

// A baseline measured in another build is named, and does not fail the
// gate; a baseline that records no build, or the same one, is not.
func TestGateNamesAnotherBuild(t *testing.T) {
	t.Parallel()
	in := arch.GateInput{
		ModelHash: "m",
		Build:     arch.Build{Platform: "darwin/arm64", Tags: []string{}},
	}
	linux := &arch.Build{Platform: "linux/amd64", Tags: []string{}}
	cases := map[string]struct {
		build *arch.Build
		want  *arch.Build
	}{
		"another platform": {build: linux, want: linux},
		"other tags": {
			build: &arch.Build{Platform: "darwin/arm64", Tags: []string{"legacy"}},
			want:  &arch.Build{Platform: "darwin/arm64", Tags: []string{"legacy"}},
		},
		"the same build":    {build: &arch.Build{Platform: "darwin/arm64"}},
		"no build recorded": {},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := arch.Gate(&in, &arch.Baseline{ModelHash: "m", Build: tc.build}, nil)
			equals(t, true, g.Passed)
			equals(t, tc.want, g.BaselineBuild)
		})
	}
}

// Shrinking a baseline of another build keeps every finding: this build
// may not see what that one does.
func TestShrinkKeepsAnotherBuildsBaseline(t *testing.T) {
	t.Parallel()
	b := arch.Baseline{
		ModelHash: "m",
		Build:     &arch.Build{Platform: "linux/amd64"},
		Findings:  []arch.BaselineFinding{{Fingerprint: "f", Rule: "r"}},
	}
	in := arch.GateInput{ModelHash: "m", Build: arch.Build{Platform: "darwin/arm64"}}
	equals(t, b.Findings, arch.Shrink(&b, &in).Findings)
	in.Build.Platform = "linux/amd64"
	equals(t, 0, len(arch.Shrink(&b, &in).Findings))
}
