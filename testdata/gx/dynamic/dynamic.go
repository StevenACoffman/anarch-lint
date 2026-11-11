// Package dynamic holds calls through an interface and a function value, for
// the loader's dynamic-call test.
package dynamic

import (
	"io"
	"os"
)

// Sink is implemented once, by sink.
type Sink interface{ Put(s string) }

type sink struct{}

// Put does nothing.
func (sink) Put(string) {}

// Report writes to whatever it is given: Main gives it os.Stdout.
func Report(w io.Writer) {
	_, _ = w.Write([]byte("report"))
}

// Main wires the program.
func Main() {
	Report(os.Stdout)
	var s Sink = sink{}
	s.Put("a")
}

// Each calls f, which no caller supplies: a call nothing resolves.
func Each(f func()) { f() }
