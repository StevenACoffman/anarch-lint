// Package cgo calls C, so go/packages type-checks the file cgo writes in
// place of this one.
package cgo

// int add(int a, int b) { return a + b; }
import "C"

import "strconv"

// Add adds through C.
func Add(a, b int) string {
	return strconv.Itoa(int(C.add(C.int(a), C.int(b))))
}
