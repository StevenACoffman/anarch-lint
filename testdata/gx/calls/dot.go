package calls

import . "time"

// Dotted calls time.Now through a dot import: a rule on time.Now must flag
// it.
func Dotted() { _ = Now() }
