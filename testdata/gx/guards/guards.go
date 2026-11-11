// Package guards holds the shapes an error guard does and does not take.
package guards

import "errors"

// ErrNegative is returned for a negative amount.
var ErrNegative = errors.New("negative amount")

// Money is an amount of cents.
type Money struct{ cents int }

// NewMoney refuses a negative amount: a guard.
func NewMoney(cents int) (Money, error) {
	if cents < 0 {
		return Money{}, ErrNegative
	}
	return Money{cents: cents}, nil
}

// MustAdd panics on overflow: a guard.
func (m Money) MustAdd(n Money) Money {
	if m.cents+n.cents < m.cents {
		panic("overflow")
	}
	return Money{cents: m.cents + n.cents}
}

// Cents returns early without refusing: no guard.
func (m Money) Cents() int {
	if m.cents == 0 {
		return 0
	}
	return m.cents
}

// Parse checks an error from a call with an init statement: no guard.
func Parse(s string) (Money, error) {
	if err := check(s); err != nil {
		return Money{}, err
	}
	return Money{}, nil
}

// Nested refuses only inside a loop: no guard at the top of its body.
func Nested(amounts []int) error {
	for _, a := range amounts {
		if a < 0 {
			return ErrNegative
		}
	}
	return nil
}

func check(string) error { return nil }
