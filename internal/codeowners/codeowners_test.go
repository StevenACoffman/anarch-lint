package codeowners_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/codeowners"
)

const file = `# owners
*                 @org/all
/billing/         @org/money
docs/*.md         @org/docs
**/generated/**   @org/gen
web               @org/web @org/a11y
/billing/legacy/  # unowned
`

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

func TestOwners(t *testing.T) {
	t.Parallel()
	rules, err := codeowners.Parse(strings.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		"main.go":                 {"@org/all"},
		"billing/invoice.go":      {"@org/money"},
		"billing/tax/rate.go":     {"@org/money"},
		"billing/legacy/old.go":   nil, // a later rule without owners wins
		"docs/guide.md":           {"@org/docs"},
		"docs/deep/guide.md":      {"@org/all"}, // * does not cross a slash
		"x/generated/y/z.go":      {"@org/gen"},
		"web/admin.go":            {"@org/a11y", "@org/web"},
		"internal/web/handler.go": {"@org/a11y", "@org/web"}, // unanchored: any depth
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			equals(t, want, rules.Owners(path))
		})
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()
	rules, err := codeowners.Parse(strings.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	p := &arch.Program{Packages: []arch.Package{
		{Path: "m/billing", Files: []string{"billing/a.go", "billing/legacy/b.go"}},
		{Path: "m/web", Files: []string{"web/w.go"}},
		{Path: "m/legacy", Files: []string{"billing/legacy/c.go"}},
	}}
	equals(t, map[string][]string{
		"m/billing": {"@org/money"},
		"m/web":     {"@org/a11y", "@org/web"},
	}, codeowners.Resolve(rules, p))
}

func TestParseRejectsUnsupportedPattern(t *testing.T) {
	t.Parallel()
	_, err := codeowners.Parse(strings.NewReader("ok @a\n[ab].go @b\n"))
	equals(t, arch.EINVALID, arch.ErrorCode(err))
	equals(t, "CODEOWNERS line 2: unexpected character '[' at position 1", arch.ErrorMessage(err))
}
