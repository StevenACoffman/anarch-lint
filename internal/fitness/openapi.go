package fitness

import (
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// apiCheck compares one OpenAPI document with the routes its serving
// packages register: the reflexion model of the domain rules, for an API.
type apiCheck struct {
	ledger *ledger
	r      *rule
	doc    *arch.APIDocument
	// routes are the HTTP routes the serving packages register.
	routes []arch.Contract
}

// checkOpenAPI runs each fitness.openapi rule against its document and the
// routes found in facts. A rule
// whose serving packages match nothing, or whose document declares no
// operation, checks nothing.
//
// Requires: docs holds one document per rule, in the rules' order.
func checkOpenAPI(l *ledger, rules []arch.OpenAPIRule, docs []arch.APIDocument,
	facts *arch.Facts, packages []string,
) {
	if len(rules) == 0 {
		return
	}
	contracts := arch.ContractsOf(facts)
	for i := range rules {
		o := &rules[i]
		c := apiCheck{
			ledger: l,
			r:      l.addRule("openapi", o.RuleOptions, arch.DefaultRuleName(o.Name, "openapi", i)),
			doc:    &docs[i],
		}
		for _, ct := range contracts {
			if _, ok := ct.Route(); ok && ct.Role == arch.ContractProvider &&
				served(o.ServedBy, ct.Site.Package) {
				c.routes = append(c.routes, ct)
			}
		}
		if len(o.ServedBy) > 0 && countMatching(packages, o.ServedBy) == 0 {
			continue
		}
		c.r.result.Matched = len(c.doc.Operations)
		c.unserved()
		c.undocumented()
	}
}

// served reports whether pkg is one of a rule's serving packages: any
// package, when the rule names none.
func served(servedBy []string, pkg string) bool {
	return len(servedBy) == 0 || arch.MatchesAny(servedBy, pkg)
}

// unserved reports each operation the document declares that no serving
// package registers, with the routes of its path that other methods
// register as evidence.
func (c *apiCheck) unserved() {
	for _, op := range c.doc.Operations {
		paths := c.doc.PathsOf(&op)
		var others []string
		found := false
		for i := range c.routes {
			rt, _ := c.routes[i].Route()
			if !slices.Contains(paths, rt.Path) {
				continue
			}
			if rt.MethodServes(op.Method) {
				found = true
				break
			}
			site := c.routes[i].Site
			others = append(others, fmt.Sprintf("%s registers %s %s at %s:%d",
				site.Package, rt.Method, rt.Path, site.File, site.Line))
		}
		if found {
			continue
		}
		f := arch.NewFinding(arch.FindingID{
			Rule:    arch.RuleOperationUnserved,
			Subject: c.doc.Path + ": " + op.Method + " " + op.Path,
			Object:  c.r.result.Name,
		}, arch.SeverityError)
		f.Location = arch.SpanAt(op.Pos)
		f.Message = fmt.Sprintf("%s documents %s %s, and no serving package registers it",
			c.doc.Path, op.Method, op.Path)
		f.Evidence = others
		f.Fix = "register a handler for the operation, or remove it from the document; a " +
			"route built at run time, or under a router's prefix, is not found"
		c.ledger.violation(c.r, &f)
	}
}

// undocumented reports each route a serving package registers that the
// document does not declare.
func (c *apiCheck) undocumented() {
	for i := range c.routes {
		site := c.routes[i].Site
		rt, _ := c.routes[i].Route()
		if _, ok := c.doc.Serves(rt); ok {
			continue
		}
		f := arch.NewFinding(arch.FindingID{
			Rule:    arch.RuleRouteUndocumented,
			Subject: site.Package + ": " + rt.Method + " " + rt.Path,
			Object:  c.r.result.Name,
		}, arch.SeverityError)
		f.Location = &arch.Span{File: site.File, Start: site.Line, End: site.Line}
		f.Message = fmt.Sprintf("%s registers %s %s, which %s does not document",
			site.Package, rt.Method, rt.Path, c.doc.Path)
		f.Fix = "document the operation, or stop serving it"
		c.ledger.violation(c.r, &f)
	}
}
