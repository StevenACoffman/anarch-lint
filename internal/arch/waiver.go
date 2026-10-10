package arch

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

// waiversVersion is the only waivers-file version this build reads.
const waiversVersion = 1

// Date is a calendar date written YYYY-MM-DD. Dates in that form order the
// same way as strings, so the domain compares them without a clock or the
// time package; the shell supplies today's date.
type Date string

// Waiver accepts one finding for a while, with a reason and an approver: a
// conscious, expiring exception rather than a silent baseline entry (archfit's
// waivers; mmi-ref-plan.md §4.4). Waivers live in their own file, not the
// model, so adding one does not change the model hash and orphan the
// baseline.
type Waiver struct {
	Fingerprint string `json:"fingerprint"`
	Reason      string `json:"reason"`
	ApprovedBy  string `json:"approvedBy"`
	// Expires is the last day the waiver applies.
	Expires Date `json:"expires"`
}

// waiversFile is the on-disk form of a set of waivers.
type waiversFile struct {
	Version int      `json:"version"`
	Waivers []Waiver `json:"waivers"`
}

// ParseDate returns s as a Date, or EINVALID when s is not a real calendar
// date in the form YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	if len(s) == len("2006-01-02") && s[4] == '-' && s[7] == '-' {
		year, month, day := digits(s[:4]), digits(s[5:7]), digits(s[8:])
		if year >= 0 && month >= 1 && month <= 12 && day >= 1 && day <= daysIn(year, month) {
			return Date(s), nil
		}
	}
	return "", &Error{
		Code:    EINVALID,
		Message: fmt.Sprintf("date %q is not a calendar date YYYY-MM-DD", s),
	}
}

// DecodeWaivers reads a waivers file: {"version": 1, "waivers": [...]}. Every
// field of a waiver is required; a fingerprint that is not 16 hex digits, a
// date that is not a calendar date, or two waivers for one finding is
// EINVALID.
func DecodeWaivers(r io.Reader) ([]Waiver, error) {
	const op = "arch.DecodeWaivers"
	invalid := func(format string, args ...any) error {
		return &Error{
			Op:  op,
			Err: &Error{Code: EINVALID, Message: "waivers: " + fmt.Sprintf(format, args...)},
		}
	}
	var f waiversFile
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, invalid("%v", err)
	}
	if f.Version != waiversVersion {
		return nil, invalid("version %d is not supported; want %d", f.Version, waiversVersion)
	}
	seen := map[string]bool{}
	for _, w := range f.Waivers {
		if err := w.validate(seen); err != nil {
			return nil, invalid("%s", err)
		}
		seen[w.Fingerprint] = true
	}
	return f.Waivers, nil
}

// validate checks one waiver; seen holds the fingerprints already waived.
func (w *Waiver) validate(seen map[string]bool) error {
	switch {
	case !isFingerprint(w.Fingerprint):
		return fmt.Errorf("fingerprint %q is not %d hex digits", w.Fingerprint, 2*fingerprintBytes)
	case seen[w.Fingerprint]:
		return fmt.Errorf("fingerprint %s is waived twice", w.Fingerprint)
	case w.Reason == "", w.ApprovedBy == "":
		return fmt.Errorf("fingerprint %s: reason and approvedBy are required", w.Fingerprint)
	}
	if _, err := ParseDate(string(w.Expires)); err != nil {
		return fmt.Errorf("fingerprint %s: expires: %s", w.Fingerprint, ErrorMessage(err))
	}
	return nil
}

// isFingerprint reports whether s has the form of a finding's fingerprint.
func isFingerprint(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil && len(s) == 2*fingerprintBytes
}

// digits returns the number s spells in ASCII digits, or -1 when s is empty
// or holds anything else, a sign included.
func digits(s string) int {
	if s == "" {
		return -1
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = 10*n + int(c-'0')
	}
	return n
}

// daysIn returns the number of days in a month of the Gregorian calendar.
func daysIn(year, month int) int {
	switch month {
	case 2:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}
