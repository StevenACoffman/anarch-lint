package fitness_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// TestGraphQLRule: a root field with no resolver method in a serving
// package, and an operation document's problem, are each a violation;
// resolver names compare as gqlgen spells them, case and underscores
// aside.
func TestGraphQLRule(t *testing.T) {
	t.Parallel()
	program := arch.Program{
		Packages: []arch.Package{{Path: "m/graph"}, {Path: "m/other"}},
		Funcs: []arch.Func{
			{
				ID:       "m/graph.queryResolver.Tasks",
				Package:  "m/graph",
				Receiver: "m/graph.queryResolver",
				Name:     "Tasks",
			},
			{
				ID: "m/graph.queryResolver.TaskSearch", Package: "m/graph",
				Receiver: "m/graph.queryResolver", Name: "TaskSearch",
			},
			{
				ID:       "m/other.mutationResolver.CreateTask",
				Package:  "m/other",
				Receiver: "m/other.mutationResolver",
				Name:     "CreateTask",
			},
			{ID: "m/graph.Task", Package: "m/graph", Name: "Task"},
		},
	}
	schema := arch.GraphQLSchema{
		Fields: []arch.GraphQLField{
			{
				Root: "Mutation",
				Name: "createTask",
				Pos:  arch.Position{File: "schema.graphqls", Line: 12},
			},
			{Root: "Query", Name: "task", Pos: arch.Position{File: "schema.graphqls", Line: 8}},
			{
				Root: "Query",
				Name: "task_search",
				Pos:  arch.Position{File: "search.graphqls", Line: 2},
			},
			{Root: "Query", Name: "tasks", Pos: arch.Position{File: "schema.graphqls", Line: 7}},
		},
		Operations: 1,
		Problems: []arch.GraphQLProblem{{
			Message: `Cannot query field "owner" on type "Task".`,
			Pos:     arch.Position{File: "ops/task.graphql", Line: 4},
		}},
	}
	cases := map[string]struct {
		servedBy []string
		matched  int
		findings []string
	}{
		"the serving package": {
			servedBy: []string{"m/graph"},
			matched:  5,
			findings: []string{
				"graphql-field-unresolved Mutation.createTask tasks-graph",
				"graphql-field-unresolved Query.task tasks-graph",
				"graphql-operation-invalid ops/task.graphql: " +
					`Cannot query field "owner" on type "Task". tasks-graph`,
			},
		},
		"every package": {
			matched: 5,
			findings: []string{
				"graphql-field-unresolved Query.task tasks-graph",
				"graphql-operation-invalid ops/task.graphql: " +
					`Cannot query field "owner" on type "Task". tasks-graph`,
			},
		},
		"serving packages that match nothing": {
			servedBy: []string{"m/web"},
			findings: []string{"empty-selector tasks-graph "},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := arch.Model{Fitness: arch.Fitness{GraphQL: []arch.GraphQLRule{{
				RuleOptions: arch.RuleOptions{Name: "tasks-graph", Reason: "r"},
				Schema:      []string{"*.graphqls"}, ServedBy: tc.servedBy,
			}}}}
			r := fitness.Analyze(&arch.Input{
				Program: program, Model: m, GraphQL: []arch.GraphQLSchema{schema},
			})
			equals(t, tc.findings, rules(&r))
			equals(t, tc.matched, r.Rules[0].Matched)
		})
	}
}
