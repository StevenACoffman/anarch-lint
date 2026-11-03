package arch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"slices"
)

// modelVersion is the only model-file version this build reads.
const modelVersion = 1

// Cyclomatic-complexity variants a model can choose for criterion 1.3.4.
const (
	CCPlain    CCVariant = "plain"
	CCModified CCVariant = "modified"
	CCExtended CCVariant = "extended"
)

// Default thresholds. None comes from the MMI chapter, which gives none; each
// is borrowed guidance and is reported as an assumption (mmi.md §7.3).
const (
	defaultLargeTypeLines    = 400 // half von Zitzewitz's 800-LoC file limit
	defaultLargeFuncLines    = 60  // archfit's default; von Zitzewitz caps statements at 100
	defaultLargePackageTypes = 40  // the top of Meyer's 5–40 classes per package
	defaultHighComplexity    = 15  // von Zitzewitz's Modified CC limit
	defaultMaxTypeCycleGroup = 5   // von Zitzewitz's largest tolerable cycle group
	defaultLargeFileLines    = 800 // von Zitzewitz's soft limit for a source file
	defaultMaxNesting        = 4   // von Zitzewitz's limit on indentation depth
	defaultHighCognitive     = 15  // SonarSource's default limit for a function
)

// CCVariant names a cyclomatic-complexity variant.
type CCVariant string

// Model is the target architecture (Soll-Architektur): how packages group
// into subsystems and layers, which patterns types play, and which imports
// are forbidden (mmi.md §7.1). The zero Model is valid; every criterion that
// needs a model then reports itself unmeasured.
type Model struct {
	// Schema optionally names the JSON Schema an editor checks the file
	// against (anarch-lint schema prints it); nothing else reads it.
	Schema     string      `json:"$schema,omitempty"`
	Version    int         `json:"version"`
	Subsystems []Subsystem `json:"subsystems,omitempty"`
	// Layers are ordered from the top, the layer that may import every other,
	// to the bottom.
	Layers []Layer `json:"layers,omitempty"`
	// SubsystemOrder optionally orders subsystems from top to bottom, for
	// criterion 2.1.2.
	SubsystemOrder []string `json:"subsystemOrder,omitempty"`
	// Roles are pattern roles, ordered from top to bottom (criterion 3.2).
	Roles            []Role            `json:"roles,omitempty"`
	ForbiddenImports []ForbiddenImport `json:"forbiddenImports,omitempty"`
	Isolations       []Isolation       `json:"isolations,omitempty"`
	// FindingRules gives a gate, a reason and a quality attribute to rules
	// no entry above declares, by the finding's rule, such as
	// "layer-order-violation" or "large-func".
	FindingRules map[RuleName]FindingRule `json:"findingRules,omitempty"`
	Thresholds   Thresholds               `json:"thresholds,omitzero"`
	// ThresholdOverrides give the packages they match other limits than
	// Thresholds: the first that matches a package wins, and a limit it
	// leaves zero is the module's (IMPROVEMENTS.md §1.6). The MMI's measures
	// read only Thresholds.
	ThresholdOverrides []ThresholdOverride `json:"thresholdOverrides,omitempty"`
	// Gates are absolute floors the check gate applies on top of the
	// baseline ratchet. omitzero keeps a model without them hashing as before.
	Gates Gates `json:"gates,omitzero"`
	// Connascence opts the connascence measures into the gate, with the
	// strength and degree each locality allows (connascence.md §1.4).
	Connascence ConnascenceLimits `json:"connascence,omitzero"`
	// Balance opts the Balanced Coupling measures into the gate
	// (connascence.md §9).
	Balance BalanceSettings `json:"balance,omitzero"`
	// Fitness declares ArchUnit-style fitness rules, which opt the fitness
	// findings into the gate.
	Fitness Fitness `json:"fitness,omitzero"`

	// hash is the declared model's hash, set by Expand so an expanded model
	// hashes as the model it came from.
	hash string
}

