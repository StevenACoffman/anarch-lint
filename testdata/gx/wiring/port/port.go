// Package port declares what the app needs.
package port

// Store keeps values.
type Store interface {
	Save(v string)
}
