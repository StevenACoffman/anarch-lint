// Package app wires a concrete store into the service: service gains a
// run-time dependency on roles without importing it.
package app

import (
	"example.com/fx/roles"
	"example.com/fx/service"
)

// Run looks up through the SQL store.
func Run() string { return service.Lookup(&roles.SQLStore{}) }
