package text_test

import (
	"bytes"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

func TestWriteExplanation(t *testing.T) {
	t.Parallel()
	at := &arch.Span{File: "a/a.go", Start: 3, End: 3}
	cases := map[string]struct {
		e     arch.ImportExplanation
		wants []string
	}{
		"not imported": {
			e:     arch.ImportExplanation{From: "m/a", To: "m/b", Hypothetical: true},
			wants: []string{"m/a does not import m/b; no declared rule would examine it\n"},
		},
		"not imported, but a rule would forbid it": {
			e: arch.ImportExplanation{
				From: "m/a", To: "m/b", Hypothetical: true,
				Rules: []arch.RuleExplanation{
					{ModelRule: "layers", Outcome: arch.OutcomeForbids, Why: "m/a sits below m/b"},
				},
			},
			wants: []string{
				"m/a does not import m/b; if it did:\n",
				"  forbids  layers  m/a sits below m/b\n",
			},
		},
		"no rule": {
			e:     arch.ImportExplanation{From: "m/a", To: "m/b", Location: at},
			wants: []string{"m/a imports m/b at a/a.go:3; no declared rule examines it\n"},
		},
		"rules": {
			e: arch.ImportExplanation{
				From: "m/a", To: "m/b", Location: at, Inactive: true,
				Rules: []arch.RuleExplanation{
					{ModelRule: "layers", Outcome: arch.OutcomeForbids, Why: "m/a sits below m/b"},
					{
						ModelRule: "interfaces",
						Outcome:   arch.OutcomeAllows,
						Why:       "m/b is an interface",
					},
				},
			},
			wants: []string{
				"m/a imports m/b at a/a.go:3, in a file the build excludes\n",
				"  forbids  layers      m/a sits below m/b\n",
				"  allows   interfaces  m/b is an interface\n",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := text.WriteExplanation(&buf, &tc.e); err != nil {
				t.Fatal(err)
			}
			contains(t, buf.String(), tc.wants...)
		})
	}
}
