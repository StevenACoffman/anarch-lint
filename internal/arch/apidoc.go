package arch

import (
	"regexp"
	"slices"
	"strings"
)

// inSegmentParam matches a parameter inside a path segment, as in
// "file.{ext}".
var inSegmentParam = regexp.MustCompile(`\{[^}]*\}`)

// APIDocument is an OpenAPI document's operations: what it declares a
// service serves.
type APIDocument struct {
	// Path is the document's path, relative to the analysis root.
	Path string
	// BasePaths are the paths of the document's servers, such as "/api/v1",
	// that prefix every operation's path; empty when no server has one.
	BasePaths  []string
	Operations []APIOperation
}

// APIOperation is one operation of an API document: a method and a path
// template, and where the document declares it.
type APIOperation struct {
	Method string
	Path   string
	Pos    Position
}

// HTTPRoute is an HTTP contract's method, "*" for every method, and its
// canonical path.
type HTTPRoute struct {
	Method string
	Path   string
}

// NormalizeRoute returns an HTTP path in canonical form: each parameter is
// "{}", whether written {id}, {id:[0-9]+}, :id or *rest, and trailing
// slashes are dropped, so a route and the document describing it compare equal.
func NormalizeRoute(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		switch {
		case strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}"),
			strings.HasPrefix(seg, ":") && len(seg) > 1,
			strings.HasPrefix(seg, "*"):
			segments[i] = "{}"
		default:
			segments[i] = inSegmentParam.ReplaceAllString(seg, "{}")
		}
	}
	path = strings.Join(segments, "/")
	if trimmed := strings.TrimRight(path, "/"); trimmed != "" || path == "" {
		return trimmed
	}
	return "/"
}

// Route returns an HTTP contract's route; false for a contract of another
// kind.
func (c *Contract) Route() (HTTPRoute, bool) {
	rest, ok := strings.CutPrefix(c.ID, "http::")
	if !ok {
		return HTTPRoute{}, false
	}
	method, path, ok := strings.Cut(rest, "::")
	return HTTPRoute{Method: method, Path: path}, ok
}

// PathsOf returns the canonical paths an operation of the document is
// served at: its path template as written, and under each base path.
func (d *APIDocument) PathsOf(op *APIOperation) []string {
	out := []string{NormalizeRoute(op.Path)}
	for _, bp := range d.BasePaths {
		out = append(out, NormalizeRoute(bp+op.Path))
	}
	return out
}

// MethodServes reports whether the route's method serves a documented one:
// the same, or a route for every method.
func (r HTTPRoute) MethodServes(documented string) bool {
	return r.Method == "*" || r.Method == documented
}

// Serves returns the operation of the document a route serves, or false
// when the document declares none.
func (d *APIDocument) Serves(r HTTPRoute) (APIOperation, bool) {
	for i := range d.Operations {
		op := &d.Operations[i]
		if r.MethodServes(op.Method) && slices.Contains(d.PathsOf(op), r.Path) {
			return *op, true
		}
	}
	return APIOperation{}, false
}
