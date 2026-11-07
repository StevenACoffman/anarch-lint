package roles_test

import (
	"testing"

	"example.com/fx/roles"
	"example.com/fx/service"
)

// TestFind checks two invariants: a test enforces them, read from source.
//
//arch:invariant store-is-empty
//arch:invariant finds-nothing
func TestFind(t *testing.T) {
	var _ service.Finder = &roles.SQLStore{}
	if (&roles.SQLStore{}).Find(1) != "" {
		t.Fatal("found something")
	}
}
