//go:build cgo

package gopackages_test

import (
	"context"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/gopackages"
)

// A file that imports C is measured as its source: at its own path and
// lines, importing C rather than the unsafe cgo writes in its place, and
// with nothing of what cgo generates listed as excluded. The test is built
// only with cgo, because without it the build constraints exclude the file.
func TestLoadReadsCgoFilesAsTheirSource(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(),
		gopackages.Config{Dir: "../../testdata/gx", Patterns: []string{"./cgo"}})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	equals(t, []arch.File{{Path: "cgo/cgo.go", Package: "example.com/gx/cgo", Lines: 6}}, p.Files)
	if len(p.Excluded) > 0 {
		t.Fatalf("cgo's files were listed as excluded: %v", p.Excluded)
	}
	var spans []arch.Span
	for _, fn := range p.Funcs {
		spans = append(spans, fn.Span)
	}
	equals(t, []arch.Span{{File: "cgo/cgo.go", Start: 11, End: 13}}, spans)
	equals(t, []arch.Import{
		{From: "example.com/gx/cgo", To: "C", Pos: arch.Position{File: "cgo/cgo.go", Line: 6}},
		{
			From: "example.com/gx/cgo",
			To:   "strconv",
			Pos:  arch.Position{File: "cgo/cgo.go", Line: 8},
		},
	}, p.Imports)
}
