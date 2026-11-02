package arch

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Gates of a fitness rule: what a violation does.
const (
	// GateFail makes a violation an error-severity finding, which gates.
	GateFail RuleGate = "fail"
	// GateWarn makes it a warning, which is advice.
	GateWarn RuleGate = "warn"
	// GateOff reports the rule as off and checks nothing.
	GateOff RuleGate = "off"
)

// RuleGate says what a fitness rule's violations do; empty means GateFail.
type RuleGate string

// Fitness declares ArchUnit-style fitness rules (TODO.md, "Ideas from
// ArchUnit"): access between layers, an onion architecture, slices of the
// module, who may depend on a package, which functions may be called, and
// which types may be used. The zero value declares nothing, and a model
// without it hashes as before.
type Fitness struct {
	// RequireAssigned makes a package outside every subsystem, or outside
	// every layer when layers exist, a violation (ArchUnit's
	// ensureAllClassesAreContainedInArchitecture).
	RequireAssigned bool `json:"requireAssigned,omitempty"`
	// StrictLayers lets each layer import only the one directly below it in
	// the model's order.
	StrictLayers   bool             `json:"strictLayers,omitempty"`
	LayerAccess    []LayerAccess    `json:"layerAccess,omitempty"`
	Onion          Onion            `json:"onion,omitzero"`
	Slices         []Slices         `json:"slices,omitempty"`
	Dependents     []Dependents     `json:"dependents,omitempty"`
	Libraries      []LibraryRule    `json:"libraries,omitempty"`
	Calls          []CallRule       `json:"calls,omitempty"`
	Fields         []FieldRule      `json:"fields,omitempty"`
	UnusedExports  []UnusedExports  `json:"unusedExports,omitempty"`
	InterfacesOnly []InterfacesOnly `json:"interfacesOnly,omitempty"`
	// CoChange rules name files that must change together; they check
	// only a change set, as check --new-from-rev gives one.
	CoChange []CoChangeRule `json:"coChange,omitempty"`
	FanOut   []FanOutRule   `json:"fanOut,omitempty"`
	// Domains compare modelith domain models with the code that should
	// express them (mmi-plan.md §10).
	Domains []DomainRule `json:"domains,omitempty"`
	// OpenAPI compares OpenAPI documents with the routes the code serves.
	OpenAPI []OpenAPIRule `json:"openapi,omitempty"`
}

// OpenAPIRule compares one OpenAPI document with the HTTP routes the code
// registers (IMPROVEMENTS.md §5.1): every operation it documents should be
// served, and every route the serving packages register documented.
type OpenAPIRule struct {
	RuleOptions
	// Path is the OpenAPI 3 document, YAML or JSON, relative to the analysis
	// root; it and every file it refers to must lie within the repository.
	Path string `json:"path"`
	// ServedBy are the packages whose routes the document describes; empty
	// means every package under analysis.
	ServedBy []string `json:"servedBy,omitempty"`
}

// DomainRule compares one modelith domain model with the code: every entity
// should be a Go type, every relationship a reference between types, and
// every enum a Go type with the same values.
type DomainRule struct {
	RuleOptions
	// Path is the *.modelith.yaml file, relative to the analysis root; it
	// must lie within the repository.
	Path string `json:"path"`
	// Subsystem is the subsystem whose code the model describes; empty
	// means the whole program.
	Subsystem string `json:"subsystem,omitempty"`
	// Types maps an entity or enum to its Go type ID where the names differ
	// on purpose, as in {"Finding": "example.com/x/finding.Diagnostic"}.
	Types map[string]string `json:"types,omitempty"`
}

// RuleOptions are what every fitness rule carries: a name for its findings,
// the reason it exists, its gate, and whether it may match nothing.
type RuleOptions struct {
	Name   string   `json:"name,omitempty"`
	Reason string   `json:"reason,omitempty"`
	Gate   RuleGate `json:"gate,omitempty"`
	// Optional lets the rule's selectors match nothing; otherwise an empty
	// rule is a finding, as in ArchUnit, where failOnEmptyShould defaults to
	// true.
	Optional bool `json:"optional,omitempty"`
	// QualityAttribute is the quality the rule protects.
	QualityAttribute QualityAttribute `json:"qualityAttribute,omitempty"`
}

