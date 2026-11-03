package fitness

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// protoCheck compares one set of .proto files with the gRPC services its
// serving packages register.
type protoCheck struct {
	ledger *ledger
	r      *rule
	files  *arch.ProtoFiles
	// registered maps each service a serving package registers, by
	// serviceKey of its contract ID, to its first registration.
	registered map[string]arch.Contract
}

// checkProtobuf runs each fitness.protobuf rule against its files and the
// gRPC registrations found in facts. A rule whose serving packages match
// nothing checks nothing.
//
// Requires: files holds one set per rule, in the rules' order.
func checkProtobuf(l *ledger, rules []arch.ProtobufRule, files []arch.ProtoFiles,
	facts *arch.Facts, packages []string,
) {
	if len(rules) == 0 {
		return
	}
	contracts := arch.ContractsOf(facts)
	for i := range rules {
		p := &rules[i]
		c := protoCheck{
			ledger: l,
			r: l.addRule(
				"protobuf",
				p.RuleOptions,
				arch.DefaultRuleName(p.Name, "protobuf", i),
			),
			files:      &files[i],
			registered: map[string]arch.Contract{},
		}
		if len(p.ServedBy) > 0 && countMatching(packages, p.ServedBy) == 0 {
			continue
		}
		for _, ct := range contracts {
			key := serviceKey(ct.ID)
			if _, ok := ct.GRPCService(); ok && ct.Role == arch.ContractProvider &&
				served(p.ServedBy, ct.Site.Package) && c.registered[key].ID == "" {
				c.registered[key] = ct
			}
		}
		c.r.result.Matched = len(c.files.Services) + c.files.Messages
		c.unregistered()
		c.undeclared()
		c.constraints()
	}
}

// serviceKey identifies a gRPC service the way the generated code names
// it: protoc-gen-go-grpc camel-cases a service's name, so tasks_service is
// TasksService, and case and underscores are left out of the comparison.
func serviceKey(id string) string {
	return strings.ToLower(strings.ReplaceAll(id, "_", ""))
}

// unregistered reports each declared service no serving package registers.
func (c *protoCheck) unregistered() {
	for _, svc := range c.files.Services {
		id := svc.ContractID()
		if id != "" {
			if _, ok := c.registered[serviceKey(id)]; ok {
				continue
			}
		}
		f := arch.NewFinding(arch.FindingID{
			Rule: arch.RuleServiceUnregistered, Subject: svc.Name, Object: c.r.result.Name,
		}, arch.SeverityError)
		f.Location = arch.SpanAt(svc.Pos)
		f.Message = fmt.Sprintf("%s declares service %s, and no serving package registers it",
			svc.Pos.File, svc.Name)
		f.Fix = "register the service's server (Register" + serviceName(svc.Name) +
			"Server), or remove it from the file"
		if id == "" {
			f.Message = fmt.Sprintf("%s declares service %s with no go_package, so no Go "+
				"registration can match it", svc.Pos.File, svc.Name)
			f.Fix = "give the file a go_package option naming the generated package"
		}
		c.ledger.violation(c.r, &f)
	}
}

// undeclared reports each service a serving package registers from one of
// the files' Go packages that the files do not declare. A registration from
// another Go package, such as the health service's, is the business of
// other files.
func (c *protoCheck) undeclared() {
	declared, goPackages := map[string]bool{}, map[string]bool{}
	for i := range c.files.Services {
		svc := &c.files.Services[i]
		declared[serviceKey(svc.ContractID())] = true
		goPackages[svc.GoPackage] = true
	}
	for _, key := range sortedKeys(c.registered) {
		ct := c.registered[key]
		svc, _ := ct.GRPCService()
		if declared[key] || !goPackages[svc.GoPackage] {
			continue
		}
		f := arch.NewFinding(arch.FindingID{
			Rule:    arch.RuleServiceUndeclared,
			Subject: ct.Site.Package + ": " + svc.GoPackage + "." + svc.Name,
			Object:  c.r.result.Name,
		}, arch.SeverityError)
		f.Location = &arch.Span{File: ct.Site.File, Start: ct.Site.Line, End: ct.Site.Line}
		f.Message = fmt.Sprintf("%s registers service %s from %s, which the rule's files "+
			"do not declare", ct.Site.Package, svc.Name, svc.GoPackage)
		f.Fix = "declare the service in the files, regenerate the code, or stop registering it"
		c.ledger.violation(c.r, &f)
	}
}

// constraints reports each message whose buf.validate constraints do not
// compile.
func (c *protoCheck) constraints() {
	for _, p := range c.files.Problems {
		f := arch.NewFinding(arch.FindingID{
			Rule: arch.RuleConstraintInvalid, Subject: p.Message, Object: c.r.result.Name,
		}, arch.SeverityError)
		f.Location = arch.SpanAt(p.Pos)
		f.Message = fmt.Sprintf("%s's buf.validate constraints do not compile: %s",
			p.Message, p.Error)
		f.Fix = "correct the constraint: a rule for another field type, or a CEL " +
			"expression that does not type-check, fails every request at run time"
		c.ledger.violation(c.r, &f)
	}
}

// serviceName is the last element of a service's full name.
func serviceName(full string) string {
	return full[strings.LastIndex(full, ".")+1:]
}
