package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/indomain/pkg/provenance"
)

const ruleSet = `fitness:
  calls:
    - name: no-sleep
      from: ["example.com/r/..."]
      callees: ["time.Sleep"]
      optional: true
      reason: wait on a signal
`

// vendored stamps body with a provenance header, as modelith deps import
// writes one.
func vendored(body string) string {
	return string(provenance.Stamp([]byte(body), &provenance.Header{
		Vendored: provenance.Banner, Fetch: "git", Origin: "https://github.com/acme/rules",
		Path: "org.anarch.yaml", Ref: "main", Commit: strings.Repeat("a", 40),
		Imported: "2026-11-10", Digest: provenance.Digest([]byte(body)),
	}))
}

// withoutLine returns src without its lines that begin with prefix.
func withoutLine(src, prefix string) string {
	var kept []string
	for _, line := range strings.Split(src, "\n") {
		if !strings.HasPrefix(line, prefix) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// docs/adr/0018: a model includes a rule set's rules; a vendored copy is
// accepted while it matches the digest its header records, and refused once
// edited or with a header that does not parse.
func TestADR_0018_RuleSetsAreVendoredWholeAndVerifiedOffline(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		set  string
		code arch.ProblemCode // "" when the model loads
	}{
		"its own":         {set: ruleSet},
		"vendored intact": {set: vendored(ruleSet)},
		"vendored, then edited": {
			set:  strings.Replace(vendored(ruleSet), "wait on a signal", "sleep if you must", 1),
			code: arch.ProblemProvenance,
		},
		"a header missing its origin": {
			set:  withoutLine(vendored(ruleSet), "# modelith-origin:"),
			code: arch.ProblemProvenance,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := repository(t, "docs/r.modelith.yaml")
			if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "rules", "org.anarch.yaml"),
				[]byte(tc.set), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "anarch.json"), []byte(`{"version": 1,
				"ruleSets": ["rules/org.anarch.yaml"]}`), 0o600); err != nil {
				t.Fatal(err)
			}
			in, err := load(t, dir)
			if tc.code != "" {
				if !strings.Contains(arch.ErrorMessage(err), "model: "+string(tc.code)+": ") {
					t.Fatalf("want %s, got %v", tc.code, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			equals(t, 1, len(in.Model.Fitness.Calls))
			equals(t, "no-sleep", in.Model.Fitness.Calls[0].Name)
		})
	}
}
