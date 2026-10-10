package text_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
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
