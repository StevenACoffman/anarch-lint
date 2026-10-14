package calls

// stopwatch has a method named Now.
type stopwatch struct{}

// Now returns zero.
func (stopwatch) Now() int { return 0 }

// Shadowed calls Now on a local variable named time: a rule on time.Now must
// not flag it.
func Shadowed() {
	time := stopwatch{}
	_ = time.Now()
}
