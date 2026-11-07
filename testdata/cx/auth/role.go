// Package auth is the fixture's upstream subsystem: roles, levels and a
// secret that other subsystems reach into.
package auth

import "sync"

// Role is a user's role.
type Role string

// The roles. RoleAdmin is compared as a literal elsewhere, a fact of
// connascence of meaning.
const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	// RoleDefault aliases RoleUser, so it is not contranascence.
	RoleDefault = RoleUser
)

// Level is a clearance level.
type Level int

// The levels. High repeats Mid's value: contranascence.
const (
	Low  Level = 1
	Mid  Level = 2
	High Level = 2
)

// DefaultPort is repeated, with the same name and value, in billing.
const DefaultPort = 8080

// Sessions counts the open sessions; other packages write it.
var Sessions int

// Secret holds a token behind an unexported field.
type Secret struct {
	token string
}

// Token returns the secret's token.
func (s Secret) Token() string { return s.token }

// Credentials are passed around by their field order.
type Credentials struct {
	User     string
	Password string
}

// Check reports whether r may administer.
func Check(r Role) bool { return r == RoleAdmin }

// Store is a lock-guarded map of sessions.
type Store struct {
	mu   sync.Mutex
	data map[string]int
}

// Put stores v under k, unlocking with defer.
func (s *Store) Put(k string, v int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = v
}

// Get forgets to unlock: an unpaired Lock.
func (s *Store) Get(k string) int {
	s.mu.Lock()
	return s.data[k]
}

// Version is auth's published contract: a plain string. Its directive
// misspells an invariant: unknown-invariant-directive.
//
//arch:invariant admin-aproves
func Version() string { return "1" }
