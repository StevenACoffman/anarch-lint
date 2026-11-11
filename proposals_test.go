package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const proposalsFixture = "testdata/cx/proposals.json"

// proposals reports each proposed rule's outcome on the code, and the
// violations of the ones that fail.
func TestProposals(t *testing.T) {
	t.Parallel()
	code, out, errOut := runCLI(t, "-C", connascenceFixture, "proposals", "report",
		proposalsFixture)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{
		"from codetospec",
		"role-writers    billing.Invoice.Role writers    fields  1        0           passed",
		"amount-writers  billing.Invoice.Amount writers  fields  1        1           failed",
		"FINDINGS (1 errors, 0 warnings)",
		"example.com/cx/billing.MustBill writes example.com/cx/billing.Invoice.Amount",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// A proposal whose name the model declares, accepted or rejected (gate
// off), is reported declared and not analyzed.
func TestProposalsDeclared(t *testing.T) {
	t.Parallel()
	model := filepath.Join(t.TempDir(), "anarch.json")
	src := `{"version": 1, "fitness": {"fields": [{"name": "billing.Invoice.Amount writers",
		"gate": "off", "fields": ["example.com/cx/billing.Invoice.Amount"], "writes": true,
		"only": true, "from": ["example.com/cx/web"]}]}}`
	if err := os.WriteFile(model, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(
		t,
		"-C",
		connascenceFixture,
		"--model",
		model,
		"proposals",
		"report",
		proposalsFixture,
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{
		"amount-writers  billing.Invoice.Amount writers  fields  -        -           declared",
		"FINDINGS (0 errors, 0 warnings)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestProposalsErrors(t *testing.T) {
	t.Parallel()
	bad := filepath.Join(t.TempDir(), "proposals.json")
	if err := os.WriteFile(bad, []byte(`{"schema": "other"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		args []string
		code int
		want string
	}{
		"no file":        {[]string{"proposals", "report"}, 2, "a proposals file is required"},
		"another schema": {[]string{"proposals", "report", bad}, 1, "not supported"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			code, _, errOut := runCLI(t, tc.args...)
			if code != tc.code || !strings.Contains(errOut, tc.want) {
				t.Errorf("exit %d, stderr %q; want exit %d containing %q",
					code, errOut, tc.code, tc.want)
			}
		})
	}
}

// accept adds a proposal's rule to the model in its canonical form, starts
// a missing model, says when the first fitness rule makes fitness gate,
// and refuses a rule the model declares, writing nothing.
func TestAccept(t *testing.T) {
	t.Parallel()
	model := filepath.Join(t.TempDir(), "anarch.json")
	code, out, errOut := runCLI(
		t,
		"--model",
		model,
		"proposals",
		"accept",
		proposalsFixture,
		"role-writers",
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "1 rule(s) added") ||
		!strings.Contains(errOut, "had no fitness rule") {
		t.Errorf("stdout %q, stderr %q", out, errOut)
	}
	written, err := os.ReadFile(model)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), `"name": "billing.Invoice.Role writers"`) {
		t.Errorf("model lacks the rule:\n%s", written)
	}
	if code, out, _ := runCLI(t, "--model", model, "fmt", "--check"); code != 0 {
		t.Errorf("the written model is not in canonical form: %s", out)
	}

	code, _, errOut = runCLI(t, "--model", model, "proposals", "accept", proposalsFixture,
		"amount-writers", "role-writers")
	if code != 1 || !strings.Contains(errOut, "duplicate-name") {
		t.Errorf("accepting a declared rule: exit %d, stderr %q", code, errOut)
	}
	again, err := os.ReadFile(model)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, written) {
		t.Error("a refused accept should write nothing, not even the other proposal")
	}
}

func TestAcceptErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, tc := range map[string]struct {
		args []string
		code int
		want string
	}{
		"no id": {[]string{"proposals", "accept", proposalsFixture}, 2, "an id are required"},
		"unknown id": {
			[]string{"--model", filepath.Join(dir, "a.json"), "proposals", "accept", proposalsFixture, "nope"},
			1, `no proposal has id "nope"`,
		},
		"a YAML model": {
			[]string{"--model", filepath.Join(dir, "a.yaml"), "proposals", "accept", proposalsFixture, "role-writers"},
			1, "only a JSON model",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			code, _, errOut := runCLI(t, tc.args...)
			if code != tc.code || !strings.Contains(errOut, tc.want) {
				t.Errorf("exit %d, stderr %q; want exit %d containing %q",
					code, errOut, tc.code, tc.want)
			}
		})
	}
}