// ConnascenceLimits are the strongest connascence and the largest degree
// allowed at each locality: "Stronger connascences are more acceptable [sic]
// within a module" (connascence.io). A locality left out takes the default
// of [ConnascenceLimits.Strength]; a degree limit applies only where set.
// Setting either map opts the connascence findings into the gate.
type ConnascenceLimits struct {
	MaxStrength map[Locality]int `json:"maxStrength,omitempty"`
	MaxDegree   map[Locality]int `json:"maxDegree,omitempty"`
}

// BalanceSettings configure the Balanced Coupling measures (connascence.md
// §9).
type BalanceSettings struct {
	// Gate opts the Balanced Coupling findings into the gate; without it they
	// are warnings.
	Gate bool `json:"gate,omitempty"`
	// Messaging lists patterns for message-bus client packages: a package
	// that imports one may integrate asynchronously, which only a reviewer
	// can confirm (connascence.md §9.3).
	Messaging []string `json:"messaging,omitempty"`
	// Packages also pairs the packages within each subsystem, a report of
	// the coupling where distance is low; it never gates.
	Packages bool `json:"packages,omitempty"`
}

// Subsystem is a named group of packages: a domain module in MMI terms.
type Subsystem struct {
	Name string `json:"name"`
	// Packages are import-path patterns: a path, or a path ending in "/..."
	// for it and everything beneath it.
	Packages []string `json:"packages"`
	// Interface lists patterns for the packages other subsystems may import.
	Interface []string `json:"interface,omitempty"`
	// Owner is the team that owns the subsystem. It takes precedence over a
	// CODEOWNERS file for the Balanced Coupling owner boundary.
	Owner string `json:"owner,omitempty"`
}

// Layer is a named, ordered group of packages.
type Layer struct {
	Name     string   `json:"name"`
	Packages []string `json:"packages"`
}

// Role is a pattern role (MMI 3.1). A type plays the role when, checked in
// this order:
//
//   - an "//arch:role NAME" comment on the type names it;
//   - it embeds one of the marker types in Embeds;
//   - it, or a pointer to it, implements one of the interfaces in Implements;
//   - its name ends in Suffix;
//   - its package matches one of Packages.
//
// The first four are explicit marks of the pattern (criterion 3.3); a package
// alone is not. A type that matches several roles plays the first.
type Role struct {
	Name string `json:"name"`
	// Embeds and Implements are type IDs: the package path, a dot, the name.
	Embeds     []string `json:"embeds,omitempty"`
	Implements []string `json:"implements,omitempty"`
	Suffix     string   `json:"suffix,omitempty"`
	Packages   []string `json:"packages,omitempty"`
}

// ForbiddenImport forbids packages matching From to import packages matching
// Imports, which may lie outside the module (MMI 3.4). With Transitive set, a
// package also violates it by reaching a forbidden package through other
// in-scope packages. With Injected set, it also violates it by receiving, as an
// interface argument, a value whose concrete type lives in a forbidden package:
// a dependency it gains at run time without importing anything. Dependency
// inversion injects such values on purpose, so this is opt-in, for
// dependencies that must not exist even at run time.
type ForbiddenImport struct {
	From       []string `json:"from"`
	Imports    []string `json:"imports"`
	Reason     string   `json:"reason"`
	Transitive bool     `json:"transitive,omitempty"`
	Injected   bool     `json:"injected,omitempty"`
	// Tests checks the imports of the packages' test files too.
	Tests bool `json:"tests,omitempty"`
	// Gate overrides the error severity of the rule's findings; off checks
	// nothing.
	Gate             RuleGate         `json:"gate,omitempty"`
	QualityAttribute QualityAttribute `json:"qualityAttribute,omitempty"`
}

