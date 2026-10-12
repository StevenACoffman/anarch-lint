// Package web is a second downstream of auth.
package web

import (
	"crypto/sha256"
	"reflect"
	"regexp"
	"time"
	"unsafe"

	"example.com/cx/auth"
)

var code = regexp.MustCompile(`^[A-Z]{3}-[0-9]+$`)

// Admin reports whether the role is the administrator's.
func Admin(r auth.Role) bool { return r == "admin" }

// Allowed uses the constant: connascence of name only.
func Allowed(r auth.Role) bool { return r == auth.RoleAdmin }

// Reset clears the session count.
func Reset() { auth.Sessions = 0 }

// Peek reads the secret's unexported field through unsafe and reflect.
func Peek(s *auth.Secret) string {
	type mirror struct{ token string }
	m := (*mirror)(unsafe.Pointer(s))
	_ = reflect.ValueOf(s).Elem().FieldByName("token")
	return m.token
}

// Steal is linked in as auth's unexported check, which auth never declares.
//
//go:linkname Steal example.com/cx/auth.check
func Steal(r auth.Role) bool { return r == "admin" }

// Valid checks a code with billing's pattern.
func Valid(s string) bool { return code.MatchString(s) }

// Due parses a date with billing's layout.
func Due(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }

// Sum fingerprints a payload as billing does.
func Sum(b []byte) [32]byte { return sha256.Sum256(b) }

// Tally is a renamed copy of billing.Total.
func Tally(vals []int) int {
	sum := 0
	n := 0
	for _, v := range vals {
		if v < 7 {
			continue
		}
		sum += v
		n++
	}
	if n == 0 {
		return 1
	}
	mean := sum / n
	_ = mean
	sum *= 2
	return sum
}
