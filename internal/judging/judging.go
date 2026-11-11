// Package judging asks judges the questions the measures cannot answer, and
// admits their answers as verification events (mmi-plan.md §9.3 C–G, J). It
// never calls a model. Emit renders a prompt for each slot, judge
// configuration and sample, with canaries of known answer mixed in; an
// operator's agents answer them; Admit parses each reply strictly, refuses a
// configuration's answers in a family whose canaries it missed, and tallies
// the independent answers into verdicts. Every family stays human: a judge's
// verdict is a draft until a person attests it (docs/adr/0010). Pure: values
// in, values out; cmd/judge does the I/O.
package judging

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// PromptVersion is turned by hand when the prompt template changes, so a
// cached reply to the old template is not reused.
const PromptVersion = 2

// manifestVersion is the only manifest format this build reads.
const manifestVersion = 1

// The kinds of item.
const (
	// KindSlot asks a slot of the run's reports.
	KindSlot Kind = "slot"
	// KindCanary asks a question of known answer, mixed in with the slots
	// so a judge cannot tell it apart, to qualify the configuration.
	KindCanary Kind = "canary"
	// KindGold asks a human-labelled question, to measure agreement and
	// calibration; it never qualifies or refuses a configuration.
	KindGold Kind = "gold"
)

// mmiFamily is the family of the five MMI judgement criteria, whose slot IDs
// have no "family/" prefix.
const mmiFamily = "mmi"

// Kind says why an item is asked.
type Kind string

// Config is one judge configuration: the model and the prompt variant an
// operator runs. Independence between judges comes from their recorded
// configurations, never from what a judge says of itself.
type Config struct {
	// Name is the operator's label for it, such as "a".
	Name    string `json:"name"`
	Model   string `json:"model"`
	Variant string `json:"variant,omitempty"`
}

// Item is one prompt: a question of one kind, for one configuration and
// sample. Sample 1 shows the exhibits in reverse order, so an answer that
// depends on the order shows as a split.
type Item struct {
	// Key names the prompt and reply files; it hashes the slot's evidence
	// hash, the prompt version, the configuration and the sample.
	Key          string `json:"key"`
	Kind         Kind   `json:"kind"`
	Family       string `json:"family"`
	Config       string `json:"config"`
	Sample       int    `json:"sample"`
	EvidenceHash string `json:"evidenceHash"`
	Edition      string `json:"edition"`
	// View is what the prompt shows, after the budget: a reply may cite
	// only its exhibits. Omitted counts the exhibits the budget left out.
	View    arch.JudgeView `json:"view"`
	Omitted int            `json:"omitted,omitempty"`
}

// Manifest lists the items of one judging run.
type Manifest struct {
	Version       int      `json:"version"`
	PromptVersion int      `json:"promptVersion"`
	Configs       []Config `json:"configs"`
	Items         []Item   `json:"items"`
}

// Budget caps what one prompt shows: the number of exhibits and the length
// of each. What it leaves out is counted in the prompt, never dropped
// silently (techne's output budgets).
type Budget struct {
	MaxExhibits     int
	MaxExhibitBytes int
}

// DefaultBudget is the budget when the operator sets none.
func DefaultBudget() Budget { return Budget{MaxExhibits: 40, MaxExhibitBytes: 600} }

// Hash identifies the configuration for a verification event: its model,
// variant and the prompt version.
func (c *Config) Hash() string {
	return shortHash(c.Model, c.Variant, strconv.Itoa(PromptVersion))
}

// FamilyOf returns the family of a slot ID: the part before "/", or "mmi"
// for an MMI criterion.
func FamilyOf(id string) string {
	if family, _, ok := strings.Cut(id, "/"); ok {
		return family
	}
	return mmiFamily
}

// shortHash is the first 16 hex digits of the SHA-256 of the parts, each
// length-prefixed, so no part can imitate a boundary.
func shortHash(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(strconv.Itoa(len(p)) + ":" + p + ",")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}
