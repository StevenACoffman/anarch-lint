package balance_test

import (
	"strconv"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
)

// TestDocumentedContract: a request to a route no Go code registers, but
// that a fitness.openapi document declares for a serving package, couples
// the requesting subsystem to the serving one at the contract level; with
// no document, the request couples nothing.
func TestDocumentedContract(t *testing.T) {
	t.Parallel()
	request := arch.ConstExpr{
		Type: "string", Value: strconv.Quote("http://tasks.internal/v1/tasks"),
		Context: arch.ContextArgument, Callee: "net/http.Get", Arg: 0,
		Site: arch.Site{File: "web/w.go", Line: 7, Package: "m/web", Func: "m/web.List"},
	}
	doc := arch.APIDocument{
		Path: "api/openapi.yaml", BasePaths: []string{"/v1"},
		Operations: []arch.APIOperation{
			{Method: "GET", Path: "/tasks", Pos: arch.Position{File: "api/openapi.yaml", Line: 9}},
		},
	}
	cases := map[string]struct {
		apis  []arch.APIDocument
		pairs []string
	}{
		"documented": {
			apis: []arch.APIDocument{doc},
			pairs: []string{"web→api contract: m/web.List consumes GET /tasks, " +
				"which api/openapi.yaml documents at line 9 for m/api"},
		},
		"undocumented": {apis: []arch.APIDocument{{Path: "api/openapi.yaml"}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := &arch.Input{
				Program: arch.Program{
					Packages: []arch.Package{
						{Path: "m/api", Name: "api", Module: "m", Files: []string{"api/a.go"}},
						{Path: "m/web", Name: "web", Module: "m", Files: []string{"web/w.go"}},
					},
					Facts: arch.Facts{ConstExprs: []arch.ConstExpr{request}},
				},
				Model: arch.Model{
					Subsystems: []arch.Subsystem{
						{Name: "api", Packages: []string{"m/api"}},
						{Name: "web", Packages: []string{"m/web"}},
					},
					Fitness: arch.Fitness{OpenAPI: []arch.OpenAPIRule{{
						Path: "api/openapi.yaml", ServedBy: []string{"m/api"},
					}}},
				},
				APIs: tc.apis,
			}
			r := balance.Analyze(&balance.Input{Input: in})
			var pairs []string
			for _, p := range r.Pairs {
				for _, e := range p.Evidence {
					pairs = append(pairs, p.From+"→"+p.To+" "+p.Level.String()+": "+e)
				}
			}
			equals(t, tc.pairs, pairs)
		})
	}
}
