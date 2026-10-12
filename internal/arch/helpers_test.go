package arch_test

import (
	"reflect"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// assert fails the test with msg when condition is false.
func assert(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Fatal(msg)
	}
}

// equals fails the test when exp and act are not deeply equal.
func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// ok fails the test when err is not nil.
func ok(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

// gateInput is a report's part of the gate.
func gateInput(r *arch.Report) *arch.GateInput {
	in := r.GateInput()
	return &in
}
