package text_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

func intPtr(n int) *int { return &n }

func floatPtr(x float64) *float64 { return &x }

// contains fails the test unless out contains every one of wants.
func contains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q\noutput:\n%s", want, out)
		}
	}
}

func TestWriteReportMMIHeader(t *testing.T) {
	t.Parallel()
	scored := arch.Criterion{
		ID: "2.2.1", Name: "Types in cycles", Kind: arch.Measured,
		Value: floatPtr(2.5), Unit: "%", Score: intPtr(9),
	}
	cases := map[string]struct {
		mmi   arch.MMI
		wants []string
	}{
		"exact": {
			mmi:   arch.MMI{Low: 7.25, High: 7.25, Exact: true, Criteria: []arch.Criterion{scored}},
			wants: []string{"\nMMI 7.25\n", "2.2.1", "2.50%", "score 9"},
		},
		"interval counts what is open": {
			mmi: arch.MMI{Low: 2, High: 8, Criteria: []arch.Criterion{
				scored,
				{ID: "1.1.5", Kind: arch.Judgement},
				{ID: "2.1.1", Kind: arch.Unmeasured},
			}},
			wants: []string{
				"MMI 2.00 – 8.00  (1 judgements pending, 1 criteria unmeasured)",
				"unscored",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			r := arch.Report{MMI: tc.mmi}
			if err := text.WriteReport(&buf, &r); err != nil {
				t.Fatal(err)
			}
			contains(t, buf.String(), tc.wants...)
		})
	}
}

func TestWriteFindingShowsEvidenceAndFix(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	text.WriteFinding(&buf, &arch.Finding{
		Rule: arch.RuleSubsystemCycle, Severity: arch.SeverityError, Subject: "a ↔ b",
		Location: &arch.Span{File: "a/a.go", Start: 4, End: 4},
		Message:  "subsystems a ↔ b form a cycle",
		Evidence: []string{"a imports b at a/a.go:4"},
		Fix:      "cut a → b",
	})
	contains(
		t,
		buf.String(),
		"error",
		"subsystem-cycle",
		"a/a.go:4",
		"evidence: a imports b at a/a.go:4",
		"fix: cut a → b",
	)
}

func TestWriteGate(t *testing.T) {
	t.Parallel()
	newErr := arch.Finding{
		Rule:     arch.RuleForbiddenImport,
		Severity: arch.SeverityError,
		Subject:  "x",
		Fix:      "f",
	}
	cases := map[string]struct {
		gate    arch.GateResult
		wants   []string
		absents []string
	}{
		"passed": {
			gate:    arch.GateResult{Passed: true, Comparable: true},
			wants:   []string{"GATE PASSED\n"},
			absents: []string{"NEW ERRORS", "scoped"},
		},
		"failed with a regression and a new error": {
			gate: arch.GateResult{
				Reason:      "1 new error findings, 1 metric regressions",
				New:         []arch.Finding{newErr},
				Regressions: []arch.Regression{{Metric: "types.sdi", Baseline: 11, Current: 22}},
				Fixed:       []arch.BaselineFinding{{Rule: arch.RuleLargeFunc, Subject: "old"}},
			},
			wants: []string{
				"GATE FAILED: 1 new error findings", "regression", "types.sdi", "11 -> 22",
				"NEW ERRORS (1)", "forbidden-import", "fixed", "old",
			},
		},
		"passed over what it read": {
			gate: arch.GateResult{Passed: true, Comparable: true, Unread: []arch.UnreadCount{
				{Reason: arch.ExcludedByBuild, Files: 1},
				{Reason: arch.ExcludedGenerated, Files: 2},
				{Reason: arch.ExcludedOtherModule, Files: 1},
			}},
			wants: []string{"GATE PASSED\nnot read: 1 file the build constraints exclude, " +
				"2 generated files, 1 nested module, which analyze lists\n"},
		},
		"against another build's baseline": {
			gate: arch.GateResult{
				Passed: true, Comparable: true,
				BaselineBuild: &arch.Build{Platform: "linux/amd64"},
			},
			wants: []string{"note: the baseline was measured for linux/amd64 with tags none, " +
				"another build than this one"},
		},
		"scoped to a change": {
			gate:  arch.GateResult{Passed: true, Scoped: true, OutOfScope: 3},
			wants: []string{"scoped to changed lines; 3 new findings elsewhere set aside"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := text.WriteGate(&buf, &tc.gate); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			contains(t, out, tc.wants...)
			for _, absent := range tc.absents {
				if strings.Contains(out, absent) {
					t.Errorf("output has %q\noutput:\n%s", absent, out)
				}
			}
		})
	}
}

// The temporal sections appear only with a revision range.
func TestWriteBalanceTemporal(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		since  string
		wants  []string
		absent string
	}{
		"with a range": {
			since: "aaaaaaaaaaaa1",
			wants: []string{
				"CO-CHANGE (1 pairs", "billing ↔ web", "2 commits shared", "score 0.5",
				"HOTSPOTS (1;", "m/billing", "1 commit ", "complexity 3",
			},
		},
		"without a range": {absent: "CO-CHANGE"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := balance.Report{
				Inputs:   balance.ReportInputs{Since: tc.since, Head: "bbbbbbbbbbbb2"},
				CoChange: []balance.CoChange{{A: "billing", B: "web", Shared: 2, Score: 0.5}},
				Hotspots: []balance.Hotspot{{Package: "m/billing", Commits: 1, Complexity: 3}},
			}
			var buf bytes.Buffer
			if err := text.WriteBalance(&buf, &r); err != nil {
				t.Fatal(err)
			}
			contains(t, buf.String(), tc.wants...)
			if tc.absent != "" && strings.Contains(buf.String(), tc.absent) {
				t.Errorf("output has %q without a range:\n%s", tc.absent, buf.String())
			}
		})
	}
}

// The gate lists its new findings with the widest reach first, saying how
// many packages depend on each, and names each loosened bar.
func TestWriteGateRanksByReach(t *testing.T) {
	t.Parallel()
	finding := func(subject string, reach int) arch.Finding {
		f := arch.NewFinding(arch.FindingID{Rule: "r", Subject: subject}, arch.SeverityError)
		f.Reach = reach
		return f
	}
	var buf bytes.Buffer
	g := arch.GateResult{
		New:      []arch.Finding{finding("narrow", 1), finding("wide", 5)},
		Loosened: []string{"thresholds.largeFuncLines raised from 40 to 80"},
	}
	if err := text.WriteGate(&buf, &g); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Index(out, "wide") > strings.Index(out, "narrow") {
		t.Errorf("the wider finding is not first:\n%s", out)
	}
	contains(t, out, "reach: 5 packages depend on it", "reach: 1 package depends on it",
		"loosened  thresholds.largeFuncLines raised from 40 to 80")
}
