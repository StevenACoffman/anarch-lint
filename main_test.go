package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/anarchtest"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
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
// differs between machines: the Go toolchain's version and the platform.
func golden(t *testing.T, name, got string) {
	t.Helper()
	got = strings.ReplaceAll(got, runtime.Version(), "GOVERSION")
	got = strings.ReplaceAll(got, runtime.GOOS+"/"+runtime.GOARCH, "PLATFORM")
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
		"new findings fail the gate in SARIF too": {
			args: []string{"-C", fixture, "--format=sarif", "check", "--baseline", empty},
			code: 3,
		},
		"SARIF is for check only": {
			args: []string{"-C", fixture, "--format=sarif", "analyze"},
			code: 1,
		},
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
// It runs through anarchtest, the gate as a go test, which the CLI's check
// shares its pipeline with.
func TestSelfCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks the module; skipped with -short")
	}
	anarchtest.Check(t, nil)
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

// TestExplain: explain names the rules that examine an import and what each
// decides, a missing import says so, and too few arguments are a usage error.
func TestExplain(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args  []string
		code  int
		wants []string
	}{
		"a bypass": {
			args: []string{"example.com/fx/billing/tax", "example.com/fx/shop/catalog"},
			wants: []string{
				"example.com/fx/billing/tax imports example.com/fx/shop/catalog at billing/tax/tax.go:4",
				"forbids  interfaces",
			},
		},
		"no such import": {
			args:  []string{"example.com/fx/shop/catalog", "example.com/fx/billing/tax"},
			wants: []string{"does not import"},
		},
		"one argument": {args: []string{"example.com/fx/shop/catalog"}, code: 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			code, out, errOut := runCLI(
				t,
				append([]string{"-C", fixture, "explain"}, tc.args...)...)
			if code != tc.code {
				t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.code, out, errOut)
			}
			for _, want := range tc.wants {
				if !strings.Contains(out, want) {
					t.Errorf("output lacks %q:\n%s", want, out)
				}
			}
		})
	}
}

// TestJudge: judge emit writes a prompt per item, admit records the tallies
// of two judge configurations that answer alike as drafts in the
// judgements file, and a configuration that misses a canary makes admit
// exit 4, the status of an untrustworthy run.
func TestJudge(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		wrongCanaries bool
		code          int
	}{
		"trustworthy judges": {code: 0},
		"a canary missed":    {wrongCanaries: true, code: 4},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			run, cache := filepath.Join(dir, "run"), filepath.Join(dir, "cache")
			judgements := filepath.Join(dir, "judgements.json")
			base := []string{
				"-C", connascenceFixture, "--judgements", judgements, "judge",
				"--run", run, "--cache", cache,
			}
			code, out, errOut := runCLI(t, append(base, "emit",
				"--config", "a=model-one", "--config", "b=model-two")...)
			if code != 0 {
				t.Fatalf("emit exit %d: %s%s", code, out, errOut)
			}
			answerRun(t, run, tc.wrongCanaries)
			code, out, errOut = runCLI(t, append(base, "admit", "--signer", "a reviewer")...)
			if code != tc.code {
				t.Fatalf("admit exit %d, want %d: %s%s", code, tc.code, out, errOut)
			}
			src, err := os.ReadFile(judgements)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(src), `"role": "judge"`) {
				t.Fatalf("no judge's event was recorded:\n%s", src)
			}
			if code, out, errOut = runCLI(t, append(base, "report")...); code != 0 ||
				!strings.Contains(out, "TALLIES") {
				t.Fatalf("report exit %d: %s%s", code, out, errOut)
			}
		})
	}
}

