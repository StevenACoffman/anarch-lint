package arch

import (
	"errors"
	"strings"
)

// Error codes. Callers branch on these through [ErrorCode], never on the
// message text or on the type of a wrapped cause.
const (
	// EINTERNAL is a failure that is not the caller's fault.
	EINTERNAL = "internal"
	// EINVALID is input that cannot be used as given: a malformed model or
	// judgements file, or a package that does not type-check.
	EINVALID = "invalid"
	// ENOTFOUND is a named input that does not exist.
	ENOTFOUND = "not_found"
)

// Error is the one error type of anarch-lint.
//
// A leaf error carries Code and Message and says what went wrong. A wrapping
// error carries Op, in the form "package.Func", and the Err it wraps; it never
// sets Code. A chain of wrapping errors reads as a one-line logical stack trace:
//
//	gopackages.Load: arch.EINVALID: example.com/x: undefined: y
type Error struct {
	// Code is machine-readable; set only on leaf errors.
	Code string
	// Message is human-readable; set only on leaf errors.
	Message string
	// Op names the operation that failed; set only on wrapping errors.
	Op string
	// Err is the wrapped cause; set only on wrapping errors.
	Err error
}

// Error returns the logical stack trace of e.
func (e *Error) Error() string {
	var b strings.Builder
	if e.Op != "" {
		b.WriteString(e.Op)
		b.WriteString(": ")
	}
	if e.Err != nil {
		b.WriteString(e.Err.Error())
		return b.String()
	}
	if e.Code != "" {
		b.WriteString(e.Code)
		b.WriteString(": ")
	}
	b.WriteString(e.Message)
	return b.String()
}

// Unwrap returns the wrapped cause, so errors.Is and errors.As see through e.
func (e *Error) Unwrap() error { return e.Err }

// ErrorCode returns the code of the first leaf [Error] in err's chain:
// "" for a nil error, and EINTERNAL when the chain carries no code.
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if !errors.As(err, &e) {
		return EINTERNAL
	}
	if e.Code != "" {
		return e.Code
	}
	if e.Err != nil {
		return ErrorCode(e.Err)
	}
	return EINTERNAL
}

// ErrorMessage returns the message of the first leaf [Error] in err's chain:
// "" for a nil error, and a generic message when the chain carries none, so
// internal details never reach a user by accident.
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if !errors.As(err, &e) {
		return "internal error"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return ErrorMessage(e.Err)
	}
	return "internal error"
}