// DeclaredRule is one fitness rule that carries options, with its kind and
// its name.
type DeclaredRule struct {
	Kind string
	Name string
	RuleOptions
}

// LayerAccess restricts one layer's incoming and outgoing imports by
// allow-lists; a layer may always use itself (ArchUnit's
// LayerDependencySpecification).
type LayerAccess struct {
	RuleOptions
	Layer                 string   `json:"layer"`
	MayOnlyBeAccessedBy   []string `json:"mayOnlyBeAccessedBy,omitempty"`
	MayNotBeAccessedByAny bool     `json:"mayNotBeAccessedByAny,omitempty"`
	MayOnlyAccess         []string `json:"mayOnlyAccess,omitempty"`
	MayNotAccessAny       bool     `json:"mayNotAccessAny,omitempty"`
	// ConsideringAll also counts imports of in-scope packages that have no
	// layer; by default only imports between two layered packages count.
	ConsideringAll bool `json:"consideringAll,omitempty"`
}

// Onion is ArchUnit's onionArchitecture: each ring may import only itself
// and the rings inside it, and no adapter may import another adapter.
type Onion struct {
	RuleOptions
	DomainModels        []string            `json:"domainModels,omitempty"`
	DomainServices      []string            `json:"domainServices,omitempty"`
	ApplicationServices []string            `json:"applicationServices,omitempty"`
	Adapters            map[string][]string `json:"adapters,omitempty"`
}

// Slices groups packages by the capture of a pattern, such as
// "example.com/app/$*/...", one group per captured value (ArchUnit's
// slices().matching).
type Slices struct {
	RuleOptions
	Pattern              string `json:"pattern"`
	FreeOfCycles         bool   `json:"freeOfCycles,omitempty"`
	NotDependOnEachOther bool   `json:"notDependOnEachOther,omitempty"`
}

// Dependents declares who may import a package, from the imported side
// (ArchUnit's onlyBeAccessedByAnyPackage).
type Dependents struct {
	RuleOptions
	Packages []string `json:"packages"`
	OnlyFrom []string `json:"onlyFrom,omitempty"`
}

// LibraryRule is an allow-list of third-party code: packages matching From
// may import, beyond the standard library and the packages under analysis,
// only packages matching Allow. Without Allow they may import none.
type LibraryRule struct {
	RuleOptions
	From  []string `json:"from"`
	Allow []string `json:"allow,omitempty"`
}

// CallRule restricts calls of functions and methods (ArchUnit's callMethod
// and onlyBeCalled). A callee is a full name, such as "time.Now" or
// "(*os.File).Write", with a method of a generic type named by the type
// alone, "(*pkg.List).Push", or "pkg.*" for every function and method of a
// package.
// Without Only, packages matching From may not call the callees; with it,
// only they may.
type CallRule struct {
	RuleOptions
	From    []string `json:"from,omitempty"`
	Callees []string `json:"callees"`
	Only    bool     `json:"only,omitempty"`
}

// FieldRule restricts reading and writing struct fields (ArchUnit's
// accessField, getField and setField). A field is "pkg.Type.Field",
// "pkg.Type.*" for every field of a type, or "pkg.*" for every field of a
// package's types. With Writes, only writes count: an assignment, an
// increment, or a keyed composite literal that sets the field. Without Only,
// packages matching From may not access the fields; with it, only they may.
type FieldRule struct {
	RuleOptions
	From   []string `json:"from,omitempty"`
	Fields []string `json:"fields"`
	Writes bool     `json:"writes,omitempty"`
	Only   bool     `json:"only,omitempty"`
}

// UnusedExports reports each exported package-level type, function and
// constant of the packages matching Packages that no other package under
// analysis uses, nor an external test: API that could be unexported, which
// inflates the visibility metrics (IMPROVEMENTS.md §9.2). A method is never
// reported, since an interface may need it, nor anything in package main.
type UnusedExports struct {
	RuleOptions
	Packages []string `json:"packages"`
}