// Isolation limits what packages matching From may import within the
// analysis: only packages matching one of MayImport, after "$1" to "$9" are
// replaced by From's captures. For example, From "example.com/app/$*/..." with
// MayImport ["example.com/app/$1/...", "example.com/app/shared/..."] keeps
// every top-level area to itself and the shared code.
type Isolation struct {
	From      string   `json:"from"`
	MayImport []string `json:"mayImport"`
	Reason    string   `json:"reason"`
	// Tests checks the imports of the packages' test files too, so a test
	// cannot reach into another area's internals.
	Tests bool `json:"tests,omitempty"`
	// Gate overrides the error severity of the rule's findings; off checks
	// nothing.
	Gate             RuleGate         `json:"gate,omitempty"`
	QualityAttribute QualityAttribute `json:"qualityAttribute,omitempty"`
}

// Gates are arch-go's coverage and compliance floors, as percentages; zero
// leaves a floor unset.
type Gates struct {
	// MinCoverage is the least share of packages a subsystem must claim.
	MinCoverage float64 `json:"minCoverage,omitempty"`
	// MinCompliance is the least share of declared rules with no error.
	MinCompliance float64 `json:"minCompliance,omitempty"`
}

// ThresholdOverride is the limits for the packages matching Packages.
type ThresholdOverride struct {
	Packages   []string   `json:"packages"`
	Thresholds Thresholds `json:"thresholds"`
}

// Thresholds are the limits the size and complexity measures use. A zero
// value takes the default.
type Thresholds struct {
	LargeTypeLines    int       `json:"largeTypeLines,omitempty"`
	LargeFuncLines    int       `json:"largeFuncLines,omitempty"`
	LargePackageTypes int       `json:"largePackageTypes,omitempty"`
	HighComplexity    int       `json:"highComplexity,omitempty"`
	ComplexityVariant CCVariant `json:"complexityVariant,omitempty"`
	MaxTypeCycleGroup int       `json:"maxTypeCycleGroup,omitempty"`
	LargeFileLines    int       `json:"largeFileLines,omitempty"`
	MaxNesting        int       `json:"maxNesting,omitempty"`
	HighCognitive     int       `json:"highCognitive,omitempty"`
}

// DecodeModel reads a model file. Unknown fields, a version other than 1,
// duplicate names, negative thresholds and an unknown complexity variant are
// all EINVALID, with a message naming the problem.
func DecodeModel(r io.Reader) (Model, error) {
	const op = "arch.DecodeModel"
	var m Model
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		// A field's own UnmarshalText reports an Error; keep its message
		// rather than the code-prefixed text.
		code, msg := ProblemMalformed, err.Error()
		if ErrorCode(err) == EINVALID {
			code, msg = ProblemValue, ErrorMessage(err)
		}
		return Model{}, &Error{Op: op, Err: InvalidModel(code, "%s", msg)}
	}
	if err := m.validate(); err != nil {
		return Model{}, &Error{Op: op, Err: err}
	}
	return m, nil
}

// Hash returns a fingerprint of the model: the sha256 of its canonical JSON,
// in which a change that keeps the model's meaning changes nothing
// (IMPROVEMENTS.md §1.4): the order of a set, the $schema key, and a
// threshold set to its default. None of Model's methods modifies it.
// A baseline is comparable only with a report whose model hash matches.
func (m *Model) Hash() string {
	if m.hash != "" {
		return m.hash
	}
	var buf bytes.Buffer
	_ = WriteJSON(&buf, m.canonical()) // a Model always encodes
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])
}

// PackageLimits returns the limits of each package: the first override
// that matches it, with the module's limits for whatever it leaves zero, or
// the module's limits.
func (m *Model) PackageLimits(packages []string) map[string]Thresholds {
	base := m.Limits()
	out := make(map[string]Thresholds, len(packages))
	for _, pkg := range packages {
		out[pkg] = base
		for i := range m.ThresholdOverrides {
			if o := &m.ThresholdOverrides[i]; MatchesAny(o.Packages, pkg) {
				out[pkg] = o.Thresholds.over(&base)
				break
			}
		}
	}
	return out
}

