// Package app saves through the port alone.
package app

import "example.com/gx/wiring/port"

// Record saves v in whatever store it is given.
func Record(s port.Store, v string) { s.Save(v) }
