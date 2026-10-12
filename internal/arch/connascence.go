package arch

import (
	"fmt"
)

// Connascence strengths, from weakest to strongest, in connascence.io's order
// (connascence.md §1.1). coupling.dev ranks position above algorithm instead;
// the default limits fall on neither side of that difference (§9.1).
const (
	// StrengthContranascence is the opposite of connascence: a property two
	// elements must hold different (Page-Jones; connascence.md §3.5). It has
	// no rank on the site's scale.
	StrengthContranascence Strength = iota
	StrengthName
	StrengthType
	StrengthMeaning
	StrengthPosition
	StrengthAlgorithm
	StrengthExecution
	StrengthTiming
	StrengthValue
	StrengthIdentity
)

// Bases of a connascence instance (connascence.md §5).
const (
	// BasisFact is sharing that is itself the coupling; a fact can gate.
	BasisFact Basis = "fact"
	// BasisCandidate is sharing that is evidence of coupling but may be
	// coincidence; it gates only once a reviewer confirms it.
	BasisCandidate Basis = "candidate"
)

// Confidences of a candidate: how rarely its evidence happens by chance.
const (
	ConfidenceHigh Confidence = "high"
	ConfidenceLow  Confidence = "low"
)

// Strength is a kind of connascence, ranked from 1 (name) to 9 (identity):
// the stronger, the harder to discover or to refactor. Contranascence has no
// rank on the site; it is reported with strength 0.
type Strength int

// Basis says whether an instance is a fact or a candidate.
type Basis string

// Confidence grades a candidate; a fact has none.
type Confidence string

// ConnascenceCount is one cell of the summary: how many instances of a kind
// lie at a locality, and their total degree.
type ConnascenceCount struct {
	Kind       Strength `json:"kind"`
	Locality   Locality `json:"locality"`
	Facts      int      `json:"facts"`
	Candidates int      `json:"candidates"`
	Degree     int      `json:"degree"`
}

// Connascence is one instance: the thing several places must agree on, the
// places that agree on it, and its strength, degree and locality
// (connascence.md §1).
type Connascence struct {
	// Fingerprint identifies the instance across runs. It hashes the kind,
	// the subject and the detector, never a site, so moving code or adding a
	// site keeps it; a rise in degree is ratcheted instead (§6).
	Fingerprint string   `json:"fingerprint"`
	Kind        Strength `json:"kind"`
	Strength    int      `json:"strength"`
	// Detector names the predicate that found the instance, such as
	// "meaning.shadowed-constant".
	Detector string `json:"detector"`
	// Subject is what must be agreed on: an identifier, a value, a type.
	Subject string `json:"subject"`
	// Object is the declared object the instance is about, when it is about
	// one: the variable, constant, type or function every site agrees on.
	// Knowledge of it flows from the package that declares it. A clone, a
	// repeated pattern or two equal values are about no object.
	Object     string     `json:"object,omitempty"`
	Basis      Basis      `json:"basis"`
	Confidence Confidence `json:"confidence,omitempty"`
	Locality   Locality   `json:"locality"`
	// Degree is the number of distinct entities that must change together,
	// counted one level below the locality.
	Degree int `json:"degree"`
	// SiteCount is the number of sites; Sites lists the first of them.
	SiteCount int    `json:"siteCount"`
	Sites     []Site `json:"sites"`
	Message   string `json:"message"`
	Fix       string `json:"fix"`
}

// strengthNames are the kinds' names, indexed by strength.
func strengthNames() []string {
	return []string{
		"contranascence", "name", "type", "meaning", "position", "algorithm",
		"execution", "timing", "value", "identity",
	}
}

// String returns the kind's name: "name" for 1, "identity" for 9.
func (s Strength) String() string {
	if names := strengthNames(); s >= 0 && int(s) < len(names) {
		return names[s]
	}
	return fmt.Sprintf("strength(%d)", int(s))
}

// Dynamic reports whether the kind exists only when the code runs:
// execution, timing, value and identity.
func (s Strength) Dynamic() bool { return s >= StrengthExecution }

// MarshalText writes the kind's name.
func (s Strength) MarshalText() ([]byte, error) { return []byte(s.String()), nil }
