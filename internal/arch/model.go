package arch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
)

// CCVariant names a cyclomatic-complexity variant.
type CCVariant string

// Model is the target architecture (Soll-Architektur): how packages group
// into subsystems and layers, which patterns types play, and which imports
// are forbidden (mmi.md §7.1). The zero Model is valid; every criterion that
// needs a model then reports itself unmeasured.
type Model struct {
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
	Thresholds       Thresholds        `json:"thresholds"`
	// Gates are absolute floors the check gate applies on top of the
	// baseline ratchet. omitzero keeps a model without them hashing as before.
	Gates Gates `json:"gates,omitzero"`
}

// Subsystem is a named group of packages: a domain module in MMI terms.
type Subsystem struct {
	Name string `json:"name"`
	// Packages are import-path patterns: a path, or a path ending in "/..."
	// for it and everything beneath it.
	Packages []string `json:"packages"`
	// Interface lists patterns for the packages other subsystems may import.
	Interface []string `json:"interface,omitempty"`
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
}

// Gates are arch-go's coverage and compliance floors, as percentages; zero
// leaves a floor unset.
type Gates struct {
	// MinCoverage is the least share of packages a subsystem must claim.
	MinCoverage float64 `json:"minCoverage,omitempty"`
	// MinCompliance is the least share of declared rules with no error.
	MinCompliance float64 `json:"minCompliance,omitempty"`
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
		return Model{}, &Error{
			Op:  op,
			Err: &Error{Code: EINVALID, Message: "model: " + err.Error()},
		}
	}
	if err := m.validate(); err != nil {
		return Model{}, &Error{Op: op, Err: err}
	}
	return m, nil
}

// Hash returns a fingerprint of the model: the sha256 of its canonical JSON.
// None of Model's methods modifies it.
// A baseline is comparable only with a report whose model hash matches.
func (m *Model) Hash() string {
	var buf bytes.Buffer
	_ = WriteJSON(&buf, m) // a Model always encodes
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])
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
	if t.ComplexityVariant == "" {
		t.ComplexityVariant = CCModified
	}
	return t
}

func (m *Model) validate() error {
	invalid := func(format string, args ...any) error {
		return &Error{Code: EINVALID, Message: "model: " + fmt.Sprintf(format, args...)}
	}
	if m.Version != modelVersion {
		return invalid("version %d is not supported; want %d", m.Version, modelVersion)
	}
	// In a fixed order, so a model with several problems always reports the
	// same one first.
	for _, check := range []func(func(string, ...any) error) error{
		m.validateNames, m.validateReferences, m.validateGates, m.Thresholds.validate,
	} {
		if err := check(invalid); err != nil {
			return err
		}
	}
	return nil
}

// validateNames rejects a subsystem, layer or role declared twice.
func (m *Model) validateNames(invalid func(string, ...any) error) error {
	for _, set := range []struct {
		kind  string
		names []string
	}{
		{"subsystem", namesOf(m.Subsystems, func(s Subsystem) string { return s.Name })},
		{"layer", namesOf(m.Layers, func(l Layer) string { return l.Name })},
		{"role", namesOf(m.Roles, func(r Role) string { return r.Name })},
	} {
		if dup := firstDuplicate(set.names); dup != "" {
			return invalid("%s %q is declared twice", set.kind, dup)
		}
	}
	return nil
}

// validateReferences rejects a back-reference past an isolation's captures,
// and a subsystem order naming an undeclared subsystem.
func (m *Model) validateReferences(invalid func(string, ...any) error) error {
	for _, iso := range m.Isolations {
		for _, pat := range iso.MayImport {
			if ref, n := highestReference(pat), captureCount(iso.From); ref > n {
				return invalid("isolation %q: mayImport %q refers to $%d, but from captures %d",
					iso.From, pat, ref, n)
			}
		}
	}
	for _, name := range m.SubsystemOrder {
		if !slices.ContainsFunc(m.Subsystems, func(s Subsystem) bool { return s.Name == name }) {
			return invalid("subsystemOrder names unknown subsystem %q", name)
		}
	}
	return nil
}

// validateGates rejects a floor outside 0–100.
func (m *Model) validateGates(invalid func(string, ...any) error) error {
	for _, g := range []struct {
		name  string
		value float64
	}{{"minCoverage", m.Gates.MinCoverage}, {"minCompliance", m.Gates.MinCompliance}} {
		if g.value < 0 || g.value > 100 {
			return invalid("gate %s %g is outside 0–100", g.name, g.value)
		}
	}
	return nil
}

func (t Thresholds) validate(invalid func(string, ...any) error) error {
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
	} {
		if limit.value < 0 {
			return invalid("threshold %s is negative", limit.name)
		}
	}
	switch t.ComplexityVariant {
	case "", CCPlain, CCModified, CCExtended:
		return nil
	default:
		return invalid(
			"complexityVariant %q is not plain, modified or extended",
			t.ComplexityVariant,
		)
	}
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
