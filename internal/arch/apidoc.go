package arch

import (
	"regexp"
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
// "{}", whether written {id}, {id:[0-9]+}, :id or *rest, and a trailing
// slash is dropped, so a route and the document describing it compare equal.
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
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	return path
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
