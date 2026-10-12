package arch_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

func TestParseDate(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"2026-10-10": true,
		"2028-02-29": true, // leap year
		"2026-02-29": false,
		"2100-02-29": false, // a century is not a leap year
		"2000-02-29": true,  // unless it is divisible by 400
		"2026-04-31": false,
		"2026-13-01": false,
		"2026-1-01":  false,
		"2026/10/10": false,
		"+026-10-10": false,
		"":           false,
	}
	for input, valid := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := arch.ParseDate(input)
			equals(t, valid, err == nil)
		})
	}
}

func TestDecodeWaivers(t *testing.T) {
	t.Parallel()
	const fp = "0123456789abcdef"
	waiver := func(fields string) string {
		return `{"version": 1, "waivers": [{` + fields + `}]}`
	}
	cases := map[string]struct {
		input   string
		message string
	}{
		"valid": {
			input: waiver(
				`"fingerprint": "` + fp + `", "reason": "r", "approvedBy": "a", "expires": "2027-01-01"`,
			),
		},
		"missing approver": {
			input:   waiver(`"fingerprint": "` + fp + `", "reason": "r", "expires": "2027-01-01"`),
			message: "waivers: fingerprint " + fp + ": reason and approvedBy are required",
		},
		"short fingerprint": {
			input: waiver(
				`"fingerprint": "abc", "reason": "r", "approvedBy": "a", "expires": "2027-01-01"`,
			),
			message: `waivers: fingerprint "abc" is not 16 hex digits`,
		},
		"bad date": {
			input: waiver(
				`"fingerprint": "` + fp + `", "reason": "r", "approvedBy": "a", "expires": "2027-02-30"`,
			),
			message: "waivers: fingerprint " + fp +
				`: expires: date "2027-02-30" is not a calendar date YYYY-MM-DD`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := arch.DecodeWaivers(strings.NewReader(tc.input))
			if tc.message == "" {
				ok(t, err)
				return
			}
			equals(t, arch.EINVALID, arch.ErrorCode(err))
			equals(t, tc.message, arch.ErrorMessage(err))
		})
	}
}

func TestGateWaivers(t *testing.T) {
	t.Parallel()
	m := fixtureModel()
	r := analyze(&m)
	// An empty baseline: every error finding is new.
	baseline := arch.NewBaseline(gateInput(&r))
	baseline.Findings = nil
	var waivers []arch.Waiver
	for i := range r.Findings {
		if r.Findings[i].Severity == arch.SeverityError {
			waivers = append(waivers, arch.Waiver{
				Fingerprint: r.Findings[i].Fingerprint,
				Reason:      "r",
				ApprovedBy:  "a",
				Expires:     "2027-01-01",
			})
		}
	}
	unused := arch.Waiver{
		Fingerprint: "ffffffffffffffff",
		Reason:      "r",
		ApprovedBy:  "a",
		Expires:     "2027-01-01",
	}

	before := arch.Gate(gateInput(&r),
		&baseline,
		&arch.GateOptions{Waivers: append(waivers, unused), Today: "2026-12-31"},
	)
	assert(t, before.Passed, "waived findings failed the gate: "+before.Reason)
	equals(t, len(waivers), len(before.Waived))
	equals(t, []arch.Waiver{unused}, before.UnusedWaivers)

	onTheDay := arch.Gate(
		gateInput(&r),
		&baseline,
		&arch.GateOptions{Waivers: waivers, Today: "2027-01-01"},
	)
	assert(t, onTheDay.Passed, "a waiver stopped applying on its last day")

	after := arch.Gate(
		gateInput(&r),
		&baseline,
		&arch.GateOptions{Waivers: waivers, Today: "2027-01-02"},
	)
	assert(t, !after.Passed, "expired waivers still passed the gate")
	equals(t, len(waivers), len(after.ExpiredWaivers))
	equals(t, len(waivers), len(after.New))
}
