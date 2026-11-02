package judging

import (
	"bytes"
	_ "embed" // the canary corpus is compiled in
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// labelledVersion is the only version of a labelled-questions file this
// build reads.
const labelledVersion = 1

// canaryCorpus holds the canaries: for each family judged mechanically
// enough to have them, a clear positive, a clear negative and a decoy whose
// surface looks positive (mmi-plan.md §9.3 E). Add one when a real
// judgement disappoints, rather than inventing one to match what a judge
// already does.
//
//go:embed canaries.json
var canaryCorpus []byte

// labelledFile is the form of a file of labelled questions.
type labelledFile struct {
	Version int        `json:"version"`
	Items   []Labelled `json:"items"`
}

// Canaries returns the compiled-in canary corpus.
func Canaries() ([]Labelled, error) {
	return DecodeLabelled(bytes.NewReader(canaryCorpus))
}

// DecodeLabelled reads a file of labelled questions, canaries or a gold set:
// {"version": 1, "items": [...]}. An unknown field, an unknown slot family or
// MMI criterion, an answer its family does not accept, and a score or range
// outside 0–10 are EINVALID.
func DecodeLabelled(r io.Reader) ([]Labelled, error) {
	const op = "judging.DecodeLabelled"
	var f labelledFile
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, &arch.Error{Op: op, Err: &arch.Error{Code: arch.EINVALID, Message: err.Error()}}
	}
	if f.Version != labelledVersion {
		return nil, &arch.Error{Op: op, Err: &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf(
			"version %d is not supported; want %d", f.Version, labelledVersion)}}
	}
	for i := range f.Items {
		if msg := f.Items[i].problem(); msg != "" {
			return nil, &arch.Error{Op: op, Err: &arch.Error{
				Code: arch.EINVALID, Message: f.Items[i].Criterion + ": " + msg,
			}}
		}
	}
	return f.Items, nil
}

// problem says what makes a labelled question unusable, or "".
func (l *Labelled) problem() string {
	answers, ok := arch.SlotAnswers(l.Criterion)
	switch {
	case !ok:
		return "not a judgement criterion or slot family"
	case answers == nil && l.Answer != "":
		return "an MMI criterion is labelled with a score"
	case answers == nil && (l.Score < 0 || l.Score > 10 || l.ScoreMax > 10):
		return "a score is outside 0–10"
	case answers != nil && !slices.Contains(answers, l.Answer):
		return fmt.Sprintf("answer %q is not one its family accepts", l.Answer)
	}
	return ""
}