// over returns base with every limit t sets replaced.
func (t *Thresholds) over(base *Thresholds) Thresholds {
	out := *base
	for _, l := range []struct {
		dst *int
		src int
	}{
		{&out.LargeTypeLines, t.LargeTypeLines},
		{&out.LargeFuncLines, t.LargeFuncLines},
		{&out.LargePackageTypes, t.LargePackageTypes},
		{&out.HighComplexity, t.HighComplexity},
		{&out.MaxTypeCycleGroup, t.MaxTypeCycleGroup},
		{&out.LargeFileLines, t.LargeFileLines},
		{&out.MaxNesting, t.MaxNesting},
		{&out.HighCognitive, t.HighCognitive},
	} {
		if l.src != 0 {
			*l.dst = l.src
		}
	}
	if t.ComplexityVariant != "" {
		out.ComplexityVariant = t.ComplexityVariant
	}
	return out
}

// Limits returns the thresholds with every default filled in.
func (m *Model) Limits() Thresholds {
	t := m.Thresholds
	t.LargeTypeLines = cmpOr(t.LargeTypeLines, defaultLargeTypeLines)
	t.LargeFuncLines = cmpOr(t.LargeFuncLines, defaultLargeFuncLines)
	t.LargePackageTypes = cmpOr(t.LargePackageTypes, defaultLargePackageTypes)
	t.HighComplexity = cmpOr(t.HighComplexity, defaultHighComplexity)
	t.MaxTypeCycleGroup = cmpOr(t.MaxTypeCycleGroup, defaultMaxTypeCycleGroup)
	t.LargeFileLines = cmpOr(t.LargeFileLines, defaultLargeFileLines)
	t.MaxNesting = cmpOr(t.MaxNesting, defaultMaxNesting)
	t.HighCognitive = cmpOr(t.HighCognitive, defaultHighCognitive)
	if t.ComplexityVariant == "" {
		t.ComplexityVariant = CCModified
	}
	return t
}

// canonical returns the model as its hash reads it: with no $schema, the
// thresholds with their defaults filled in, and every list in a sorted
// order, except the lists whose order means something ([ordered]).
func (m *Model) canonical() any {
	c := *m
	c.Schema, c.Thresholds = "", m.Limits()
	src, _ := json.Marshal(&c) // a Model always encodes
	var doc any
	_ = json.Unmarshal(src, &doc)
	return sortSets(doc, "")
}

// sortSets sorts every list in a decoded JSON document by its elements'
// encodings, except a list under a key [ordered] names, and returns it.
func sortSets(v any, key string) any {
	switch v := v.(type) {
	case map[string]any:
		for k, x := range v {
			v[k] = sortSets(x, k)
		}
	case []any:
		for i := range v {
			v[i] = sortSets(v[i], "")
		}
		if !ordered(key) {
			slices.SortStableFunc(v, func(a, b any) int {
				ja, _ := json.Marshal(a)
				jb, _ := json.Marshal(b)
				return bytes.Compare(ja, jb)
			})
		}
	}
	return v
}

// ordered reports a list of the model whose order means something: the
// layers, the subsystem order and the roles, each from the top down, and the
// threshold overrides and a protobuf rule's import paths, where the first
// match wins.
func ordered(key string) bool {
	return key == "layers" || key == "subsystemOrder" || key == "roles" ||
		key == "thresholdOverrides" || key == "importPaths"
}

func (m *Model) validate() error {
	invalid := InvalidModel
	if m.Version != modelVersion {
		return invalid(
			ProblemVersion,
			"version %d is not supported; want %d",
			m.Version,
			modelVersion,
		)
	}
	// In a fixed order, so a model with several problems always reports the
	// same one first.
	for _, check := range []func(invalidFunc) error{
		m.validateNames, m.validateReferences, m.validateGates, m.validateFindingRules,
		m.Thresholds.validate, m.validateOverrides,
		m.Connascence.validate, m.Balance.validate,
		func(invalid invalidFunc) error {
			return m.Fitness.validate(declaredNames{
				layers:     namesOf(m.Layers, func(l Layer) string { return l.Name }),
				subsystems: namesOf(m.Subsystems, func(s Subsystem) string { return s.Name }),
			}, invalid)
		},
	} {
		if err := check(invalid); err != nil {
			return err
		}
	}
	return nil
}

