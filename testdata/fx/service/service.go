// Package service looks things up through an interface it owns.
package service

// Finder finds by id.
type Finder interface {
	Find(id int) string
}

// Lookup finds id 1 with any finder.
func Lookup(f Finder) string { return f.Find(1) }
