package text_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

func render(t *testing.T, in *arch.Input) string {
	t.Helper()
	var b bytes.Buffer
	if err := text.WriteArchitecture(&b, in); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// docs/adr/0006: model text cannot inject HTML or break a table or a
// Mermaid label.
func TestADR_0006_ModelTextIsEscapedWhole(t *testing.T) {
	t.Parallel()
	in := &arch.Input{Model: arch.Model{
		Version:    1,
		Subsystems: []arch.Subsystem{{Name: `a"]-->x`, Packages: []string{"m/a|b"}}},
		Isolations: []arch.Isolation{
			{
				From:      "m/a",
				MayImport: []string{"m/a"},
				Reason:    "<img src=x onerror=alert(1)> | & more",
			},
		},
	}}
	out := render(t, in)
	contains(t, out,
		"&lt;img src=x onerror=alert(1)&gt; \\| &amp; more",
		"`m/a\\|b`",
		`s0["a#34;]--#62;x"]`,
	)
	if strings.Contains(out, "<img") {
		t.Errorf("the reason's HTML survived:\n%s", out)
	}
}

func TestWriteArchitectureListsRulesAndDomainModels(t *testing.T) {
	t.Parallel()
	in := &arch.Input{
		Model: arch.Model{Version: 1, Fitness: arch.Fitness{
			Domains: []arch.DomainRule{{
				RuleOptions: arch.RuleOptions{Name: "auth-model", Reason: "auth speaks its model"},
				Path:        "auth.modelith.yaml", Subsystem: "auth",
			}},
		}},
		Domains: []arch.DomainModel{{
			Path: "auth.modelith.yaml", Shared: true,
			Entities: []arch.DomainEntity{{Name: "Role"}, {Name: "User"}},
			Enums:    []arch.DomainEnum{{Name: "Level", Values: []string{"admin", "member"}}},
		}},
	}
	out := render(t, in)
	contains(t, out,
		"| auth-model | domains | fail | auth speaks its model |",
		"### `auth.modelith.yaml`",
		"Describes subsystem auth. Shared with other models.",
		"Entities: Role, User.",
		"Enums: Level (admin, member).",
	)
	if out != render(t, in) {
		t.Error("two renderings of one input differ")
	}
}

// TestDependencyChartMarksProblems: an edge on a cycle, and one running up
// the subsystem order, are labelled with the problem and drawn in red; an
// edge down the order is neither.
func TestDependencyChartMarksProblems(t *testing.T) {
	t.Parallel()
	imp := func(from, to string) arch.Import { return arch.Import{From: from, To: to} }
	in := arch.Input{
		Program: arch.Program{
			Packages: []arch.Package{
				{Path: "m/a"}, {Path: "m/b"}, {Path: "m/c"}, {Path: "m/d"}, {Path: "m/e"},
			},
			Imports: []arch.Import{
				imp("m/a", "m/b"), imp("m/b", "m/a"), imp("m/c", "m/d"), imp("m/d", "m/a"),
				imp("m/e", "m/c"),
			},
		},
		Model: arch.Model{
			Version: 1,
			Subsystems: []arch.Subsystem{
				{Name: "a", Packages: []string{"m/a"}},
				{Name: "b", Packages: []string{"m/b"}},
				{Name: "c", Packages: []string{"m/c"}},
				{Name: "d", Packages: []string{"m/d"}},
				{Name: "e", Packages: []string{"m/e"}},
			},
			SubsystemOrder: []string{"c", "d", "a", "b", "e"},
		},
	}
	var buf bytes.Buffer
	if err := text.WriteArchitecture(&buf, &in); err != nil {
		t.Fatal(err)
	}
	contains(t, buf.String(),
		"  s0 -->|1, cycle| s1\n",
		"  s1 -->|1, cycle| s0\n",
		"  s2 -->|1| s3\n",
		"  s3 -->|1| s0\n",
		"  s4 -->|1, upward| s2\n",
		"  linkStyle 0,1,4 stroke:#d62728,stroke-width:2px\n",
	)
}
