// Package anarchtest runs anarch-lint's check inside go test, so a module can
// gate its architecture without the CLI, as ArchUnit's rules run as JUnit
// tests (TODO.md, "Rules that run inside go test"):
//
//	func TestArchitecture(t *testing.T) {
//		anarchtest.Check(t, nil)
//	}
//
// The test loads and type-checks the packages with go/packages, as the CLI
// does, so it takes seconds rather than milliseconds.
package anarchtest

import (
	"bytes"
	"cmp"
	"context"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// T is what Check needs of a test; *testing.T and *testing.B have it. It is
// an interface, declared here, so this package need not import testing and
// add its flags to a binary that imports this one.
type T interface {
	Helper()
	Context() context.Context
	Errorf(format string, args ...any)
}

// Options say what to check, as check's flags do. The zero value, and a nil
// pointer, check ./... in the current directory, which go test makes the
// package's, against the model and baseline there.
type Options struct {
	// Dir is the directory to analyze; empty means the current one.
	Dir string
	// Patterns are the packages to load; empty means ./....
	Patterns []string
	// Model, Baseline, Waivers and Judgements name files as the flags of the
	// same names do; empty means each one's default in Dir.
	Model, Baseline, Waivers, Judgements string
}

// Check measures the packages, compares them with the baseline, and fails t,
// with the report check prints, when the gate fails: a new error finding, a
// metric worse than the baseline's, or a floor not met.
func Check(t T, opts *Options) {
	t.Helper()
	if opts == nil {
		opts = &Options{}
	}
	o := pipeline.Options{
		Dir: cmp.Or(opts.Dir, "."), ModelPath: opts.Model, JudgementsPath: opts.Judgements,
	}
	in, err := o.GateInput(t.Context(), opts.Patterns, nil)
	if err != nil {
		t.Errorf("anarchtest: %v", err)
		return
	}
	baseline, err := o.ReadBaseline(opts.Baseline)
	if err != nil {
		t.Errorf("anarchtest: %v", err)
		return
	}
	waivers, today, err := o.ReadWaivers(opts.Waivers, "")
	if err != nil {
		t.Errorf("anarchtest: %v", err)
		return
	}
	r := arch.Gate(&in, &baseline, &arch.GateOptions{Waivers: waivers, Today: today})
	if r.Passed {
		return
	}
	var report bytes.Buffer
	if err := text.WriteGate(&report, &r); err != nil {
		t.Errorf("anarchtest: %v", err)
		return
	}
	t.Errorf("anarchtest: the architecture gate failed:\n%s", report.String())
}
