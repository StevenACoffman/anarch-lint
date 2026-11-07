// Package roles marks each type's pattern role in a different way.
package roles

// Entity is a marker: a type that embeds it is an entity.
type Entity struct{}

// Repository is the interface every repository implements.
type Repository interface {
	Find(id int) string
}

// Customer is an entity because it embeds the marker, and the domain
// model's Client, by its directive.
//
//arch:entity Client
type Customer struct {
	Entity
	Name string
}

// SQLStore is a repository because it implements Repository.
type SQLStore struct{}

// Find finds nothing, and says it keeps the store empty.
//
//arch:invariant store-is-empty
func (*SQLStore) Find(int) string { return "" }

// Handler is a handler because a directive says so.
//
//arch:role handler
type Handler struct{}

// Untagged plays no role.
type Untagged struct{}