// InterfacesOnly lets packages matching From use only the interface types of
// packages matching To.
type InterfacesOnly struct {
	RuleOptions
	From []string `json:"from"`
	To   []string `json:"to"`
}

// CoChangeRule requires that a change touching a file matching When, and not
// Except, also touch a file matching Require (gortex's co-change guard). The
// patterns are globs over file paths relative to the analysis root, in
// github.com/bmatcuk/doublestar's syntax: "*" stays within a directory and
// "**" crosses directories, as in "api/**/*.proto". A change touches a file
// when it adds or modifies a line of it; deleting a file touches nothing.
type CoChangeRule struct {
	RuleOptions
	When    []string `json:"when"`
	Require []string `json:"require"`
	Except  []string `json:"except,omitempty"`
}

// FanOutRule caps the number of packages under analysis that a package
// matching Packages may import: its efferent coupling, Martin's Ce. A package
// that depends on many others changes whenever any of them does.
type FanOutRule struct {
	RuleOptions
	Packages []string `json:"packages"`
	Max      int      `json:"max"`
}

// declaredNames are the model's layers and subsystems, in order: all of the
// model a fitness rule may refer to.
type declaredNames struct {
	layers, subsystems []string
}

// Enabled reports whether the model declares any fitness rule, which opts the
// fitness findings into the gate.
func (f *Fitness) Enabled() bool {
	return f.RequireAssigned || f.StrictLayers || len(f.Rules()) > 0
}

// Declared reports whether the onion names any ring.
func (o *Onion) Declared() bool {
	return len(o.DomainModels)+len(o.DomainServices)+len(o.ApplicationServices)+len(o.Adapters) > 0
}

// Severity returns the severity of a violation under the gate, and false
// when the rule is off.
func (g RuleGate) Severity() (Severity, bool) {
	switch g {
	case GateWarn:
		return SeverityWarning, true
	case GateOff:
		return "", false
	default:
		return SeverityError, true
	}
}

// Rules returns every fitness rule that carries options, in declaration
// order, the onion first when it is declared. Each is named by its own Name,
// or by its kind and position, such as "slices[0]".
func (f *Fitness) Rules() []DeclaredRule {
	var out []DeclaredRule
	add := func(kind string, opts []RuleOptions) {
		for i, o := range opts {
			out = append(
				out,
				DeclaredRule{Kind: kind, Name: DefaultRuleName(o.Name, kind, i), RuleOptions: o},
			)
		}
	}
	if f.Onion.Declared() {
		add("onion", []RuleOptions{f.Onion.RuleOptions})
	}
	add(
		"layerAccess",
		optionsOf(f.LayerAccess, func(r LayerAccess) RuleOptions { return r.RuleOptions }),
	)
	add("slices", optionsOf(f.Slices, func(r Slices) RuleOptions { return r.RuleOptions }))
	add(
		"dependents",
		optionsOf(f.Dependents, func(r Dependents) RuleOptions { return r.RuleOptions }),
	)
	add(
		"libraries",
		optionsOf(f.Libraries, func(r LibraryRule) RuleOptions { return r.RuleOptions }),
	)
	add("calls", optionsOf(f.Calls, func(r CallRule) RuleOptions { return r.RuleOptions }))
	add("fields", optionsOf(f.Fields, func(r FieldRule) RuleOptions { return r.RuleOptions }))
	add("unusedExports",
		optionsOf(f.UnusedExports, func(r UnusedExports) RuleOptions { return r.RuleOptions }))
	add(
		"interfacesOnly",
		optionsOf(f.InterfacesOnly, func(r InterfacesOnly) RuleOptions { return r.RuleOptions }),
	)
	add("domains", optionsOf(f.Domains, func(r DomainRule) RuleOptions { return r.RuleOptions }))
	add("openapi", optionsOf(f.OpenAPI, func(r OpenAPIRule) RuleOptions { return r.RuleOptions }))
	add(
		"coChange",
		optionsOf(f.CoChange, func(r CoChangeRule) RuleOptions { return r.RuleOptions }),
	)
	add("fanOut", optionsOf(f.FanOut, func(r FanOutRule) RuleOptions { return r.RuleOptions }))
	return out
}