// validateNames rejects a subsystem, layer or role declared twice.
func (m *Model) validateNames(invalid invalidFunc) error {
	for _, set := range []struct {
		kind  string
		names []string
	}{
		{"subsystem", namesOf(m.Subsystems, func(s Subsystem) string { return s.Name })},
		{"layer", namesOf(m.Layers, func(l Layer) string { return l.Name })},
		{"role", namesOf(m.Roles, func(r Role) string { return r.Name })},
	} {
		if dup := firstDuplicate(set.names); dup != "" {
			return invalid(ProblemDuplicate, "%s %q is declared twice", set.kind, dup)
		}
	}
	return nil
}

// validateReferences rejects a back-reference past an isolation's captures,
// and a subsystem order naming an undeclared subsystem.
func (m *Model) validateReferences(invalid invalidFunc) error {
	for _, iso := range m.Isolations {
		for _, pat := range iso.MayImport {
			if ref, n := highestReference(pat), captureCount(iso.From); ref > n {
				return invalid(
					ProblemReference,
					"isolation %q: mayImport %q refers to $%d, but from captures %d",
					iso.From,
					pat,
					ref,
					n,
				)
			}
		}
	}
	for _, sub := range m.Subsystems {
		if err := sub.validateTemplate(invalid); err != nil {
			return err
		}
	}
	for _, name := range m.SubsystemOrder {
		if !slices.ContainsFunc(m.Subsystems, func(s Subsystem) bool { return s.Name == name }) {
			return invalid(ProblemReference, "subsystemOrder names unknown subsystem %q", name)
		}
	}
	return nil
}

// validateTemplate rejects a subsystem template whose name, interface or
// owner refers to a capture one of its package patterns lacks, and a
// reference in the interface or owner of a subsystem that is no template.
func (s *Subsystem) validateTemplate(invalid invalidFunc) error {
	refs := highestReference(s.Name)
	for _, pat := range slices.Concat(s.Interface, []string{s.Owner}) {
		if r := highestReference(pat); r > 0 && refs == 0 {
			return invalid(ProblemReference, "subsystem %q refers to $%d in %q, but its name "+
				"refers to no capture, so it is not a template", s.Name, r, pat)
		}
		refs = max(refs, highestReference(pat))
	}
	if refs == 0 {
		return nil
	}
	for _, pat := range s.Packages {
		if n := captureCount(pat); n < refs {
			return invalid(
				ProblemReference,
				"subsystem %q refers to $%d, but package pattern %q captures %d",
				s.Name,
				refs,
				pat,
				n,
			)
		}
	}
	if len(s.Packages) == 0 {
		return invalid(
			ProblemReference,
			"subsystem %q refers to $%d, but has no package pattern",
			s.Name,
			refs,
		)
	}
	return nil
}

// validateOverrides rejects an override that names no packages, and one
// whose limits Thresholds would reject.
func (m *Model) validateOverrides(invalid invalidFunc) error {
	for i := range m.ThresholdOverrides {
		o := &m.ThresholdOverrides[i]
		if len(o.Packages) == 0 {
			return invalid(ProblemIncomplete, "thresholdOverrides[%d] names no packages", i)
		}
		if err := o.Thresholds.validate(invalid); err != nil {
			return err
		}
	}
	return nil
}

// validateGates rejects a floor outside 0–100.
func (m *Model) validateGates(invalid invalidFunc) error {
	for _, g := range []struct {
		name  string
		value float64
	}{{"minCoverage", m.Gates.MinCoverage}, {"minCompliance", m.Gates.MinCompliance}} {
		if g.value < 0 || g.value > 100 {
			return invalid(ProblemRange, "gate %s %g is outside 0–100", g.name, g.value)
		}
	}
	return nil
}

