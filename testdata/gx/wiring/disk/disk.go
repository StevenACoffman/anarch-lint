// Package disk is a Store nothing wires, which CHA links but VTA does not.
package disk

// Store keeps values on disk.
type Store struct{ path string }

// Save keeps v.
func (s *Store) Save(v string) { _ = s.path + v }