// DefaultRuleName is a rule's name, or its kind and position when it has none.
func DefaultRuleName(name, kind string, i int) string {
	if name != "" {
		return name
	}
	return kind + "[" + strconv.Itoa(i) + "]"
}

// optionsOf returns the options of each rule.
func optionsOf[T any](rules []T, opts func(T) RuleOptions) []RuleOptions {
	out := make([]RuleOptions, len(rules))
	for i, r := range rules {
		out[i] = opts(r)
	}
	return out
}

// validate rejects an unknown gate, layer or subsystem, a duplicate name, a
// slice pattern without a capture, and a rule without the selectors it needs.
func (f *Fitness) validate(names declaredNames, invalid invalidFunc) error {
	for _, check := range []func(declaredNames, invalidFunc) error{
		f.validateNames, f.validateLayers, f.validateSelectors,
	} {
		if err := check(names, invalid); err != nil {
			return err
		}
	}
	return nil
}

// validateNames rejects two rules of one name and an unknown gate.
func (f *Fitness) validateNames(_ declaredNames, invalid invalidFunc) error {
	rules := f.Rules()
	all := make([]string, len(rules))
	for i := range rules {
		all[i] = rules[i].Name
	}
	if dup := firstDuplicate(all); dup != "" {
		return invalid(ProblemDuplicate, "fitness rule %q is declared twice", dup)
	}
	for i := range rules {
		switch o := &rules[i]; o.Gate {
		case "", GateFail, GateWarn, GateOff:
		default:
			return invalid(ProblemValue, "fitness gate %q is not fail, warn or off", o.Gate)
		}
	}
	return nil
}

// validateLayers rejects a layer access rule that names an undeclared layer.
func (f *Fitness) validateLayers(names declaredNames, invalid invalidFunc) error {
	layers := names.layers
	declared := map[string]bool{}
	for _, l := range layers {
		declared[l] = true
	}
	for i := range f.LayerAccess {
		la := &f.LayerAccess[i]
		for _, name := range slices.Concat([]string{la.Layer}, la.MayOnlyBeAccessedBy, la.MayOnlyAccess) {
			if !declared[name] {
				return invalid(ProblemReference, "fitness layerAccess names unknown layer %q", name)
			}
		}
	}
	if f.StrictLayers && len(layers) < 2 {
		return invalid(ProblemIncomplete, "fitness strictLayers needs at least two layers")
	}
	return nil
}

// validateSelectors rejects a rule without the selectors it needs, and a
// domains rule naming an unknown subsystem.
func (f *Fitness) validateSelectors(names declaredNames, invalid invalidFunc) error {
	problems := slices.Concat(
		problemsOf(f.Slices),
		problemsOf(f.Dependents),
		problemsOf(f.Libraries),
		problemsOf(f.Calls),
		problemsOf(f.Fields),
		problemsOf(f.UnusedExports),
		problemsOf(f.InterfacesOnly),
		problemsOf(f.CoChange),
		problemsOf(f.FanOut),
		problemsOf(f.Domains),
		problemsOf(f.OpenAPI),
	)
	for i := range f.Domains {
		if sub := f.Domains[i].Subsystem; sub != "" && !slices.Contains(names.subsystems, sub) {
			problems = append(problems, ruleProblem{ProblemReference, fmt.Sprintf(
				"fitness domains rule %q names unknown subsystem %q", f.Domains[i].Name, sub)})
		}
	}
	for _, p := range problems {
		if p.code != "" {
			return invalid(p.code, "%s", p.text)
		}
	}
	return nil
}

// problemsOf returns what makes each rule unusable, the zero value for a
// usable one.
func problemsOf[T any, R interface {
	*T
	problem() ruleProblem
}](rules []T) []ruleProblem {
	out := make([]ruleProblem, len(rules))
	for i := range rules {
		out[i] = R(&rules[i]).problem()
	}
	return out
}

