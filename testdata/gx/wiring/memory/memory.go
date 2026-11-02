// Package memory is the Store the server wires.
package memory

// Store keeps values in memory.
type Store struct{ values []string }

// Save keeps v.
func (s *Store) Save(v string) { s.values = append(s.values, v) }
