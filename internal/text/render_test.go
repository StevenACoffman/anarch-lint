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
