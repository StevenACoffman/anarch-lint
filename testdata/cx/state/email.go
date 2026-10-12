// Package state holds stateful types whose methods must be called in order.
package state

import (
	"errors"
	"time"
)

// Email must not change once sent.
type Email struct {
	subject string
	sent    bool
}

// SetSubject refuses to change a sent email.
func (e *Email) SetSubject(s string) error {
	if e.sent {
		return errors.New("already sent")
	}
	e.subject = s
	return nil
}

// Send sends the email.
func (e *Email) Send() {
	e.sent = true
}

// Conn has an Open/Close lifecycle.
type Conn struct{ open bool }

// Open opens the connection.
func (c *Conn) Open() { c.open = true }

// Close closes it.
func (c *Conn) Close() { c.open = false }

// Wait sleeps instead of synchronizing.
func Wait() { time.Sleep(10 * time.Millisecond) }

// Same compares two emails by identity.
func Same(a, b *Email) bool { return a == b }
