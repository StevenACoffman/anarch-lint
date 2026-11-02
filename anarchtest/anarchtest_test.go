package anarchtest_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/anarchtest"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
)

// recorder is a T that records what Check reports.
type recorder struct {
	ctx    context.Context
	errors []string
}

func (r *recorder) Helper()                  {}
func (r *recorder) Context() context.Context { return r.ctx }
func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

// TestCheck: a gate that cannot run, here for a missing baseline, fails the
// test and says why. anarch-lint's own TestSelfCheck shows a passing gate.
func TestCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks modules; skipped with -short")
	}
	r := &recorder{ctx: t.Context()}
	anarchtest.Check(r, &anarchtest.Options{
		Dir: "../testdata/fx", Baseline: "../testdata/fx/no-such-baseline.json",
	})
	if len(r.errors) != 1 || !strings.Contains(r.errors[0], "no baseline") {
		t.Fatalf("want one error naming the missing baseline, got %q", r.errors)
	}
}

// TestCheckFails: the fixture module's findings, against a baseline that
// accepts none of them, fail the test with check's report.
func TestCheckFails(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	const fx = "../testdata/fx"
	o := pipeline.Options{Dir: fx}
	in, err := o.GateInput(t.Context(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b := arch.NewBaseline(&in)
	b.Findings = nil
	var buf bytes.Buffer
	if err := arch.WriteJSON(&buf, &b); err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(baseline, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &recorder{ctx: t.Context()}
	anarchtest.Check(r, &anarchtest.Options{Dir: fx, Baseline: baseline})
	if len(r.errors) != 1 || !strings.Contains(r.errors[0], "GATE FAILED") ||
		!strings.Contains(r.errors[0], "NEW ERRORS") {
		t.Fatalf("want the gate's report, got %q", r.errors)
	}
}
