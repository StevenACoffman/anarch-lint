package connascence

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// grpcClientConstructor matches the constructor protoc-gen-go-grpc
// generates for a client: NewTasksClient.
var grpcClientConstructor = regexp.MustCompile(`\.New\w+Client$`)

// resilienceLibraries returns the import paths, which cover their
// subpackages, of libraries that retry calls or break circuits around them.
func resilienceLibraries() []string {
	return []string{
		"github.com/afex/hystrix-go",
		"github.com/avast/retry-go",
		"github.com/cenkalti/backoff",
		"github.com/eapache/go-resiliency",
		"github.com/failsafe-go/failsafe-go",
		"github.com/grpc-ecosystem/go-grpc-middleware/retry",
		"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry",
		"github.com/hashicorp/go-retryablehttp",
		"github.com/sethvargo/go-retry",
		"github.com/sony/gobreaker",
	}
}

// noRetryDetector reports functions that call another service, through
// net/http or a generated gRPC client, in a package that neither imports a
// retry or circuit-breaker library, nor imports a package under analysis
// that does, nor sets a gRPC service config, where retry policies live: a
// failure of the remote at that moment is a failure of the caller
// (connascence.md §9.7, runtime coupling). The library may be wired
// elsewhere, so it is a candidate, one per calling function.
func noRetryDetector(s *subject) []arch.Connascence {
	covered := s.resilientPackages()
	var out []arch.Connascence
	for _, g := range grouped(s.p.Facts.Calls, func(c arch.Call) (string, bool) {
		return c.Site.Func, c.Site.Func != "" && outbound(c.Callee) && !covered[c.Site.Package]
	}) {
		sites := make([]arch.Site, len(g))
		callees := map[string]bool{}
		for i := range g {
			sites[i], callees[g[i].Callee] = g[i].Site, true
		}
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthTiming,
			detector:   "timing.no-retry",
			subject:    g[0].Site.Func,
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf("%s calls %s, and nothing its package imports retries the "+
				"call or breaks a circuit around it", g[0].Site.Func,
				strings.Join(slices.Sorted(maps.Keys(callees)), ", ")),
			fix: "retry the errors that are safe to retry, with backoff and jitter, and put a " +
				"circuit breaker around a remote that can fail for long",
		}))
	}
	return out
}

// resilientPackages returns the packages that import a resilience library
// or set a gRPC service config, and the packages that import one of those.
func (s *subject) resilientPackages() map[string]bool {
	direct := map[string]bool{}
	for _, imp := range s.p.Imports {
		if slices.ContainsFunc(resilienceLibraries(), func(lib string) bool {
			return imp.To == lib || strings.HasPrefix(imp.To, lib+"/")
		}) {
			direct[imp.From] = true
		}
	}
	for _, c := range s.p.Facts.Calls {
		if c.Callee == "google.golang.org/grpc.WithDefaultServiceConfig" {
			direct[c.Site.Package] = true
		}
	}
	out := maps.Clone(direct)
	for _, imp := range s.p.Imports {
		if direct[imp.To] {
			out[imp.From] = true
		}
	}
	return out
}

// outbound reports a call that sends a request to another service: an
// HTTP request through net/http, or the making of a generated gRPC client.
func outbound(callee string) bool {
	switch callee {
	case "net/http.Get", "net/http.Head", "net/http.Post", "net/http.PostForm",
		"(*net/http.Client).Do", "(*net/http.Client).Get", "(*net/http.Client).Head",
		"(*net/http.Client).Post", "(*net/http.Client).PostForm":
		return true
	}
	return grpcClientConstructor.MatchString(callee)
}