func (t *Thresholds) validate(invalid invalidFunc) error {
	for _, limit := range []struct {
		name  string
		value int
	}{
		{"largeTypeLines", t.LargeTypeLines},
		{"largeFuncLines", t.LargeFuncLines},
		{"largePackageTypes", t.LargePackageTypes},
		{"highComplexity", t.HighComplexity},
		{"maxTypeCycleGroup", t.MaxTypeCycleGroup},
		{"largeFileLines", t.LargeFileLines},
		{"maxNesting", t.MaxNesting},
		{"highCognitive", t.HighCognitive},
	} {
		if limit.value < 0 {
			return invalid(ProblemRange, "threshold %s is negative", limit.name)
		}
	}
	switch t.ComplexityVariant {
	case "", CCPlain, CCModified, CCExtended:
		return nil
	default:
		return invalid(ProblemValue,
			"complexityVariant %q is not plain, modified or extended",
			t.ComplexityVariant,
		)
	}
}

// StrengthLimit returns the strongest connascence allowed at locality l: the
// model's limit, or the default. The defaults allow every kind up to the
// package, then follow connascence.md §1.4: algorithm across subsystems,
// meaning across a module, type beyond it.
func (c *ConnascenceLimits) StrengthLimit(l Locality) int {
	if v, ok := c.MaxStrength[l]; ok {
		return v
	}
	switch l {
	case LocalityFunction, LocalityType, LocalityFile, LocalityPackage:
		return int(StrengthIdentity)
	case LocalitySubsystem:
		return int(StrengthAlgorithm)
	case LocalitySystem:
		return int(StrengthMeaning)
	default:
		return int(StrengthType)
	}
}

// DegreeLimit returns the largest degree allowed at locality l, and false when no
// limit is set there.
func (c *ConnascenceLimits) DegreeLimit(l Locality) (int, bool) {
	v, ok := c.MaxDegree[l]
	return v, ok
}

// Enabled reports whether the model opts the connascence findings into the
// gate, by setting either limit.
func (c *ConnascenceLimits) Enabled() bool {
	return len(c.MaxStrength) > 0 || len(c.MaxDegree) > 0
}

// validate rejects a strength limit outside 1–9 and a degree limit below 1.
func (c ConnascenceLimits) validate(invalid invalidFunc) error {
	for _, l := range slices.Sorted(maps.Keys(c.MaxStrength)) {
		if v := c.MaxStrength[l]; v < 1 || v > int(StrengthIdentity) {
			return invalid(ProblemRange, "connascence maxStrength %s %d is outside 1–9", l, v)
		}
	}
	for _, l := range slices.Sorted(maps.Keys(c.MaxDegree)) {
		if v := c.MaxDegree[l]; v < 1 {
			return invalid(ProblemRange, "connascence maxDegree %s %d is below 1", l, v)
		}
	}
	return nil
}

// Enabled reports whether the model opts the Balanced Coupling findings into
// the gate.
func (b *BalanceSettings) Enabled() bool { return b.Gate }

// validate rejects an empty messaging pattern.
func (b BalanceSettings) validate(invalid invalidFunc) error {
	if slices.Contains(b.Messaging, "") {
		return invalid(ProblemPattern, "balance messaging has an empty pattern")
	}
	return nil
}

// namesOf returns the name of every element of xs.
func namesOf[T any](xs []T, name func(T) string) []string {
	names := make([]string, len(xs))
	for i, x := range xs {
		names[i] = name(x)
	}
	return names
}

// firstDuplicate returns the smallest name that occurs twice, or "".
func firstDuplicate(names []string) string {
	sorted := slices.Sorted(slices.Values(names))
	for i := 1; i < len(sorted); i++ {
		if sorted[i] == sorted[i-1] {
			return sorted[i]
		}
	}
	return ""
}

// cmpOr returns v when it is set, and def otherwise.
func cmpOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
