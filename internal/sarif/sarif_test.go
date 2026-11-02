package sarif_test

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	gosarif "github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/xeipuuv/gojsonschema"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/sarif"
)

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// TestWriteGate: every new, warned or waived finding is a result with its
// rule, level, place and fingerprint; a waived one is suppressed; a finding
// about the model points at the model file; and a regression, which has no
// place in the code, is a notification. The log is valid SARIF 2.1.0.
func TestWriteGate(t *testing.T) {
	t.Parallel()
	finding := func(rule string, sev arch.Severity, loc *arch.Span) arch.Finding {
		f := arch.NewFinding(arch.FindingID{Rule: rule, Subject: "m/a"}, sev)
		f.Location, f.Message, f.Fix = loc, rule+" message", "do something"
		return f
	}
	governed := finding(
		"layer-violation",
		arch.SeverityError,
		&arch.Span{File: "a/a.go", Start: 3, End: 3},
	)
	governed.ModelRule, governed.QualityAttribute = "layers", arch.QualityMaintainability
	result := arch.GateResult{
		Comparable: true,
		New: []arch.Finding{
			governed,
			finding("package-outside-architecture", arch.SeverityError, nil),
		},
		NewWarnings: []arch.Finding{
			finding(
				"deep-nesting",
				arch.SeverityWarning,
				&arch.Span{File: "a/b.go", Start: 7, End: 20},
			),
		},
		Waived: []arch.Finding{
			finding("interface-bypass", arch.SeverityError, &arch.Span{File: "a/c.go", Start: 1}),
		},
		Regressions: []arch.Regression{{Metric: "subsystems.inCycles", Baseline: 0, Current: 2}},
		Unread:      []arch.UnreadCount{{Reason: arch.ExcludedOtherModule, Files: 1}},
	}
	var buf bytes.Buffer
	if err := sarif.WriteGate(
		&buf,
		&result,
		sarif.Options{Version: "v1.0.0", ModelPath: "anarch.json"},
	); err != nil {
		t.Fatal(err)
	}
	validate(t, buf.Bytes())
	log, err := gosarif.FromBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	run := log.Runs[0]
	equals(t, "v1.0.0", *run.Tool.Driver.Version)
	var rules []string
	for _, r := range run.Tool.Driver.Rules {
		rules = append(rules, r.ID)
	}
	equals(
		t,
		[]string{
			"deep-nesting",
			"interface-bypass",
			"layer-violation",
			"package-outside-architecture",
		},
		rules,
	)

	type got struct {
		rule, level, file string
		line              int
		suppressed        bool
	}
	var results []got
	for _, r := range run.Results {
		g := got{rule: *r.RuleID, level: *r.Level, suppressed: len(r.Suppressions) > 0}
		loc := r.Locations[0].PhysicalLocation
		g.file = *loc.ArtifactLocation.URI
		if loc.Region != nil {
			g.line = *loc.Region.StartLine
		}
		equals(t, "new", *r.BaselineState)
		equals(t, 1, len(r.PartialFingerprints))
		results = append(results, g)
	}
	equals(t, map[string]any{"modelRule": "layers", "qualityAttribute": "maintainability"},
		map[string]any(run.Results[0].Properties))
	equals(t, 0, len(run.Results[1].Properties))
	equals(t, []got{
		{rule: "layer-violation", level: "error", file: "a/a.go", line: 3},
		{rule: "package-outside-architecture", level: "error", file: "anarch.json"},
		{rule: "deep-nesting", level: "warning", file: "a/b.go", line: 7},
		{rule: "interface-bypass", level: "error", file: "a/c.go", line: 1, suppressed: true},
	}, results)
	notes := run.Invocations[0].ToolExecutionNotifications
	equals(t, 2, len(notes))
	equals(t, "not read: 1 nested module", *notes[0].Message.Text)
	equals(t, "note", notes[0].Level)
	equals(t, "subsystems.inCycles regressed from 0 to 2", *notes[1].Message.Text)
}

// validate checks a log against the SARIF 2.1.0 schema in testdata, OASIS's
// errata 01 version.
func validate(t *testing.T, log []byte) {
	t.Helper()
	schema, err := os.ReadFile("testdata/sarif-schema-2.1.0.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := gojsonschema.Validate(
		gojsonschema.NewBytesLoader(schema),
		gojsonschema.NewBytesLoader(log),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid() {
		var msgs []string
		for _, e := range res.Errors() {
			msgs = append(msgs, e.String())
		}
		t.Fatalf("invalid SARIF:\n%s", strings.Join(msgs, "\n"))
	}
}
