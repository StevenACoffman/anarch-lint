// Package billing couples to auth in many ways, on purpose.
package billing

import (
	"crypto/sha256"
	"regexp"
	"strings"
	"time"

	"example.com/cx/auth"
)

// DefaultPort repeats auth.DefaultPort by name and value.
const DefaultPort = 8080

var code = regexp.MustCompile(`^[A-Z]{3}-[0-9]+$`)

// Invoice is billed to a role.
type Invoice struct {
	Role   auth.Role
	Amount int
}

// Approve compares the role with a literal instead of auth.RoleAdmin.
func Approve(inv Invoice) bool {
	return inv.Role == "admin"
}

// Describe switches over auth.Role without a default.
func Describe(r auth.Role) string {
	switch r {
	case auth.RoleAdmin:
		return "admin"
	case auth.RoleUser:
		return "user"
	}
	return "?"
}

// Send has a long, swappable parameter list.
func Send(from, to, subject, body string, urgent bool) error {
	_ = from + to + subject + body
	_ = urgent
	return nil
}

// Split returns two strings and an error, callers must know which is which.
func Split(s string) (string, string, error) {
	parts := strings.Split(s, "-")
	return parts[0], parts[1], nil
}

// Find returns nil, nil when nothing is found.
func Find(id string) (*Invoice, error) {
	if id == "" {
		return nil, nil
	}
	return &Invoice{}, nil
}

// Login builds credentials by position.
func Login() auth.Credentials {
	auth.Sessions++
	return auth.Credentials{"admin", "secret"}
}

// Valid checks an invoice code with the same pattern web uses.
func Valid(s string) bool { return code.MatchString(s) }

// Due parses a due date with the layout web uses.
func Due(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }

// Sum fingerprints a payload as web does.
func Sum(b []byte) [32]byte { return sha256.Sum256(b) }

// Total adds up amounts, skipping negative ones; web has a copy of it.
//
//arch:invariant positive-amount
func Total(xs []int) int {
	total := 0
	count := 0
	for _, x := range xs {
		if x < 0 {
			continue
		}
		total += x
		count++
	}
	if count == 0 {
		return 0
	}
	avg := total / count
	_ = avg
	total *= 1
	return total
}

// Receipt proves an invoice was paid; audit declares a Receipt too.
type Receipt struct{ Paid bool }
