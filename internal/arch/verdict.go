package arch

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Roles of a verification event.
const (
	// RoleJudge is an automated judge, such as an LLM, whose verdict is a
	// draft until a human attests it.
	RoleJudge EventRole = "judge"
	// RoleHuman is a person who judged the slot or accepted a judge's
	// verdict.
	RoleHuman EventRole = "human"
)

// The first judgements format's provenances, read only when decoding.
const (
	legacyHuman = "human"
	legacyLLM   = "llm"
)

// judgementsVersion is the only judgements-file version this build reads.
const judgementsVersion = 1

// EventRole says in what capacity a verification event attests a verdict.
type EventRole string

// Verdict is one reviewer's answer to a judgement slot: a Score for an MMI
// criterion, an Answer for a family slot, never both. Its Events say who
// attests it; a verdict asserts nothing about its own trust.
type Verdict struct {
	Criterion string `json:"criterion"`
	Score     int    `json:"score"`
	Answer    string `json:"answer,omitempty"`
	Rationale string `json:"rationale"`
	// EvidenceHash must equal the slot's hash for the verdict to apply.
	EvidenceHash string `json:"evidenceHash"`
	// Edition optionally records the slot's edition when it was judged, so
	// a stale slot can say whether the question or the evidence changed.
	Edition string              `json:"edition,omitempty"`
	Events  []VerificationEvent `json:"events"`
}

// VerificationEvent is one attestation of a verdict (skillet's
// verification events). Actor comes from the judgements file's signer,
// never from the event, so no entry can name its own author.
type VerificationEvent struct {
	Role  EventRole `json:"role"`
	Actor string    `json:"actor,omitempty"`
	// Config identifies a judge's configuration: its model, version and
	// prompt template. A human event has none.
	Config string `json:"config,omitempty"`
	// Confidence is the judge's stated confidence in [0, 1], and Cites the
	// exhibits its reply cited; a human event has neither. Neither gates nor
	// ranks anything until a gold set shows the confidence is calibrated
	// (IMPROVEMENTS.md §7.5).
	Confidence *float64 `json:"confidence,omitempty"`
	Cites      []string `json:"cites,omitempty"`
}

// judgementsFile is the on-disk form of a set of verdicts. Signer names who
// committed the file, and becomes the actor of every event in it.
type judgementsFile struct {
	Version  int             `json:"version"`
	Signer   string          `json:"signer,omitempty"`
	Verdicts []verdictRecord `json:"verdicts"`
}

// verdictRecord is a verdict as a file holds it. A verdict of the first
// format carries provenance and approved instead of events; it is read as
// one event: human when approved, and a judge's draft otherwise.
type verdictRecord struct {
	Criterion    string        `json:"criterion"`
	Score        int           `json:"score"`
	Answer       string        `json:"answer,omitempty"`
	Rationale    string        `json:"rationale"`
	EvidenceHash string        `json:"evidenceHash"`
	Edition      string        `json:"edition,omitempty"`
	Events       []eventRecord `json:"events,omitempty"`
	Provenance   string        `json:"provenance,omitempty"`
	Approved     *bool         `json:"approved,omitempty"`
}

// eventRecord is an event as a file holds it: with no actor.
type eventRecord struct {
	Role       EventRole `json:"role"`
	Config     string    `json:"config,omitempty"`
	Confidence *float64  `json:"confidence,omitempty"`
	Cites      []string  `json:"cites,omitempty"`
}

// DecodeJudgements reads a judgements file: {"version": 1, "signer": "…",
// "verdicts": [...]}. An unknown slot, two verdicts for one slot, a verdict
// with no events or with both events and the first format's provenance, an
// unknown role or provenance, a score outside 0–10, an answer on an MMI
// criterion, and a score or an invalid answer on a family slot are all
// EINVALID.
func DecodeJudgements(r io.Reader) ([]Verdict, error) {
	_, verdicts, err := DecodeSignedJudgements(r)
	return verdicts, err
}

// DecodeSignedJudgements reads a judgements file as DecodeJudgements does,
// and returns its signer too, for a tool that rewrites the file.
func DecodeSignedJudgements(r io.Reader) (string, []Verdict, error) {
	const op = "arch.DecodeJudgements"
	invalid := func(format string, args ...any) error {
		return &Error{
			Op:  op,
			Err: &Error{Code: EINVALID, Message: "judgements: " + fmt.Sprintf(format, args...)},
		}
	}
	var f judgementsFile
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return "", nil, invalid("%v", err)
	}
	if f.Version != judgementsVersion {
		return "", nil, invalid(
			"version %d is not supported; want %d",
			f.Version,
			judgementsVersion,
		)
	}
	seen := map[string]bool{}
	out := make([]Verdict, 0, len(f.Verdicts))
	for i := range f.Verdicts {
		rec := &f.Verdicts[i]
		if seen[rec.Criterion] {
			return "", nil, invalid("criterion %s has two verdicts", rec.Criterion)
		}
		v, msg := rec.verdict(f.Signer)
		if msg == "" {
			msg = v.problem()
		}
		if msg != "" {
			return "", nil, invalid("criterion %s: %s", rec.Criterion, msg)
		}
		seen[rec.Criterion] = true
		out = append(out, v)
	}
	return f.Signer, out, nil
}

