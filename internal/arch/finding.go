package arch

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"slices"
)

// Severities. Only an error-severity finding that is new against the baseline
// fails the gate; a warning is advice.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Rule names, one per architecture predicate (mmi.md §7.4).
const (
	RuleUnassignedPackage     = "unassigned-package"
	RuleAmbiguousPackage      = "ambiguous-package"
	RuleStalePattern          = "stale-pattern"
	RuleLayerOrder            = "layer-order-violation"
	RuleSubsystemOrder        = "subsystem-order-violation"
	RuleInterfaceBypass       = "interface-bypass"
	RuleSubsystemWithoutIface = "subsystem-without-interface"
	RuleForbiddenImport       = "forbidden-import"
	RuleIsolation             = "isolation-violation"
	RuleHiddenDependency      = "hidden-dependency"
	RuleRoleOrder             = "role-order-violation"
	RuleUnknownRole           = "unknown-role-directive"
	RuleSubsystemCycle        = "subsystem-cycle"
	RuleTypeCycleTooLarge     = "type-cycle-too-large"
	RuleLargeFunc             = "large-func"
	RuleComplexFunc           = "complex-func"
	RuleLargeType             = "large-type"
	RuleLargeFile             = "large-file"
	RuleDeepNesting           = "deep-nesting"
	RuleCognitive             = "cognitive-complexity"
	fingerprintBytes          = 8
)

// Severity says whether a finding can fail the gate.
type Severity string

// Finding is one violation of an architecture rule, with the evidence for it
// and a concrete way to fix it.
type Finding struct {
	// Fingerprint identifies the finding across runs. It hashes the rule, the
	// subject and the object, never a line number, so moving code keeps it.
	Fingerprint string   `json:"fingerprint"`
	Rule        string   `json:"rule"`
	Severity    Severity `json:"severity"`
	// Criterion is the MMI criterion the finding counts toward, if any.
	Criterion string `json:"criterion,omitempty"`
	// Subject is what violates the rule: a package, type, function or
	// pattern. Object is what it violates it against, when there is one.
	Subject string `json:"subject"`
	Object  string `json:"object,omitempty"`
	// Location is where to look; a finding about the model itself has none.
	Location *Span    `json:"location,omitempty"`
	Message  string   `json:"message"`
	Fix      string   `json:"fix"`
	Evidence []string `json:"evidence,omitempty"`

	// ModelRule names the rule of the model the finding breaks, such as
	// "layers", "forbiddenImports[2]" or "fitness.layerAccess web", so a
	// reader can find it in anarch.json; the compliance measure counts by it.
	// It is not part of the fingerprint.
	ModelRule string `json:"modelRule,omitempty"`
	// sites are the places besides Location that the finding rests on, such
	// as every import of a subsystem cycle: a change to any of them scopes
	// the finding (docs/adr/0011). Only the gate reads them.
	sites []Span
}

// FindingID is what identifies a finding: the rule it violates, what
// violates it, and what it violates it against, when there is something.
type FindingID struct {
	Rule    string
	Subject string
	Object  string
}

// NewFinding returns a finding with its fingerprint set. The fingerprint
// hashes the ID, never a line number, so moving code keeps it.
func NewFinding(id FindingID, sev Severity) Finding {
	sum := sha256.Sum256([]byte(id.Rule + "\x00" + id.Subject + "\x00" + id.Object))
	return Finding{
		Fingerprint: hex.EncodeToString(sum[:fingerprintBytes]),
		Rule:        id.Rule,
		Severity:    sev,
		Subject:     id.Subject,
		Object:      id.Object,
	}
}

// SpanAt returns the one-line span of a position.
func SpanAt(p Position) *Span {
	return &Span{File: p.File, Start: p.Line, End: p.Line}
}

// touchedBy reports whether a change set scopes the finding: its location or
// one of its sites is on a changed line (docs/adr/0011). A finding with
// neither, such as one about the model, belongs to the whole system and is
// touched by no change.
func (f *Finding) touchedBy(c ChangedLines) bool {
	if c.Touches(f.Location) {
		return true
	}
	for i := range f.sites {
		if c.Touches(&f.sites[i]) {
			return true
		}
	}
	return false
}

// SortFindings orders findings by rule, subject, object and fingerprint, a
// total order because the fingerprint hashes the other three.
func SortFindings(fs []Finding) {
	slices.SortFunc(fs, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(a.Rule, b.Rule),
			cmp.Compare(a.Subject, b.Subject),
			cmp.Compare(a.Object, b.Object),
			cmp.Compare(a.Fingerprint, b.Fingerprint),
		)
	})
}
