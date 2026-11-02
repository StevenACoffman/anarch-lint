package memory_test

import (
	"testing"

	"example.com/gx/wiring/memory"
)

// The external test names Store, which keeps it exported.
func TestStore(t *testing.T) { (&memory.Store{}).Save("x") }