// EncodeJudgements writes verdicts as a judgements file signed by signer,
// sorted by criterion, in the form DecodeJudgements reads. An event's actor
// is not written: the signer is every event's actor.
func EncodeJudgements(w io.Writer, signer string, verdicts []Verdict) error {
	f := judgementsFile{
		Version: judgementsVersion, Signer: signer,
		Verdicts: make([]verdictRecord, 0, len(verdicts)),
	}
	for i := range verdicts {
		v := &verdicts[i]
		rec := verdictRecord{
			Criterion: v.Criterion, Score: v.Score, Answer: v.Answer, Rationale: v.Rationale,
			EvidenceHash: v.EvidenceHash, Edition: v.Edition,
		}
		for _, e := range v.Events {
			rec.Events = append(rec.Events, eventRecord{
				Role: e.Role, Config: e.Config, Confidence: e.Confidence, Cites: e.Cites,
			})
		}
		f.Verdicts = append(f.Verdicts, rec)
	}
	slices.SortFunc(f.Verdicts, func(a, b verdictRecord) int {
		return strings.Compare(a.Criterion, b.Criterion)
	})
	return WriteJSON(w, f)
}

// verdict converts a record, its events signed by signer, or says why it
// cannot.
func (rec *verdictRecord) verdict(signer string) (Verdict, string) {
	v := Verdict{
		Criterion: rec.Criterion, Score: rec.Score, Answer: rec.Answer,
		Rationale: rec.Rationale, EvidenceHash: rec.EvidenceHash, Edition: rec.Edition,
	}
	var msg string
	switch legacy := rec.Provenance != "" || rec.Approved != nil; {
	case legacy && len(rec.Events) > 0:
		msg = "events and the first format's provenance and approved cannot be mixed"
	case legacy:
		v.Events, msg = rec.legacyEvents(signer)
	case len(rec.Events) == 0:
		msg = "no events attest the verdict"
	default:
		v.Events, msg = rec.signedEvents(signer)
	}
	return v, msg
}

// legacyEvents reads the first format's provenance and approved as one
// event: a human's when approved, and a judge's draft otherwise.
func (rec *verdictRecord) legacyEvents(signer string) ([]VerificationEvent, string) {
	if rec.Provenance != legacyHuman && rec.Provenance != legacyLLM {
		return nil, fmt.Sprintf("provenance %q is not human or llm", rec.Provenance)
	}
	role := RoleJudge
	if rec.Approved != nil && *rec.Approved {
		role = RoleHuman
	}
	return []VerificationEvent{{Role: role, Actor: signer}}, ""
}

// signedEvents returns the record's events, each signed by signer.
func (rec *verdictRecord) signedEvents(signer string) ([]VerificationEvent, string) {
	out := make([]VerificationEvent, 0, len(rec.Events))
	for _, e := range rec.Events {
		if e.Role != RoleJudge && e.Role != RoleHuman {
			return nil, fmt.Sprintf("role %q is not judge or human", e.Role)
		}
		out = append(out, VerificationEvent{
			Role: e.Role, Actor: signer, Config: e.Config, Confidence: e.Confidence, Cites: e.Cites,
		})
	}
	return out, ""
}

// problem returns what makes v unusable, or "" when nothing does.
func (v *Verdict) problem() string {
	q, ok := questionOf(v.Criterion)
	switch {
	case !ok:
		return "not a judgement criterion or slot family"
	case q.answers == nil:
		return v.scoreProblem()
	default:
		return v.answerProblem(q.answers)
	}
}

// scoreProblem checks the verdict of an MMI criterion: a score, or the
// answer that the evidence is insufficient, never both.
func (v *Verdict) scoreProblem() string {
	switch {
	case v.Answer == AnswerInsufficient && v.Score != 0:
		return "an abstaining verdict takes no score"
	case v.Answer != "" && v.Answer != AnswerInsufficient:
		return "an MMI criterion takes a score, or the answer " + AnswerInsufficient
	case v.Score < 0 || v.Score > 10:
		return fmt.Sprintf("score %d is outside 0–10", v.Score)
	}
	return ""
}

// answerProblem checks the verdict of a family slot: one of answers, no score.
func (v *Verdict) answerProblem(answers []string) string {
	switch {
	case v.Score != 0:
		return "a slot family takes an answer, not a score"
	case !slices.Contains(answers, v.Answer):
		return fmt.Sprintf("answer %q is not one of %s", v.Answer, strings.Join(answers, ", "))
	}
	return ""
}

// attestedBy reports an event of the role among the verdict's events.
func (v *Verdict) attestedBy(role EventRole) bool {
	return slices.ContainsFunc(v.Events, func(e VerificationEvent) bool { return e.Role == role })
}