// problem says what makes the rule unusable, if anything.
func (s *Slices) problem() ruleProblem {
	if captureCount(s.Pattern) == 0 {
		return ruleProblem{ProblemPattern, fmt.Sprintf(
			"fitness slices pattern %q captures nothing; use $* or $**", s.Pattern)}
	}
	return ruleProblem{}
}

// problem says what makes the rule unusable, if anything.
func (d *Dependents) problem() ruleProblem {
	if len(d.Packages) == 0 {
		return incomplete("fitness dependents rule %q names no packages", d.Name)
	}
	return ruleProblem{}
}

// problem says what makes the rule unusable, if anything.
func (l *LibraryRule) problem() ruleProblem {
	if len(l.From) == 0 {
		return incomplete("fitness libraries rule %q names no packages in from", l.Name)
	}
	return ruleProblem{}
}

// problem says what makes the rule unusable, if anything.
func (c *CallRule) problem() ruleProblem {
	if len(c.Callees) == 0 || (!c.Only && len(c.From) == 0) {
		return incomplete(
			"fitness calls rule %q needs callees, and from unless only is set",
			c.Name,
		)
	}
	return ruleProblem{}
}

// problem says what makes the rule unusable, if anything.
func (r *UnusedExports) problem() ruleProblem {
	if len(r.Packages) == 0 {
		return incomplete("fitness unusedExports rule %q names no packages", r.Name)
	}
	return ruleProblem{}
}

// problem says what makes the rule unusable, if anything.
func (r *FieldRule) problem() ruleProblem {
	if len(r.Fields) == 0 || (!r.Only && len(r.From) == 0) {
		return incomplete(
			"fitness fields rule %q needs fields, and from unless only is set",
			r.Name,
		)
	}
	for _, field := range r.Fields {
		if strings.Count(field, ".") == 0 || strings.HasSuffix(field, ".") {
			return ruleProblem{ProblemPattern, fmt.Sprintf(
				"fitness fields rule %q: %q is not pkg.Type.Field, pkg.Type.* or pkg.*",
				r.Name, field)}
		}
	}
	return ruleProblem{}
}

// problem says what makes the rule unusable, if anything.
func (r *InterfacesOnly) problem() ruleProblem {
	if len(r.From) == 0 || len(r.To) == 0 {
		return incomplete("fitness interfacesOnly rule %q needs from and to", r.Name)
	}
	return ruleProblem{}
}

// String returns the gate, with the default spelled out.
func (g RuleGate) String() string {
	if g == "" {
		return string(GateFail)
	}
	return string(g)
}

// problem says what makes the rule unusable, if anything.
func (c *CoChangeRule) problem() ruleProblem {
	if len(c.When) == 0 || len(c.Require) == 0 {
		return incomplete("fitness coChange rule %q needs when and require", c.Name)
	}
	for _, pat := range slices.Concat(c.When, c.Require, c.Except) {
		if !doublestar.ValidatePattern(pat) {
			return ruleProblem{ProblemPattern, fmt.Sprintf(
				"fitness coChange rule %q has malformed glob %q", c.Name, pat)}
		}
	}
	return ruleProblem{}
}

// problem says what makes the rule unusable, if anything.
func (r *FanOutRule) problem() ruleProblem {
	if len(r.Packages) == 0 || r.Max < 0 {
		return incomplete("fitness fanOut rule %q needs packages and a max of 0 or more", r.Name)
	}
	return ruleProblem{}
}

// problem says what makes the rule unusable, if anything.
func (o *OpenAPIRule) problem() ruleProblem {
	if o.Path == "" {
		return incomplete("fitness openapi rule %q names no path", o.Name)
	}
	return ruleProblem{}
}

// problem says what makes the rule unusable, if anything.
func (d *DomainRule) problem() ruleProblem {
	if d.Path == "" {
		return incomplete("fitness domains rule %q names no path", d.Name)
	}
	for name, typ := range d.Types {
		if name == "" || typ == "" {
			return incomplete("fitness domains rule %q maps an empty name or type", d.Name)
		}
	}
	return ruleProblem{}
}
