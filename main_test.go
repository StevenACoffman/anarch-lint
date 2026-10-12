package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixture is the fixture module, with its own anarch.json, relative to the
// repository root where go test runs this package.
const fixture = "testdata/fx"

// connascenceFixture is the connascence and Balanced Coupling fixture module.
const connascenceFixture = "testdata/cx"

// fitnessModel is the fitness rules' model of the connascence fixture.
const fitnessModel = "testdata/cx/fitness.json"

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

func TestConnascenceGolden(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args []string
		file string
	}{
		"text": {args: []string{"-C", connascenceFixture, "connascence"}, file: "connascence.txt"},
		"json": {
			args: []string{"-C", connascenceFixture, "--format=json", "connascence"},
			file: "connascence.json",
		},
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

func TestBalanceGolden(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args []string
		file string
	}{
		"text": {args: []string{"-C", connascenceFixture, "balance"}, file: "balance.txt"},
		"json": {
			args: []string{"-C", connascenceFixture, "--format=json", "balance"},
			file: "balance.json",
		},
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

// With connascence and balance sections in the model, baseline and check
// gate their findings too.
func TestGateIncludesConnascenceAndBalance(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	accepted := filepath.Join(dir, "accepted.json")
	if code, _, errOut := runCLI(
		t,
		"-C",
		connascenceFixture,
		"baseline",
		"--out",
		accepted,
	); code != 0 {
		t.Fatalf("baseline: exit %d: %s", code, errOut)
	}
	src, err := os.ReadFile(accepted)
	if err != nil {
		t.Fatal(err)
	}
	var b struct {
		ModelHash string `json:"modelHash"`
		Findings  []struct {
			Rule string `json:"rule"`
		} `json:"findings"`
		Metrics []struct {
			Name string `json:"name"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(src, &b); err != nil {
		t.Fatal(err)
	}
	rules := map[string]bool{}
	for _, f := range b.Findings {
		rules[f.Rule] = true
	}
	for _, want := range []string{"connascence-identity", "tight-coupling-intrusive", "subsystem-owners-differ"} {
		if !rules[want] {
			t.Errorf("the baseline records no %s finding: %v", want, rules)
		}
	}
	metrics := map[string]bool{}
	for _, m := range b.Metrics {
		metrics[m.Name] = true
	}
	if !metrics["balance.strongRefs/web->auth"] {
		t.Errorf("the baseline ratchets no strong references of web -> auth: %v", metrics)
	}

	if code, out, _ := runCLI(
		t,
		"-C",
		connascenceFixture,
		"check",
		"--baseline",
		accepted,
	); code != 0 {
		t.Fatalf("check against its own baseline: exit %d\n%s", code, out)
	}
	empty := filepath.Join(dir, "empty.json")
	emptyBaseline := `{"schema": "anarch-lint.baseline.v1", "modelHash": "` + b.ModelHash +
		`", "findings": [], "metrics": []}`
	if err := os.WriteFile(empty, []byte(emptyBaseline), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI(t, "-C", connascenceFixture, "check", "--baseline", empty)
	if code != 3 || !strings.Contains(out, "tight-coupling-intrusive") {
		t.Fatalf("check against an empty baseline: exit %d\n%s", code, out)
	}
}

func TestSchemaGolden(t *testing.T) {
	t.Parallel()
	code, out, errOut := runCLI(t, "schema")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	golden(t, "schema.json", out)
}

func TestFitnessGolden(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args []string
		file string
	}{
		"text": {
			args: []string{"-C", connascenceFixture, "--model", fitnessModel, "fitness"},
			file: "fitness.txt",
		},
		"json": {
			args: []string{
				"-C",
				connascenceFixture,
				"--model",
				fitnessModel,
				"--format=json",
				"fitness",
			},
			file: "fitness.json",
		},
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

func TestFitnessIsByteIdentical(t *testing.T) {
	t.Parallel()
	args := []string{"-C", connascenceFixture, "--model", fitnessModel, "--format=json", "fitness"}
	_, first, _ := runCLI(t, args...)
	_, second, _ := runCLI(t, args...)
	if first != second {
		t.Fatal("two runs over the same inputs produced different output")
	}
}

// With a fitness section, check gates its findings, and baseline --shrink
// removes only what was fixed.
func TestFitnessGateAndShrink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	accepted := filepath.Join(dir, "accepted.json")
	run := func(args ...string) (int, string) {
		t.Helper()
		code, out, _ := runCLI(
			t,
			append([]string{"-C", connascenceFixture, "--model", fitnessModel}, args...)...)
		return code, out
	}
	if code, out := run("baseline", "--out", accepted); code != 0 {
		t.Fatalf("baseline: exit %d\n%s", code, out)
	}
	if code, out := run("check", "--baseline", accepted); code != 0 {
		t.Fatalf("check against its own baseline: exit %d\n%s", code, out)
	}

	// A baseline that also accepts a finding the code no longer has: check
	// passes and says the baseline can shrink; --shrink removes the entry.
	src, err := os.ReadFile(accepted)
	if err != nil {
		t.Fatal(err)
	}
	var b map[string]any
	if err := json.Unmarshal(src, &b); err != nil {
		t.Fatal(err)
	}
	findings, _ := b["findings"].([]any)
	before := len(findings)
	b["findings"] = append(findings, map[string]any{
		"fingerprint": "00000000000000ff", "rule": "forbidden-call", "severity": "error",
		"subject": "gone", "object": "gone",
	})
	stale, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(accepted, stale, 0o600); err != nil {
		t.Fatal(err)
	}
	code, out := run("check", "--baseline", accepted)
	if code != 0 || !strings.Contains(out, "the baseline can shrink: 1 fixed findings") {
		t.Fatalf("check with a stale entry: exit %d\n%s", code, out)
	}
	if code, out := run("baseline", "--out", accepted, "--shrink"); code != 0 {
		t.Fatalf("baseline --shrink: exit %d\n%s", code, out)
	}
	src, err = os.ReadFile(accepted)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(src, &b); err != nil {
		t.Fatal(err)
	}
	if after, _ := b["findings"].([]any); len(after) != before {
		t.Fatalf("shrunk baseline has %d findings, want %d", len(after), before)
	}
}

func TestBalanceIsByteIdentical(t *testing.T) {
	t.Parallel()
	_, first, _ := runCLI(t, "-C", connascenceFixture, "--format=json", "balance")
	_, second, _ := runCLI(t, "-C", connascenceFixture, "--format=json", "balance")
	if first != second {
		t.Fatal("two runs over the same inputs produced different output")
	}
}

func TestConnascenceIsByteIdentical(t *testing.T) {
	t.Parallel()
	_, first, _ := runCLI(t, "-C", connascenceFixture, "--format=json", "connascence")
	_, second, _ := runCLI(t, "-C", connascenceFixture, "--format=json", "connascence")
	if first != second {
		t.Fatal("two runs over the same inputs produced different output")
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

func TestRenderCheck(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "anarch.md")
	args := []string{"-C", connascenceFixture, "--model", fitnessModel, "render", "--out", out}
	if code, _, errOut := runCLI(t, append(args, "--check")...); code != 3 {
		t.Fatalf("check of a missing file: exit %d, want 3: %s", code, errOut)
	}
	if code, _, errOut := runCLI(t, args...); code != 0 {
		t.Fatalf("render: exit %d: %s", code, errOut)
	}
	if code, _, errOut := runCLI(t, append(args, "--check")...); code != 0 {
		t.Fatalf("check of a fresh rendering: exit %d: %s", code, errOut)
	}
	if err := os.WriteFile(out, []byte("# edited by hand\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := runCLI(t, append(args, "--check")...); code != 3 {
		t.Fatalf("check of a drifted file: exit %d, want 3: %s", code, errOut)
	}
}

// docs/adr/0001: every decision record after the first is pinned by a test
// named after it.
func TestEveryDecisionRecordIsPinned(t *testing.T) {
	t.Parallel()
	records, err := filepath.Glob(filepath.Join("docs", "adr", "[0-9][0-9][0-9][0-9]-*.md"))
	if err != nil || len(records) == 0 {
		t.Fatalf("no decision records: %v", err)
	}
	var tests strings.Builder
	repo := os.DirFS(".")
	err = fs.WalkDir(repo, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := fs.ReadFile(repo, path)
		tests.Write(src)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		n := filepath.Base(record)[:4]
		if n != "0001" && !strings.Contains(tests.String(), "func TestADR_"+n+"_") {
			t.Errorf("%s has no TestADR_%s_ test", record, n)
		}
	}
}

func TestRenderGolden(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "anarch.md")
	code, _, errOut := runCLI(t,
		"-C", connascenceFixture, "--model", fitnessModel, "render", "--out", out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	src, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "render.md", string(src))
}

// TestCorpusSkillet runs domain-model conformance on skillet's own model
// with no type mappings, and expects the four drifts mmi-plan.md §10.0 found
// by hand.
func TestCorpusSkillet(t *testing.T) {
	t.Parallel()
	if *corpus == "" {
		t.Skip("set -corpus to the directory holding a clone of skillet")
	}
	model, err := filepath.Abs(filepath.Join("testdata", "corpus", "skillet.json"))
	if err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(t, "-C", filepath.Join(*corpus, "skillet"), "--model", model,
		"--format=json", "fitness", "./...")
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
	found := map[string]bool{}
	for _, f := range r.Findings {
		found[f.Rule+" "+f.Subject] = true
	}
	for _, want := range []string{
		"domain-entity-missing skillet.modelith.yaml:Prompt",
		"domain-entity-ambiguous skillet.modelith.yaml:Skill",
		"domain-entity-missing skillet.modelith.yaml:Proof",
		"domain-enum-missing skillet.modelith.yaml:RuleSeverity",
	} {
		if !found[want] {
			t.Errorf("expected %s; found %v", want, found)
		}
	}
}
