// Package calls holds calls that a rule on calls must and must not flag.
package calls

import clock "time"

// Aliased calls time.Now through an aliased import: a rule on time.Now must
// flag it.
func Aliased() { _ = clock.Now() }