// answerRun writes a reply to every item of a judging run: a canary's
// labelled answer, or its opposite when wrongCanaries; for any other item
// the family's first answer, or a score of 5.
func answerRun(t *testing.T, run string, wrongCanaries bool) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(run, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Items []struct {
			Key  string `json:"key"`
			Kind string `json:"kind"`
			View struct {
				ID      string   `json:"id"`
				Answers []string `json:"answers"`
			} `json:"view"`
		} `json:"items"`
	}
	if err := json.Unmarshal(src, &m); err != nil {
		t.Fatal(err)
	}
	labels := map[string]string{
		"connascence/5e1a07c2d9b34f18":                      "confirmed",
		"connascence/9b03c6e14a7f5d22":                      "rejected",
		"connascence/c4d8e2f0a6b19357":                      "rejected",
		"contract-type/example.com/shop/api.OrderPlaced":    "contract",
		"contract-type/example.com/shop/order.Order":        "model",
		"contract-type/example.com/shop/order.OrderRequest": "model",
	}
	for _, it := range m.Items {
		choice := `"score": 5`
		if len(it.View.Answers) > 0 {
			choice = fmt.Sprintf("%q: %q", "answer", it.View.Answers[0])
		}
		if label, ok := labels[it.View.ID]; ok && it.Kind == "canary" {
			if wrongCanaries {
				label = map[string]string{
					"confirmed": "rejected", "rejected": "confirmed",
					"contract": "model", "model": "contract",
				}[label]
			}
			choice = fmt.Sprintf("%q: %q", "answer", label)
		}
		reply := fmt.Sprintf("```json\n{%s, \"confidence\": 0.7, \"cites\": [\"E1\"], "+
			"\"rationale\": \"checked %s\"}\n```\n", choice, it.Key)
		if err := os.WriteFile(
			filepath.Join(run, it.Key+".reply.md"),
			[]byte(reply),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCorpusGoDDD is an acceptance test on a real codebase, sklinkert/go-ddd,
// whose README documents its layers (mmi-ref-plan.md §5). Its interface layer
// imports domain entities in two packages; the model in
// testdata/corpus/go-ddd.json forbids that, so both must be found. The whole
// report is a golden file, so any change in what the pinned clone yields
// shows in review (IMPROVEMENTS.md §8.1). Run it with
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
	code, out, errOut := runCLI(t, "-C", corpusClone(t, "go-ddd"), "--model", model,
		"--format=json", "analyze", "./internal/...")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	golden(t, "corpus-go-ddd.json", out)
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
	// Every operation of api/openapi.yaml is an echo route, its parameters
	// written :id where the document writes {id}, and every route is
	// documented.
	matched, findings := corpusOpenAPI(t, corpusClone(t, "go-ddd"), model, "./internal/...")
	if matched != 12 || len(findings) > 0 {
		t.Errorf("want 12 operations, all served and none undocumented; got %d and %q",
			matched, findings)
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

// TestADR_0012_TheCoreApprovesItsLibraries pins the allow-list of the core's
// third-party code: the self-check's model approves each library by name,
// for every core package, and go.mod pins each one.
func TestADR_0012_TheCoreApprovesItsLibraries(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("anarch.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := arch.DecodeModel(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(m.Fitness.Libraries, func(l arch.LibraryRule) bool {
		return l.Name == "core-approved-libraries"
	})
	if i < 0 {
		t.Fatal("anarch.json has no libraries rule core-approved-libraries")
	}
	rule := m.Fitness.Libraries[i]
	if rule.Gate != "" && rule.Gate != arch.GateFail {
		t.Errorf("core-approved-libraries gates %q, want fail", rule.Gate)
	}
	const mod = "github.com/StevenACoffman/anarch-lint/internal/"
	for _, core := range []string{"arch", "connascence", "balance", "fitness"} {
		if !arch.MatchesAny(rule.From, mod+core) {
			t.Errorf("core-approved-libraries does not cover %s", core)
		}
	}
	gomod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, pat := range rule.Allow {
		module := strings.TrimSuffix(pat, "/...")
		if !strings.Contains(string(gomod), "\t"+module+" v") {
			t.Errorf("approved library %s is not required by go.mod", module)
		}
	}
}

// TestADR_0015_TheSelfCheckKeepsThreeLayers: the self-check model's layers
// are command, adapter and domain, in that order, and domain holds the
// core and every pure measure.
func TestADR_0015_TheSelfCheckKeepsThreeLayers(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("anarch.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := arch.DecodeModel(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(m.Layers))
	for i, l := range m.Layers {
		names[i] = l.Name
	}
	if want := []string{"command", "adapter", "domain"}; !slices.Equal(names, want) {
		t.Fatalf("layers %q, want %q", names, want)
	}
	const mod = "github.com/StevenACoffman/anarch-lint/internal/"
	for _, core := range []string{"arch", "connascence", "balance", "fitness", "judging"} {
		if !arch.MatchesAny(m.Layers[2].Packages, mod+core) {
			t.Errorf("the domain layer does not hold %s", core)
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

// TestDraft: draft prints a valid model that claims every package of the
// fixture, and fmt would leave it as it is.
func TestDraft(t *testing.T) {
	t.Parallel()
	code, out, errOut := runCLI(t, "-C", connascenceFixture, "draft")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	m, err := arch.DecodeModel(strings.NewReader(out))
	if err != nil {
		t.Fatalf("draft printed no valid model: %v\n%s", err, out)
	}
	if len(m.Subsystems) == 0 || string(m.Format()) != out {
		t.Fatalf("draft printed:\n%s", out)
	}
}

// TestFmt: fmt --check fails on an unformatted model and writes nothing,
// fmt writes it, then --check passes; a YAML model is refused.
func TestFmt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	model := filepath.Join(dir, "anarch.json")
	src := `{"version": 1, "subsystems": [{"name": "a", "packages": ["m/b", "m/a", "m/b"]}]}`
	if err := os.WriteFile(model, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, _ := runCLI(t, "-C", dir, "fmt", "--check")
	if after, _ := os.ReadFile(model); code != 3 || string(after) != src {
		t.Fatalf("fmt --check: exit %d, file %q", code, after)
	}
	if code, _, errOut := runCLI(t, "-C", dir, "fmt"); code != 0 {
		t.Fatalf("fmt: exit %d: %s", code, errOut)
	}
	if code, _, errOut := runCLI(t, "-C", dir, "fmt", "--check"); code != 0 {
		t.Fatalf("fmt --check after fmt: exit %d: %s", code, errOut)
	}
	after, err := os.ReadFile(model)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "\"m/a\",\n        \"m/b\"\n") {
		t.Fatalf("fmt wrote:\n%s", after)
	}
	yaml := filepath.Join(dir, "anarch.yaml")
	if err := os.WriteFile(yaml, []byte("version: 1 # kept\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := runCLI(t, "-C", dir, "--model", yaml, "fmt"); code == 0 ||
		!strings.Contains(errOut, "would lose its comments") {
		t.Fatalf("fmt on YAML: exit %d: %s", code, errOut)
	}
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
	code, out, errOut := runCLI(t, "-C", corpusClone(t, "skillet"), "--model", model,
		"--format=json", "fitness", "./...")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	golden(t, "corpus-skillet.json", out)
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

// gitModule makes an empty directory for a module, and returns it with a
// function that runs git there and one that writes a file.
func gitModule(t *testing.T) (dir string, gitIn func(...string), write func(name, content string)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
	dir = t.TempDir()
	gitIn = func(args ...string) {
		t.Helper()
		//nolint:gosec // G204: the program is the constant "git" and every argument is a literal of this test
		cmd := exec.CommandContext(t.Context(), "git", append([]string{
			"-C", dir, "-c", "user.email=t@example.com",
			"-c", "user.name=t", "-c", "commit.gpgsign=false",
		}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write = func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, gitIn, write
}

// A coChange rule gates check --new-from-rev: a change to the schema without
// its generated code fails, and one with it passes.
func TestCheckCoChange(t *testing.T) {
	t.Parallel()
	dir, gitIn, write := gitModule(t)
	write("go.mod", "module example.com/cc\n\ngo 1.24\n")
	write("a/a.go", "package a\n\n// A is a.\nfunc A() {}\n")
	write("api/schema.txt", "v1\n")
	write("gen/schema.txt", "v1\n")
	write("anarch.json", `{"version": 1, "fitness": {"coChange": [{"name": "regen",
		"reason": "generated code follows the schema", "when": ["api/**"],
		"require": ["gen/**"]}]}}`)
	if code, _, errOut := runCLI(t, "-C", dir, "baseline"); code != 0 {
		t.Fatalf("baseline: exit %d: %s", code, errOut)
	}
	gitIn("init", "-q")
	gitIn("add", ".")
	gitIn("commit", "-q", "-m", "init")

	write("api/schema.txt", "v2\n")
	code, out, _ := runCLI(t, "-C", dir, "check", "--new-from-rev", "HEAD")
	if code != 3 || !strings.Contains(out, "missing-co-change") {
		t.Fatalf("a schema change alone: exit %d, want 3 with missing-co-change\n%s", code, out)
	}
	write("gen/schema.txt", "v2\n")
	if code, out, _ := runCLI(t, "-C", dir, "check", "--new-from-rev", "HEAD"); code != 0 {
		t.Fatalf("a schema change with its code: exit %d, want 0\n%s", code, out)
	}
}

// TestCheckHistoryWindow: with a history window in the model, check reads
// the last commits and fails when a package's hotspot score rises above the
// limit, though the code's design did not change (docs/adr/0017).
func TestCheckHistoryWindow(t *testing.T) {
	t.Parallel()
	dir, gitIn, write := gitModule(t)
	write("go.mod", "module example.com/hw\n\ngo 1.24\n")
	write("a/a.go", "package a\n\n// A is a.\nfunc A() {}\n")
	write("anarch.json", `{"version": 1, "balance": {"gate": true,
		"history": {"commits": 5, "maxHotspot": 1}}}`)
	gitIn("init", "-q")
	gitIn("add", ".")
	gitIn("commit", "-q", "-m", "init")
	if code, _, errOut := runCLI(t, "-C", dir, "baseline"); code != 0 {
		t.Fatalf("baseline: exit %d: %s", code, errOut)
	}
	write("a/a.go", "package a\n\n// A is a.\nfunc A() { _ = 1 }\n")
	gitIn("commit", "-q", "-am", "change a")
	code, out, _ := runCLI(t, "-C", dir, "check")
	if code != 3 || !strings.Contains(out, "hotspot") {
		t.Fatalf("a second commit to a: exit %d, want 3 with hotspot\n%s", code, out)
	}
}

// TestCheckLockedBars: a change that raises a threshold, re-records the
// baseline and changes code fails, naming the loosened bar; the same
// loosening without code passes (IMPROVEMENTS.md §6.1).
func TestCheckLockedBars(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		code bool
		exit int
	}{
		"loosened with code":         {code: true, exit: 3},
		"loosened in its own change": {exit: 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, gitIn, write := gitModule(t)
			write("go.mod", "module example.com/lb\n\ngo 1.24\n")
			write("a/a.go", "package a\n\n// A is a.\nfunc A() {}\n")
			write("anarch.json", `{"version": 1, "thresholds": {"largeFuncLines": 40}}`)
			if code, _, errOut := runCLI(t, "-C", dir, "baseline"); code != 0 {
				t.Fatalf("baseline: exit %d: %s", code, errOut)
			}
			gitIn("init", "-q")
			gitIn("add", ".")
			gitIn("commit", "-q", "-m", "init")

			write("anarch.json", `{"version": 1, "thresholds": {"largeFuncLines": 80}}`)
			if code, _, errOut := runCLI(t, "-C", dir, "baseline"); code != 0 {
				t.Fatalf("baseline: exit %d: %s", code, errOut)
			}
			if tc.code {
				write("a/a.go", "package a\n\n// A is a.\nfunc A() { _ = 1 }\n")
			}
			code, out, _ := runCLI(t, "-C", dir, "check", "--new-from-rev", "HEAD")
			if code != tc.exit {
				t.Fatalf("exit %d, want %d\n%s", code, tc.exit, out)
			}
			loosened := strings.Contains(out, "thresholds.largeFuncLines raised from 40 to 80")
			if loosened != tc.code {
				t.Fatalf("the loosened threshold is named: %v, want %v\n%s", loosened, tc.code, out)
			}
		})
	}
}

// TestCheckBaselineRev: check --baseline-rev measures the revision in a
// worktree under the current model, so an import the change adds is new
// with no baseline file at all, and the revision itself passes.
func TestCheckBaselineRev(t *testing.T) {
	t.Parallel()
	dir, gitIn, write := gitModule(t)
	write("go.mod", "module example.com/br\n\ngo 1.24\n")
	write("a/a.go", "package a\n\n// A is a.\nfunc A() {}\n")
	write("b/b.go", "package b\n\n// B is b.\nfunc B() {}\n")
	write("anarch.json", `{"version": 1, "forbiddenImports": [{"from": ["example.com/br/a"],
		"imports": ["example.com/br/b"], "reason": "a stands alone"}]}`)
	gitIn("init", "-q")
	gitIn("add", ".")
	gitIn("commit", "-q", "-m", "init")
	if code, out, errOut := runCLI(t, "-C", dir, "check", "--baseline-rev", "HEAD"); code != 0 {
		t.Fatalf("the revision itself: exit %d\n%s%s", code, out, errOut)
	}
	write("a/a.go", "package a\n\nimport \"example.com/br/b\"\n\n// A is a.\nfunc A() { b.B() }\n")
	code, out, _ := runCLI(t, "-C", dir, "check", "--baseline-rev", "HEAD")
	if code != 3 || !strings.Contains(out, "forbidden-import") {
		t.Fatalf("an added forbidden import: exit %d, want 3 with forbidden-import\n%s", code, out)
	}
	if code, _, _ := runCLI(t, "-C", dir, "check", "--baseline-rev", "HEAD",
		"--baseline", "x.json"); code != 2 {
		t.Fatalf("two baselines: exit %d, want 2", code)
	}
}

// TestCorpusTodoAPI runs analyze with the VTA call graph on
// MarioCarrion/todo-api-microservice-example, whose REST server wires a
// redis message broker into the service while kafka and rabbitmq brokers
// implement the same interface. VTA finds the redis dependency the model
// forbids, and none on kafka or rabbitmq, which CHA would link because they
// implement the interface too (mmi-ref-plan.md §5).
func TestCorpusTodoAPI(t *testing.T) {
	t.Parallel()
	if *corpus == "" {
		t.Skip("set -corpus to the directory holding a clone of todo-api-microservice-example")
	}
	model, err := filepath.Abs(filepath.Join("testdata", "corpus", "todo-api.json"))
	if err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(t, "-C", corpusClone(t, "todo-api-microservice-example"),
		"--model", model, "--format=json", "analyze", "./...")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	golden(t, "corpus-todo-api.json", out)
	var r struct {
		Findings []struct {
			Rule   string `json:"rule"`
			Object string `json:"object"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	var hidden []string
	for _, f := range r.Findings {
		if f.Rule == "hidden-dependency" {
			hidden = append(hidden, f.Object)
		}
	}
	const in = "github.com/MarioCarrion/todo-api/internal/"
	if !slices.Equal(hidden, []string{in + "redis.Task"}) {
		t.Errorf("want the one wired broker, %sredis.Task; found %v", in, hidden)
	}
	// Three of the five documented operations are registered with
	// fmt.Sprintf of constants; the two routes that serve the document
	// itself are not in it.
	matched, findings := corpusOpenAPI(t,
		corpusClone(t, "todo-api-microservice-example"), model, "./...")
	want := []string{
		"openapi-route-undocumented " + in + "rest: GET /openapi3.json",
		"openapi-route-undocumented " + in + "rest: GET /openapi3.yaml",
	}
	if matched != 5 || !slices.Equal(findings, want) {
		t.Errorf("want 5 operations and the findings %q; got %d and %q", want, matched, findings)
	}
}

// corpusOpenAPI runs fitness on a corpus clone and returns its openapi
// rule's matched operations and the openapi findings, as "rule subject".
func corpusOpenAPI(t *testing.T, dir, model, pattern string) (matched int, findings []string) {
	t.Helper()
	code, out, errOut := runCLI(t, "-C", dir, "--model", model, "--format=json", "fitness", pattern)
	if code != 0 {
		t.Fatalf("fitness: exit %d: %s", code, errOut)
	}
	var r struct {
		Rules []struct {
			Kind    string `json:"kind"`
			Matched int    `json:"matched"`
		} `json:"rules"`
		Findings []struct {
			Rule    string `json:"rule"`
			Subject string `json:"subject"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	for _, rule := range r.Rules {
		if rule.Kind == "openapi" {
			matched = rule.Matched
		}
	}
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Rule, "openapi-") {
			findings = append(findings, f.Rule+" "+f.Subject)
		}
	}
	return matched, findings
}

// corpusPin is the commit a corpus test was checked against. A clone
// elsewhere may find more or fewer violations, so the test fails rather than
// lapse: check out the pinned commit, or recheck the test and move the pin.
func corpusPin(name string) string {
	switch name {
	case "go-ddd":
		return "1028139dacb3e850c4a282a31fb4e7cd40d80d77"
	case "skillet":
		return "e735cde4da04f6ad6c6c9adb984150cf09eed48f"
	case "todo-api-microservice-example":
		return "402ab6b35a250102d8251c776c3587a0ad9e4971"
	default:
		return ""
	}
}

// corpusClone returns the clone of a corpus under -corpus, after checking it
// is at its pinned commit.
func corpusClone(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(*corpus, name)
	//nolint:gosec // G204: the program is the constant "git"; dir is the test's own -corpus flag
	out, err := exec.CommandContext(t.Context(), "git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("read the commit of %s: %v", dir, err)
	}
	if got, want := strings.TrimSpace(string(out)), corpusPin(name); got != want {
		t.Fatalf("%s is at %s, but the test was checked against %s: check that commit out, "+
			"or recheck the test and update corpusPin", dir, got, want)
	}
	return dir
}
