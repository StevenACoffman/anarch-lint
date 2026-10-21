// Package platform has a file for plan9 alone, which no build of the tests
// includes.
package platform

import "strings"

// Upper upper-cases s.
func Upper(s string) string { return strings.ToUpper(s) }
