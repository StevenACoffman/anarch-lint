// Package audit records who did what. It shares an owner with auth and calls
// only its contract.
package audit

import "example.com/cx/auth"

// Record notes an action against auth's version.
func Record(action string) string { return action + "@" + auth.Version() }

// Receipt is audit's own record of a payment, of the same name as billing's.
type Receipt struct{ Action string }
