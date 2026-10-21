package baseline_test

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/anarch-lint/cmd/baseline"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// A baseline that cannot be encoded leaves the old file whole, rather than
// truncating it before the write fails.
func TestWriteBaselineKeepsTheOldFileOnFailure(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "anarch-baseline.json")
	old := arch.Baseline{Schema: arch.BaselineSchema, ModelHash: "m"}
	if err := baseline.WriteBaseline(path, &old); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := arch.Baseline{Metrics: []arch.Metric{{Name: "x", Value: math.NaN()}}}
	if err := baseline.WriteBaseline(path, &bad); err == nil {
		t.Fatal("a NaN metric was encoded")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("the old baseline changed:\nbefore: %s\nafter:  %s", before, after)
	}
}
