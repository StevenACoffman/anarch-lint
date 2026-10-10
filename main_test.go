package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixture is the fixture module, with its own anarch.json, relative to the
// repository root where go test runs this package.
const fixture = "testdata/fx"

var (
	update = flag.Bool("update", false, "rewrite the golden files in testdata/golden")
	corpus = flag.String("corpus", "",
		"directory holding clones of the test corpora (go-ddd); empty skips the acceptance tests")
)

// runCLI runs the whole CLI as main would, returning the exit code and output.
func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	if testing.Short() {
		t.Skip("loads and type-checks the fixture module; skipped with -short")
	}
	var out, errOut bytes.Buffer
	code = run(context.Background(), append([]string{"anarch-lint"}, args...),
		strings.NewReader(""), &out, &errOut)
	return code, out.String(), errOut.String()
}

// golden compares got with a golden file, after replacing what legitimately
// differs between machines: the Go toolchain's version.
func golden(t *testing.T, name, got string) {
	t.Helper()
	got = strings.ReplaceAll(got, runtime.Version(), "GOVERSION")
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file (run go test -update to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf(
			"%s differs from the golden file; review the diff, then run go test -update\n got:\n%s",
			name,
			got,
		)
	}
}

func TestAnalyzeGolden(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args []string
		file string
	}{
		"text": {args: []string{"-C", fixture, "analyze"}, file: "analyze.txt"},
		"json": {args: []string{"-C", fixture, "--format=json", "analyze"}, file: "analyze.json"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			code, out, errOut := runCLI(t, tc.args...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, errOut)
			}
			golden(t, tc.file, out)
		})
	}
}

// The whole stack, loader included, must give the same bytes twice.
func TestAnalyzeIsByteIdentical(t *testing.T) {
	t.Parallel()
	_, first, _ := runCLI(t, "-C", fixture, "--format=json", "analyze")
	_, second, _ := runCLI(t, "-C", fixture, "--format=json", "analyze")
	if first != second {
		t.Fatal("two runs over the same inputs produced different output")
	}
}

func TestCheckExitCodes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	accepted := filepath.Join(dir, "accepted.json")
	if code, _, errOut := runCLI(t, "-C", fixture, "baseline", "--out", accepted); code != 0 {
		t.Fatalf("baseline: exit %d: %s", code, errOut)
	}

	// An empty baseline under the same model: every error finding is new.
	_, report, _ := runCLI(t, "-C", fixture, "--format=json", "analyze")
	var r struct {
		Inputs struct {
			ModelHash string `json:"modelHash"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal([]byte(report), &r); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.json")
	emptyBaseline := `{"schema": "anarch-lint.baseline.v1", "modelHash": "` + r.Inputs.ModelHash +
		`", "findings": [], "metrics": []}`
	if err := os.WriteFile(empty, []byte(emptyBaseline), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		args []string
		code int
	}{
		"the accepted state passes": {
			args: []string{"-C", fixture, "check", "--baseline", accepted},
			code: 0,
		},
		"new findings fail the gate": {
			args: []string{"-C", fixture, "check", "--baseline", empty},
			code: 3,
		},
		"a missing baseline is an error": {
			args: []string{"-C", fixture, "check", "--baseline", filepath.Join(dir, "none")},
			code: 1,
		},
		"an unknown flag is a usage error": {args: []string{"check", "--no-such-flag"}, code: 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			code, out, errOut := runCLI(t, tc.args...)
			if code != tc.code {
				t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.code, out, errOut)
			}
		})
	}
}

// TestSelfCheck runs anarch-lint's own fitness function: the repository must
// pass check against its committed model (anarch.json) and baseline
// (anarch-baseline.json). A change that breaks the layering, imports I/O into
// the domain, or adds a cycle fails here; run `go run . baseline` only after
// deciding the new state is acceptable.
func TestSelfCheck(t *testing.T) {
	t.Parallel()
	code, out, errOut := runCLI(t, "check")
	if code != 0 {
		t.Fatalf("self-check exit %d\n%s%s", code, out, errOut)
	}
}

// Waivers turn a failing gate into a passing one until they expire. --today
// pins the date, so the test never depends on the clock.
func TestCheckWaivers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, report, _ := runCLI(t, "-C", fixture, "--format=json", "analyze")
	var r struct {
		Inputs struct {
			ModelHash string `json:"modelHash"`
		} `json:"inputs"`
		Findings []struct {
			Fingerprint string `json:"fingerprint"`
			Severity    string `json:"severity"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(report), &r); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.json")
	emptyBaseline := `{"schema": "anarch-lint.baseline.v1", "modelHash": "` + r.Inputs.ModelHash +
		`", "findings": [], "metrics": []}`
	waivers := filepath.Join(dir, "waivers.json")
	var entries []string
	for _, f := range r.Findings {
		if f.Severity == "error" {
			entries = append(entries, `{"fingerprint": "`+f.Fingerprint+
				`", "reason": "accepted for now", "approvedBy": "a reviewer", "expires": "2027-01-01"}`)
		}
	}
	for path, content := range map[string]string{
		empty:   emptyBaseline,
		waivers: `{"version": 1, "waivers": [` + strings.Join(entries, ",") + `]}`,
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cases := map[string]struct {
		today string
		code  int
	}{
		"before expiry the waivers hold":    {today: "2026-12-31", code: 0},
		"after expiry they protect nothing": {today: "2027-01-02", code: 3},
		"a malformed date is an error":      {today: "2027-13-01", code: 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			code, out, errOut := runCLI(t, "-C", fixture, "check", "--baseline", empty,
				"--waivers", waivers, "--today", tc.today)
			if code != tc.code {
				t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.code, out, errOut)
			}
		})
	}
}

// TestCorpusGoDDD is an acceptance test on a real codebase, sklinkert/go-ddd,
// whose README documents its layers (mmi-ref-plan.md §5). Its interface layer
// imports domain entities in two packages; the model in
// testdata/corpus/go-ddd.json forbids that, so both must be found. Run it with
// go test -run TestCorpusGoDDD -corpus ~/Documents/git
func TestCorpusGoDDD(t *testing.T) {
	t.Parallel()
	if *corpus == "" {
		t.Skip("set -corpus to the directory holding a clone of go-ddd")
	}
	model, err := filepath.Abs(filepath.Join("testdata", "corpus", "go-ddd.json"))
	if err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(t, "-C", filepath.Join(*corpus, "go-ddd"), "--model", model,
		"--format=json", "analyze", "./internal/...")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var r struct {
		Findings []struct {
			Rule    string `json:"rule"`
			Subject string `json:"subject"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	leaks := map[string]bool{}
	for _, f := range r.Findings {
		if f.Rule == "forbidden-import" {
			leaks[f.Subject] = true
		}
	}
	const rest = "github.com/sklinkert/go-ddd/internal/interface/api/rest"
	for _, pkg := range []string{rest, rest + "/dto/request"} {
		if !leaks[pkg] {
			t.Errorf("the domain leak in %s was not found; found %v", pkg, leaks)
		}
	}
}
