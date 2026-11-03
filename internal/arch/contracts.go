package arch

import (
	"cmp"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The kinds of a contract.
const (
	contractHTTP = "http"
	contractGRPC = "grpc"
)

// The roles of a contract: a provider, which ContractProvider names for the
// packages that read contracts, or a consumer.
const (
	ContractProvider = "provider"
	contractConsumer = "consumer"
)

var (
	// grpcRegister and grpcClient match the functions protoc-gen-go-grpc
	// generates: RegisterTasksServer and NewTasksClient.
	grpcRegister = regexp.MustCompile(`^(.+)\.Register(\w+)Server$`)
	grpcClient   = regexp.MustCompile(`^(.+)\.New(\w+)Client$`)
)

// Contract is one end of an integration through a published contract: a
// route an HTTP handler serves or a request calls, or a gRPC service a
// server registers or a client is made for (IMPROVEMENTS.md §5.1). Its ID is
// canonical, such as "http::GET::/tasks/{}" or "grpc::example.com/pb.Tasks",
// so a provider and its consumers match by it.
type Contract struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Role string `json:"role"`
	Site Site   `json:"site"`
}

// ContractPair is a consumer and a provider of one contract in different
// packages: coupling through the contract, which needs no import.
type ContractPair struct {
	ID       string `json:"id"`
	Consumer Site   `json:"consumer"`
	Provider Site   `json:"provider"`
}

// call is the constant arguments of one call, by position.
type call struct {
	callee string
	site   Site
	args   map[int]string
}

// ContractsOf finds the contracts a program provides and consumes, from the
// facts alone: gRPC servers and clients by the generated functions they
// call, and HTTP routes by the constant strings passed to handler
// registrations and requests, sorted by ID, role and site. Requires the
// program's facts. A route built at run time is not found.
func ContractsOf(f *Facts) []Contract {
	var out []Contract
	for _, c := range f.Calls {
		if m := grpcRegister.FindStringSubmatch(c.Callee); m != nil {
			out = append(out, grpcContract(m, ContractProvider, c.Site))
		}
		if m := grpcClient.FindStringSubmatch(c.Callee); m != nil {
			out = append(out, grpcContract(m, contractConsumer, c.Site))
		}
	}
	out = append(out, httpContracts(f.ConstExprs)...)
	slices.SortFunc(out, func(a, b Contract) int {
		return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Role, b.Role),
			CompareSites(a.Site, b.Site))
	})
	return slices.CompactFunc(out, func(a, b Contract) bool {
		return a.ID == b.ID && a.Role == b.Role && a.Site.Package == b.Site.Package
	})
}

// grpcContract is the contract of a generated gRPC function's match: the
// service, named by the generated package and the service's name.
func grpcContract(m []string, role string, site Site) Contract {
	return Contract{ID: grpcID(m[1], m[2]), Kind: contractGRPC, Role: role, Site: site}
}

// grpcID is the ID of the gRPC service a generated Go package holds.
func grpcID(goPackage, service string) string {
	return "grpc::" + goPackage + "." + service
}

// httpContracts finds HTTP routes in constant call arguments: a handler
// registration, a router method named after an HTTP method or Handle, with a
// constant path as its first argument, provides one; a request built with a
// constant URL consumes one.
func httpContracts(exprs []ConstExpr) []Contract {
	calls := map[string]*call{}
	var order []string
	for i := range exprs {
		e := &exprs[i]
		if e.Context != ContextArgument || e.Type != "string" {
			continue
		}
		key := e.Site.File + ":" + strconv.Itoa(e.Site.Line) + ":" + e.Callee
		if calls[key] == nil {
			calls[key] = &call{callee: e.Callee, site: e.Site, args: map[int]string{}}
			order = append(order, key)
		}
		if v, err := strconv.Unquote(e.Value); err == nil {
			calls[key].args[e.Arg] = v
		}
	}
	var out []Contract
	for _, key := range order {
		if c, ok := calls[key].contract(); ok {
			out = append(out, c)
		}
	}
	return out
}

