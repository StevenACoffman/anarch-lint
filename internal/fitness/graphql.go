package fitness

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// checkGraphQL runs each fitness.graphql rule against its schema: a root
// field with no resolver method in a serving package, and an operation
// document's error against the schema, are each a violation. A rule whose
// serving packages match nothing checks nothing.
//
// Requires: schemas holds one schema per rule, in the rules' order.
func checkGraphQL(l *ledger, rules []arch.GraphQLRule, schemas []arch.GraphQLSchema,
	funcs []arch.Func, packages []string,
) {
	for i := range rules {
		g := &rules[i]
		r := l.addRule("graphql", g.RuleOptions, arch.DefaultRuleName(g.Name, "graphql", i))
		if len(g.ServedBy) > 0 && countMatching(packages, g.ServedBy) == 0 {
			continue
		}
		s := &schemas[i]
		r.result.Matched = len(s.Fields) + s.Operations
		methods := resolverMethods(funcs, g.ServedBy)
		for _, fld := range s.Fields {
			if methods[resolverKey(fld.Root+"Resolver", fld.Name)] {
				continue
			}
			f := arch.NewFinding(arch.FindingID{
				Rule:    arch.RuleFieldUnresolved,
				Subject: fld.Root + "." + fld.Name,
				Object:  r.result.Name,
			}, arch.SeverityError)
			f.Location = arch.SpanAt(fld.Pos)
			f.Message = fmt.Sprintf("%s.%s has no resolver method in a serving package",
				fld.Root, fld.Name)
			f.Fix = fmt.Sprintf("regenerate the resolvers (gqlgen generate) and implement %s "+
				"on the %s resolver, or remove the field from the schema",
				resolverGoName(fld.Name), fld.Root)
			l.violation(r, &f)
		}
		for _, p := range s.Problems {
			f := arch.NewFinding(arch.FindingID{
				Rule:    arch.RuleOperationInvalid,
				Subject: p.Pos.File + ": " + p.Message,
				Object:  r.result.Name,
			}, arch.SeverityError)
			f.Location = arch.SpanAt(p.Pos)
			f.Message = fmt.Sprintf("%s: %s", p.Pos.File, p.Message)
			f.Fix = "change the operation to match the schema, then regenerate its client " +
				"(genqlient)"
			l.violation(r, &f)
		}
	}
}

// resolverMethods returns the methods of the serving packages' types, keyed
// by resolverKey of the receiver type's name and the method's name.
func resolverMethods(funcs []arch.Func, servedBy []string) map[string]bool {
	out := map[string]bool{}
	for i := range funcs {
		fn := &funcs[i]
		if fn.Receiver == "" || !served(servedBy, fn.Package) {
			continue
		}
		recv := fn.Receiver[strings.LastIndex(fn.Receiver, ".")+1:]
		out[resolverKey(recv, fn.Name)] = true
	}
	return out
}

// resolverKey identifies a resolver method by its type and name, in the
// form gqlgen's naming leaves unchanged: case and underscores aside, the
// method is named after the field and its type after the root type, so
// Query.task_list is queryResolver.TaskList.
func resolverKey(typ, name string) string {
	fold := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "")) }
	return fold(typ) + "." + fold(name)
}

// resolverGoName is the method name gqlgen gives a field's resolver, with
// common initialisms aside: task_list is TaskList.
func resolverGoName(field string) string {
	var b strings.Builder
	for part := range strings.SplitSeq(field, "_") {
		if part != "" {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return b.String()
}
