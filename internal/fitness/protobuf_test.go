package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// TestProtobufRule: a declared service no serving package registers, a
// registered service from the files' Go package that they do not declare,
// and a message whose constraints do not compile are each a violation; a
// camel-cased registration matches its snake-cased service, and a
// registration from another Go package, such as the health service's, is
// left alone.
func TestProtobufRule(t *testing.T) {
	t.Parallel()
	register := func(pkg, callee string, line int) arch.Call {
		return arch.Call{
			Callee: callee, Count: 1,
			Site: arch.Site{File: pkg + "/server.go", Line: line, Package: pkg},
		}
	}
	program := arch.Program{
		Packages: []arch.Package{{Path: "m/server"}, {Path: "m/admin"}},
		Facts: arch.Facts{Calls: []arch.Call{
			register("m/server", "m/gen/tasks.RegisterTasksServer", 1),
			register("m/server", "m/gen/tasks.RegisterArchiveServer", 2),
			register(
				"m/server",
				"google.golang.org/grpc/health/grpc_health_v1.RegisterHealthServer",
				3,
			),
			register("m/admin", "m/gen/tasks.RegisterTaskAdminServer", 1),
		}},
	}
	at := func(line int) arch.Position { return arch.Position{File: "proto/tasks.proto", Line: line} }
	files := arch.ProtoFiles{
		Services: []arch.ProtoService{
			{Name: "tasks.v1.Tasks", GoPackage: "m/gen/tasks", Pos: at(5)},
			{Name: "tasks.v1.task_admin", GoPackage: "m/gen/tasks", Pos: at(9)},
		},
		Messages: 3,
		Problems: []arch.ProtoProblem{{
			Message: "tasks.v1.BadRule", Error: "expected rule int32, got string", Pos: at(20),
		}},
	}
	cases := map[string]struct {
		servedBy []string
		matched  int
		findings []string
	}{
		"the serving package": {
			servedBy: []string{"m/server"},
			matched:  5,
			findings: []string{
				"proto-constraint-invalid tasks.v1.BadRule tasks-proto",
				"proto-service-undeclared m/server: m/gen/tasks.Archive tasks-proto",
				"proto-service-unregistered tasks.v1.task_admin tasks-proto",
			},
		},
		"every package": {
			matched: 5,
			findings: []string{
				"proto-constraint-invalid tasks.v1.BadRule tasks-proto",
				"proto-service-undeclared m/server: m/gen/tasks.Archive tasks-proto",
			},
		},
		"serving packages that match nothing": {
			servedBy: []string{"m/web"},
			findings: []string{"empty-selector tasks-proto "},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := arch.Model{Fitness: arch.Fitness{Protobuf: []arch.ProtobufRule{{
				RuleOptions: arch.RuleOptions{Name: "tasks-proto", Reason: "r"},
				Files:       []string{"proto/**/*.proto"}, ServedBy: tc.servedBy,
			}}}}
			r := fitness.Analyze(&arch.Input{
				Program: program, Model: m, Protos: []arch.ProtoFiles{files},
			})
			equals(t, tc.findings, rules(&r))
			equals(t, tc.matched, r.Rules[0].Matched)
		})
	}
}

// TestProtobufRuleWithoutGoPackage: a service in a file with no go_package
// is unregistered, since no Go registration can name it, and the finding
// says why.
func TestProtobufRuleWithoutGoPackage(t *testing.T) {
	t.Parallel()
	m := arch.Model{Fitness: arch.Fitness{Protobuf: []arch.ProtobufRule{{
		RuleOptions: arch.RuleOptions{Name: "p", Reason: "r"}, Files: []string{"*.proto"},
	}}}}
	r := fitness.Analyze(&arch.Input{
		Program: arch.Program{Packages: []arch.Package{{Path: "m/server"}}},
		Model:   m,
		Protos: []arch.ProtoFiles{{Services: []arch.ProtoService{
			{Name: "legacy.Store", Pos: arch.Position{File: "store.proto", Line: 3}},
		}}},
	})
	equals(t, []string{"proto-service-unregistered legacy.Store p"}, rules(&r))
	equals(t, "give the file a go_package option naming the generated package", r.Findings[0].Fix)
}