// contract returns the HTTP contract a call provides or consumes, if any.
func (c *call) contract() (Contract, bool) {
	name := c.callee[strings.LastIndexAny(c.callee, ".)")+1:]
	if method, ok := consumerMethod(c.callee, c.args); ok {
		if path, ok := urlPath(c.args[consumerURLArg(c.callee)]); ok {
			return httpContract(method, path, contractConsumer, c.site), true
		}
		return Contract{}, false
	}
	path := c.args[0]
	if !strings.HasPrefix(path, "/") && !strings.Contains(path, " /") {
		return Contract{}, false
	}
	switch upper := strings.ToUpper(name); {
	case name == "Handle" || name == "HandleFunc" || name == "Path":
		// Go 1.22 patterns name the method: "GET /tasks/{id}".
		if method, rest, ok := strings.Cut(path, " "); ok {
			return httpContract(method, rest, ContractProvider, c.site), true
		}
		return httpContract("*", path, ContractProvider, c.site), true
	case slices.Contains(httpMethods(), upper):
		return httpContract(upper, path, ContractProvider, c.site), true
	}
	return Contract{}, false
}

// consumerMethod returns the HTTP method a call to the net/http client
// sends, and false when the call is no request.
func consumerMethod(callee string, args map[int]string) (string, bool) {
	switch callee {
	case "net/http.Get", "(*net/http.Client).Get":
		return "GET", true
	case "net/http.Head", "(*net/http.Client).Head":
		return "HEAD", true
	case "net/http.Post",
		"(*net/http.Client).Post",
		"net/http.PostForm",
		"(*net/http.Client).PostForm":
		return "POST", true
	case "net/http.NewRequest", "net/http.NewRequestWithContext":
		method := args[0]
		if callee == "net/http.NewRequestWithContext" {
			method = args[1]
		}
		return cmp.Or(strings.ToUpper(method), "*"), true
	}
	return "", false
}

// consumerURLArg is the position of a request's URL argument.
func consumerURLArg(callee string) int {
	switch callee {
	case "net/http.NewRequest":
		return 1
	case "net/http.NewRequestWithContext":
		return 2
	default:
		return 0
	}
}

// urlPath returns the path of a URL, or of a path alone.
func urlPath(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(u.Path, "/") {
		return "", false
	}
	return u.Path, true
}

// httpContract is an HTTP contract, its path canonical (NormalizeRoute), so
// /tasks/:id and /tasks/{id}/ match.
func httpContract(method, path, role string, site Site) Contract {
	path = NormalizeRoute(path)
	return Contract{
		ID: "http::" + method + "::" + path, Kind: contractHTTP, Role: role, Site: site,
	}
}

// httpMethods lists the methods a router method may be named after.
func httpMethods() []string {
	return []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
}

// MatchContracts pairs each consumer with each provider of its contract in
// another package, an HTTP consumer of any method matching a provider of
// any; it returns the pairs and the contracts with no counterpart, both
// sorted.
func MatchContracts(cs []Contract) (pairs []ContractPair, orphans []Contract) {
	providers := map[string][]Contract{}
	for _, c := range cs {
		if c.Role == ContractProvider {
			providers[routeKey(c.ID)] = append(providers[routeKey(c.ID)], c)
		}
	}
	consumed := map[string]bool{}
	for _, c := range cs {
		if c.Role != contractConsumer {
			continue
		}
		matched, agreed := matchConsumer(&c, providers[routeKey(c.ID)], consumed)
		if !agreed {
			orphans = append(orphans, c)
		}
		pairs = append(pairs, matched...)
	}
	for _, c := range cs {
		if c.Role == ContractProvider && !consumed[c.ID] {
			orphans = append(orphans, c)
		}
	}
	return pairs, orphans
}

// matchConsumer returns a consumer's pairs with the providers of its route
// whose method agrees, marking each such provider consumed, and whether any
// agrees; a provider in the consumer's own package agrees but makes no pair.
func matchConsumer(
	c *Contract, providers []Contract, consumed map[string]bool,
) (pairs []ContractPair, agreed bool) {
	for _, p := range providers {
		if !methodsMatch(c.ID, p.ID) {
			continue
		}
		agreed, consumed[p.ID] = true, true
		if p.Site.Package != c.Site.Package {
			pairs = append(pairs, ContractPair{ID: p.ID, Consumer: c.Site, Provider: p.Site})
		}
	}
	return pairs, agreed
}

// routeKey is a contract's ID without its HTTP method.
func routeKey(id string) string {
	if rest, ok := strings.CutPrefix(id, "http::"); ok {
		_, path, _ := strings.Cut(rest, "::")
		return "http::" + path
	}
	return id
}

// methodsMatch reports whether two contracts of one route agree on the
// method: the same, or either unknown.
func methodsMatch(a, b string) bool {
	method := func(id string) string {
		parts := strings.SplitN(id, "::", 3)
		if len(parts) < 3 {
			return "*"
		}
		return parts[1]
	}
	ma, mb := method(a), method(b)
	return ma == mb || ma == "*" || mb == "*"
}
