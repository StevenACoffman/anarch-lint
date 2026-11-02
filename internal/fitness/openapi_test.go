package fitness_test

import (
	"strconv"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// route is a chi-style registration of path for method in pkg, as the
// loader records its constant argument.
func route(pkg, method, path string, line int) arch.ConstExpr {
	return arch.ConstExpr{
		Type: "string", Value: strconv.Quote(path), Context: arch.ContextArgument,
		Callee: "(*github.com/go-chi/chi/v5.Mux)." + method, Arg: 0,
		Site: arch.Site{File: pkg + "/routes.go", Line: line, Package: pkg},
	}
}

// api is a program whose m/api package serves four routes, one of them
// at every method, and whose m/admin package serves one more.
func api() arch.Program {
	return arch.Program{
		Packages: []arch.Package{{Path: "m/admin"}, {Path: "m/api"}},
		Facts: arch.Facts{ConstExprs: []arch.ConstExpr{
			route("m/admin", "Get", "/admin/stats", 1),
			route("m/api", "Get", "/v1/tasks", 1),
			route("m/api", "Post", "/v1/tasks", 2),
			route("m/api", "Get", "/v1/tasks/{id:[0-9]+}", 3),
			route("m/api", "Handle", "/v1/health", 4),
		}},
	}
}

// tasks documents the tasks API under the base path /v1.
func tasks() arch.APIDocument {
	return arch.APIDocument{
		Path: "api/openapi.yaml", BasePaths: []string{"/v1"},
		Operations: []arch.APIOperation{
			{Method: "GET", Path: "/health", Pos: arch.Position{File: "api/openapi.yaml", Line: 5}},
			{Method: "GET", Path: "/tasks", Pos: arch.Position{File: "api/openapi.yaml", Line: 9}},
			{
				Method: "DELETE",
				Path:   "/tasks/{taskId}",
				Pos:    arch.Position{File: "api/openapi.yaml", Line: 14},
			},
			{
				Method: "GET",
				Path:   "/tasks/{taskId}",
				Pos:    arch.Position{File: "api/openapi.yaml", Line: 17},
			},
		},
	}
}

// TestOpenAPIRule: an operation no serving package registers, and a route
// a serving package registers that the document leaves out, are each a
// violation; a route for every method serves any, the base path prefixes
// every operation, and parameters compare however they are spelled.
func TestOpenAPIRule(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		servedBy []string
		matched  int
		findings []string
		evidence []string
	}{
		"the serving package": {
			servedBy: []string{"m/api"},
			matched:  4,
			findings: []string{
				"openapi-operation-unserved api/openapi.yaml: DELETE /tasks/{taskId} tasks-api",
				"openapi-route-undocumented m/api: POST /v1/tasks tasks-api",
			},
			evidence: []string{"m/api registers GET /v1/tasks/{} at m/api/routes.go:3"},
		},
		"every package": {
			matched: 4,
			findings: []string{
				"openapi-operation-unserved api/openapi.yaml: DELETE /tasks/{taskId} tasks-api",
				"openapi-route-undocumented m/admin: GET /admin/stats tasks-api",
				"openapi-route-undocumented m/api: POST /v1/tasks tasks-api",
			},
			evidence: []string{"m/api registers GET /v1/tasks/{} at m/api/routes.go:3"},
		},
		"serving packages that match nothing": {
			servedBy: []string{"m/web"},
			findings: []string{"empty-selector tasks-api "},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := arch.Model{Fitness: arch.Fitness{OpenAPI: []arch.OpenAPIRule{{
				RuleOptions: arch.RuleOptions{Name: "tasks-api", Reason: "r"},
				Path:        "api/openapi.yaml", ServedBy: tc.servedBy,
			}}}}
			r := fitness.Analyze(&arch.Input{
				Program: api(), Model: m, APIs: []arch.APIDocument{tasks()},
			})
			equals(t, tc.findings, rules(&r))
			equals(t, tc.matched, r.Rules[0].Matched)
			if len(tc.evidence) > 0 {
				equals(t, tc.evidence, r.Findings[0].Evidence)
				equals(
					t,
					&arch.Span{File: "api/openapi.yaml", Start: 14, End: 14},
					r.Findings[0].Location,
				)
			}
		})
	}
}
