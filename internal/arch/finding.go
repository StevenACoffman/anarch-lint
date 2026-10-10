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

	// instance names the declared rule the finding violates, such as
	// "layers" or "forbiddenImports[2]", for the compliance measure. It is
	// unexported so it changes neither the JSON nor the fingerprint.
	instance string
}

// newFinding returns a finding with its fingerprint set.
func newFinding(rule string, sev Severity, subject, object string) Finding {
	sum := sha256.Sum256([]byte(rule + "\x00" + subject + "\x00" + object))
	return Finding{
		Fingerprint: hex.EncodeToString(sum[:fingerprintBytes]),
		Rule:        rule,
		Severity:    sev,
		Subject:     subject,
		Object:      object,
	}
}

// spanAt returns the one-line span of a position.
func spanAt(p Position) *Span {
	return &Span{File: p.File, Start: p.Line, End: p.Line}
}

// sortFindings orders findings by rule, subject, object and fingerprint, a
// total order because the fingerprint hashes the other three.
func sortFindings(fs []Finding) {
	slices.SortFunc(fs, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(a.Rule, b.Rule),
			cmp.Compare(a.Subject, b.Subject),
			cmp.Compare(a.Object, b.Object),
			cmp.Compare(a.Fingerprint, b.Fingerprint),
		)
	})
}
