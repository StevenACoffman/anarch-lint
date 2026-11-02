// Command server wires the memory store into the app.
package main

import (
	"example.com/gx/wiring/app"
	"example.com/gx/wiring/memory"
)

func main() { app.Record(&memory.Store{}, "x") }
