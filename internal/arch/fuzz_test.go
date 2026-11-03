package arch_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// FuzzDecodeWaivers checks that no input crashes the waivers decoder, that
// it refuses only as EINVALID, and that what it accepts encodes and decodes
// to the same waivers.
func FuzzDecodeWaivers(f *testing.F) {
	f.Add(`{"version": 1, "waivers": [{"fingerprint": "0123456789abcdef", "reason": "r", ` +
		`"approvedBy": "a", "expires": "2027-01-01"}]}`)
	f.Add(`{"version": 1, "waivers": []}`)
	f.Add(`{"version": 2}`)
	f.Fuzz(func(t *testing.T, input string) {
		ws, err := arch.DecodeWaivers(strings.NewReader(input))
		if err != nil {
			if arch.ErrorCode(err) != arch.EINVALID {
				t.Fatalf("refused with %q, not EINVALID: %v", arch.ErrorCode(err), err)
			}
			return
		}
		src, err := json.Marshal(map[string]any{"version": 1, "waivers": ws})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		again, err := arch.DecodeWaivers(bytes.NewReader(src))
		if err != nil {
			t.Fatalf("re-decode: %v", err)
		}
		if !reflect.DeepEqual(ws, again) && len(ws)+len(again) > 0 {
			t.Fatalf("round trip changed the waivers: %+v, then %+v", ws, again)
		}
	})
}

// FuzzDecodeJudgements checks that no input crashes the judgements decoder,
// that it refuses only as EINVALID, and that it decodes an input the same
// way every time.
func FuzzDecodeJudgements(f *testing.F) {
	f.Add(`{"version": 1, "signer": "r@example.com", "verdicts": [{"criterion": ` +
		`"subdomain/billing", "answer": "core", "rationale": "r", "evidenceHash": "h", ` +
		`"events": [{"role": "judge", "config": "model-v1"}, {"role": "human"}]}]}`)
	f.Add(`{"version": 1, "verdicts": [{"criterion": "1.1.5", "answer": ` +
		`"insufficient-evidence", "provenance": "llm", "rationale": "r", "evidenceHash": "h", ` +
		`"approved": true}]}`)
	f.Add(`{"version": 1, "verdicts": [{"criterion": "3.3", "score": 11}]}`)
	f.Fuzz(func(t *testing.T, input string) {
		signer, vs, err := arch.DecodeSignedJudgements(strings.NewReader(input))
		if err != nil {
			if arch.ErrorCode(err) != arch.EINVALID {
				t.Fatalf("refused with %q, not EINVALID: %v", arch.ErrorCode(err), err)
			}
			return
		}
		signer2, vs2, err := arch.DecodeSignedJudgements(strings.NewReader(input))
		if err != nil || signer != signer2 || !reflect.DeepEqual(vs, vs2) {
			t.Fatalf("decoding twice differs: %v", err)
		}
	})
}
