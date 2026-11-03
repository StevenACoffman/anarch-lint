package arch_test

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestContracts: gRPC servers and clients by their generated functions, and
// HTTP routes by the constants given to handler registrations, router
// methods and requests, canonical so that a parameter's spelling does not
// matter; a consumer matches a provider in another package, and what has no
// counterpart is an orphan.
func TestContracts(t *testing.T) {
	t.Parallel()
	site := func(pkg string, line int) arch.Site {
		return arch.Site{File: "f.go", Line: line, Package: pkg}
	}
	arg := func(callee string, line, i int, value string) arch.ConstExpr {
		return arch.ConstExpr{
			Type: "string", Value: `"` + value + `"`, Context: arch.ContextArgument,
			Callee: callee, Arg: i, Site: site("m/x", line),
		}
	}
	f := arch.Facts{
		Calls: []arch.Call{
			{Callee: "m/pb.RegisterTasksServer", Site: site("m/server", 1)},
			{Callee: "m/pb.NewTasksClient", Site: site("m/client", 2)},
		},
		ConstExprs: []arch.ConstExpr{
			arg("(*net/http.ServeMux).HandleFunc", 3, 0, "GET /tasks/{id}"),
			arg("(github.com/go-chi/chi/v5.Router).Post", 4, 0, "/tasks/"),
			arg("net/http.NewRequest", 5, 0, "GET"),
			arg("net/http.NewRequest", 5, 1, "http://api.internal/tasks/:id"),
			arg("net/http.Get", 6, 0, "https://example.com/health"),
			arg("fmt.Println", 7, 0, "/not/a/route"),
		},
	}
	f.ConstExprs[5].Site.Package = "m/y"
	cs := arch.ContractsOf(&f)
	var ids []string
	for _, c := range cs {
		ids = append(ids, c.Role+" "+c.ID)
	}
	equals(t, []string{
		"consumer grpc::m/pb.Tasks",
		"provider grpc::m/pb.Tasks",
		"consumer http::GET::/health",
		"consumer http::GET::/tasks/{}",
		"provider http::GET::/tasks/{}",
		"provider http::POST::/tasks",
	}, ids)

	pairs, orphans := arch.MatchContracts(cs)
	var paired []string
	for _, p := range pairs {
		paired = append(paired, p.ID+" "+p.Consumer.Package+" -> "+p.Provider.Package)
	}
	equals(t, []string{"grpc::m/pb.Tasks m/client -> m/server"}, paired)
	var lonely []string
	for _, o := range orphans {
		lonely = append(lonely, o.ID)
	}
	equals(t, []string{"http::GET::/health", "http::POST::/tasks"}, lonely)
}

// TestNormalizeRoute: a parameter is "{}" however a router or a document
// writes it, a regular expression with braces of its own included, and a
// trailing slash is dropped; a colon inside a segment is no parameter.
func TestNormalizeRoute(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"/tasks/{taskId}":                  "/tasks/{}",
		"/tasks/{id:[0-9a-f]{8}-[0-9]{4}}": "/tasks/{}",
		"/tasks/:id/":                      "/tasks/{}",
		"/static/*filepath":                "/static/{}",
		"/files/report.{ext}":              "/files/report.{}",
		"/v1/tasks:search":                 "/v1/tasks:search",
		"/":                                "/",
	} {
		equals(t, want, arch.NormalizeRoute(in))
	}
}

// TestNormalizeRouteIsIdempotent: a canonical route is its own canonical
// form, so normalizing twice never changes what matches.
func TestNormalizeRouteIsIdempotent(t *testing.T) {
	t.Parallel()
	segment := rapid.SampledFrom([]string{
		"", "tasks", "{id}", "{id:[0-9]{2}}", ":id", "*rest", "file.{ext}", "v1:search", "{}",
	})
	rapid.Check(t, func(t *rapid.T) {
		path := "/" + strings.Join(rapid.SliceOfN(segment, 0, 6).Draw(t, "segments"), "/")
		once := arch.NormalizeRoute(path)
		if twice := arch.NormalizeRoute(once); twice != once {
			t.Fatalf(
				"NormalizeRoute(%q) = %q, but NormalizeRoute(%q) = %q",
				path,
				once,
				once,
				twice,
			)
		}
	})
}

// TestServes: a route serves an operation of the document on the same
// canonical path, as written or under a base path, when its method is the
// operation's or every method.
func TestServes(t *testing.T) {
	t.Parallel()
	doc := arch.APIDocument{
		BasePaths: []string{"/v1"},
		Operations: []arch.APIOperation{
			{Method: "GET", Path: "/tasks/{taskId}", Pos: arch.Position{Line: 4}},
		},
	}
	cases := map[string]struct {
		route arch.HTTPRoute
		line  int // 0 when no operation is served
	}{
		"the path as written": {route: arch.HTTPRoute{Method: "GET", Path: "/tasks/{}"}, line: 4},
		"under the base path": {
			route: arch.HTTPRoute{Method: "GET", Path: "/v1/tasks/{}"},
			line:  4,
		},
		"every method":         {route: arch.HTTPRoute{Method: "*", Path: "/v1/tasks/{}"}, line: 4},
		"another method":       {route: arch.HTTPRoute{Method: "POST", Path: "/v1/tasks/{}"}},
		"another path":         {route: arch.HTTPRoute{Method: "GET", Path: "/v2/tasks/{}"}},
		"a literal for a part": {route: arch.HTTPRoute{Method: "GET", Path: "/tasks/42"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			op, ok := doc.Serves(tc.route)
			equals(t, tc.line != 0, ok)
			equals(t, tc.line, op.Pos.Line)
		})
	}
}
