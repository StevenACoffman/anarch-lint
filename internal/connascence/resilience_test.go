package connascence_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
)

// TestNoRetry: a function that calls another service is a candidate unless
// its package imports a retry or circuit-breaker library, imports a package
// under analysis that does, or sets a gRPC service config.
func TestNoRetry(t *testing.T) {
	t.Parallel()
	call := func(pkg, fn, callee string) arch.Call {
		return arch.Call{Callee: callee, Count: 1, Site: arch.Site{
			File: pkg + "/c.go", Line: 3, Package: "m/" + pkg, Func: "m/" + pkg + "." + fn,
		}}
	}
	p := arch.Program{
		Packages: []arch.Package{
			{Path: "m/bare", Module: "m"},
			{Path: "m/grpcconf", Module: "m"},
			{Path: "m/retry", Module: "m"},
			{Path: "m/wrapped", Module: "m"},
		},
		Imports: []arch.Import{
			{From: "m/retry", To: "github.com/cenkalti/backoff/v4"},
			{From: "m/wrapped", To: "m/retry"},
		},
		Facts: arch.Facts{Calls: []arch.Call{
			call("bare", "Fetch", "net/http.Get"),
			call("bare", "Dial", "m/gen/tasks.NewTasksClient"),
			call("grpcconf", "Dial", "m/gen/tasks.NewTasksClient"),
			call("grpcconf", "Dial", "google.golang.org/grpc.WithDefaultServiceConfig"),
			call("retry", "Fetch", "(*net/http.Client).Do"),
			call("wrapped", "Fetch", "(*net/http.Client).Do"),
		}},
	}
	var got []string
	for _, c := range connascence.Instances(&p, &arch.Model{Version: 1}) {
		if c.Detector == "timing.no-retry" {
			got = append(got, c.Subject)
			equals(t, arch.BasisCandidate, c.Basis)
		}
	}
	equals(t, []string{"m/bare.Dial", "m/bare.Fetch"}, got)
}
