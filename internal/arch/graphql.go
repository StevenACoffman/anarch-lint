package arch

// GraphQLSchema is what a fitness.graphql rule reads: the root operation
// fields a GraphQL schema declares, and what its operation documents get
// wrong against it.
type GraphQLSchema struct {
	// Fields are the fields of the schema's query, mutation and subscription
	// types, sorted by root type and name; introspection fields are left
	// out.
	Fields []GraphQLField
	// Operations is how many operation documents were checked.
	Operations int
	// Problems are the operation documents' syntax and validation errors,
	// sorted by position.
	Problems []GraphQLProblem
}

// GraphQLField is one field of a root operation type, such as Query.tasks,
// and where the schema declares it.
type GraphQLField struct {
	// Root is the root type's name as the schema declares it: "Query"
	// unless a schema definition renames it.
	Root string
	Name string
	Pos  Position
}

// GraphQLProblem is an operation document's error against its schema.
type GraphQLProblem struct {
	Message string
	Pos     Position
}
