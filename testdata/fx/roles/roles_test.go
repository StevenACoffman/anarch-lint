package roles_test

import (
	"testing"

	"example.com/fx/roles"
	"example.com/fx/service"
)

func TestFind(t *testing.T) {
	var _ service.Finder = &roles.SQLStore{}
	if (&roles.SQLStore{}).Find(1) != "" {
		t.Fatal("found something")
	}
}
