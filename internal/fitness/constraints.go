package fitness

import (
	"fmt"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// constraint is one rule compiled to a predicate over an import: layer
// access, strict layers, the onion, slices and dependents all become
// constraints, which one pass over the imports checks.
type constraint struct {
	rule *rule
	// kind is the finding's rule name; fix says how to repair a violation.
	kind, fix string
	// anyTarget also checks imports of packages outside the analysis.
	anyTarget bool
	// violates returns why importing to from from breaks the rule, or "".
	violates func(from, to string) string
	// reads returns everything about a package that violates reads, so two
	// packages it reads alike are one class to the verification.
	reads func(pkg string) string
	// allowances are the imports the rule allows by name, such as a layer
	// in an allow-list; a rule that only forbids has none.
	allowances []allowance
}

// allowance is one name a rule allows imports by: item is the name as the
// model spells it, and names reports whether an import is one it allows.
type allowance struct {
	item  string
	names func(from, to string) bool
}

// end is one end of an import: its layer, and whether it has one.
type end struct {
	layer   string
	layered bool
}

// ring is a package's place in the onion: 0 for the domain models, 1 domain
// services, 2 application services, 3 an adapter, named.
type ring struct {
	level   int
	adapter string
}

// importConstraints compiles every import rule of the model.
func (s *subject) importConstraints() []constraint {
	f := &s.m.Fitness
	var cs []constraint
	if f.StrictLayers {
		cs = append(cs, s.strictLayers())
	}
	for i := range f.LayerAccess {
		cs = append(cs, s.layerAccess(&f.LayerAccess[i], i))
	}
	if f.Onion.Declared() {
		cs = append(cs, s.onion(&f.Onion))
	}
	for i := range f.Slices {
		if sl := &f.Slices[i]; sl.NotDependOnEachOther {
			cs = append(cs, s.sliceIsolation(sl, i))
		}
	}
	for i := range f.Dependents {
		cs = append(cs, s.dependents(&f.Dependents[i], i))
	}
	for i := range f.Libraries {
		cs = append(cs, libraryConstraint(s.ledger, &f.Libraries[i], i, s.p, s.g.SubsystemOf))
	}
	return cs
}

// checkImports runs every constraint over every import the rules check, in
// sorted order: the build's, then those of the files it excludes.
func (s *subject) checkImports(cs []constraint) {
	for _, imp := range s.p.RuleImports() {
		for _, c := range cs {
			if f, ok := s.importFinding(&c, &imp); ok {
				s.violation(c.rule, &f)
			}
		}
	}
}

// importFinding returns the finding of c's violation by imp, and false when
// c does not check imp or imp breaks nothing c states.
func (s *subject) importFinding(c *constraint, imp *arch.Import) (arch.Finding, bool) {
	if _, inScope := s.g.SubsystemOf[imp.To]; !inScope && !c.anyTarget {
		return arch.Finding{}, false
	}
	why := c.violates(imp.From, imp.To)
	if why == "" {
		return arch.Finding{}, false
	}
	f := arch.NewFinding(arch.FindingID{
		Rule: c.kind, Subject: imp.From, Object: c.rule.result.Name + ": " + imp.To,
	}, arch.SeverityError)
	f.Location = arch.SpanAt(imp.Pos)
	f.Message = fmt.Sprintf("%s imports %s: %s", imp.From, imp.To, why)
	f.Fix = c.fix
	f.Provenance = imp.Provenance()
	return f, true
}

// strictLayers lets each layer import only the one directly below it. An
// import upward is analyze's layer-order violation and is not repeated here.
func (s *subject) strictLayers() constraint {
	r := s.addRule("strictLayers", arch.RuleOptions{}, "strictLayers")
	r.result.Matched = len(s.g.LayerAssignment.Of)
	index := map[string]int{}
	for i, l := range s.m.Layers {
		index[l.Name] = i
	}
	return constraint{
		rule: r, kind: arch.RuleStrictLayers, reads: s.layerKey,
		fix: "import through the layer directly below, or move the code into it",
		violates: func(from, to string) string {
			a, okA := s.g.LayerAssignment.Of[from]
			b, okB := s.g.LayerAssignment.Of[to]
			if !okA || !okB || index[b] <= index[a]+1 {
				return ""
			}
			return fmt.Sprintf("layer %s may import only layer %s, not %s",
				a, s.m.Layers[index[a]+1].Name, b)
		},
	}
}

// layerAccess restricts one layer's incoming and outgoing imports.
func (s *subject) layerAccess(la *arch.LayerAccess, i int) constraint {
	r := s.addRule("layerAccess", la.RuleOptions, arch.DefaultRuleName(la.Name, "layerAccess", i))
	for _, l := range s.g.LayerAssignment.Of {
		if l == la.Layer {
			r.result.Matched++
		}
	}
	return constraint{
		rule:       r,
		kind:       arch.RuleLayerAccess,
		fix:        "depend through a layer the rule allows, or change the rule if the dependency is intended",
		violates:   func(from, to string) string { return s.layerViolation(la, from, to) },
		reads:      s.layerKey,
		allowances: layerAllowances(la, s.g.LayerAssignment.Of),
	}
}

// layerAllowances are the layers a layer access rule names as allowed: each
// layer its layer may import, and each layer that may import it.
func layerAllowances(la *arch.LayerAccess, layerOf map[string]string) []allowance {
	layer := func(pkg string) string { return layerOf[pkg] }
	var out []allowance
	for _, x := range la.MayOnlyAccess {
		if x != la.Layer {
			out = append(out, allowance{
				item:  "mayOnlyAccess " + x,
				names: func(from, to string) bool { return layer(from) == la.Layer && layer(to) == x },
			})
		}
	}
	for _, x := range la.MayOnlyBeAccessedBy {
		if x != la.Layer {
			out = append(out, allowance{
				item:  "mayOnlyBeAccessedBy " + x,
				names: func(from, to string) bool { return layer(from) == x && layer(to) == la.Layer },
			})
		}
	}
	return out
}

// layerKey is what the layer rules read of a package: its layer.
func (s *subject) layerKey(pkg string) string {
	if l, ok := s.g.LayerAssignment.Of[pkg]; ok {
		return "layer " + l
	}
	return "no layer"
}

// layerViolation says why an import from from to to breaks la, or "". A
// layer may always use itself.
func (s *subject) layerViolation(la *arch.LayerAccess, from, to string) string {
	a, okA := s.g.LayerAssignment.Of[from]
	b, okB := s.g.LayerAssignment.Of[to]
	if okA && okB && a == b {
		return ""
	}
	if why := incoming(la, end{a, okA}, end{b, okB}); why != "" {
		return why
	}
	return outgoing(la, end{a, okA}, end{b, okB})
}

// incoming says why an import into la's layer from from breaks la, or "".
func incoming(la *arch.LayerAccess, from, to end) string {
	if !to.layered || to.layer != la.Layer || (!from.layered && !la.ConsideringAll) {
		return ""
	}
	switch {
	case la.MayNotBeAccessedByAny:
		return "no layer may import layer " + la.Layer
	case len(la.MayOnlyBeAccessedBy) > 0 && !slices.Contains(la.MayOnlyBeAccessedBy, from.layer):
		return fmt.Sprintf(
			"only %s may import layer %s",
			strings.Join(la.MayOnlyBeAccessedBy, ", "),
			la.Layer,
		)
	}
	return ""
}

// outgoing says why an import from la's layer into to breaks la, or "".
func outgoing(la *arch.LayerAccess, from, to end) string {
	if !from.layered || from.layer != la.Layer || (!to.layered && !la.ConsideringAll) {
		return ""
	}
	switch {
	case la.MayNotAccessAny:
		return "layer " + la.Layer + " may import no other layer"
	case len(la.MayOnlyAccess) > 0 && !slices.Contains(la.MayOnlyAccess, to.layer):
		return fmt.Sprintf(
			"layer %s may import only %s",
			la.Layer,
			strings.Join(la.MayOnlyAccess, ", "),
		)
	}
	return ""
}

// onion lets each ring import only itself and the rings inside it, and no
// adapter import another (ArchUnit's onionArchitecture).
func (s *subject) onion(o *arch.Onion) constraint {
	r := s.addRule("onion", o.RuleOptions, arch.DefaultRuleName(o.Name, "onion", 0))
	rings := s.onionRings(o)
	r.result.Matched = len(rings)
	names := []string{"domain model", "domain service", "application service", "adapter"}
	return constraint{
		rule: r, kind: arch.RuleOnion,
		reads: func(pkg string) string {
			if rg, ok := rings[pkg]; ok {
				return fmt.Sprintf("ring %d %s", rg.level, rg.adapter)
			}
			return "no ring"
		},
		fix: "invert the dependency: declare what the inner ring needs as an interface in it, " +
			"and implement it outside",
		violates: func(from, to string) string {
			a, okA := rings[from]
			b, okB := rings[to]
			switch {
			case !okA || !okB:
				return ""
			case b.level > a.level:
				return fmt.Sprintf(
					"a %s may not import an outer %s",
					names[a.level],
					names[b.level],
				)
			case a.adapter != "" && b.adapter != "" && a.adapter != b.adapter:
				return fmt.Sprintf("adapter %s may not import adapter %s", a.adapter, b.adapter)
			}
			return ""
		},
	}
}

// onionRings places each in-scope package in the innermost ring that
// matches it; adapters are tried in name order.
func (s *subject) onionRings(o *arch.Onion) map[string]ring {
	type candidate struct {
		patterns []string
		ring     ring
	}
	candidates := []candidate{
		{o.DomainModels, ring{level: 0}},
		{o.DomainServices, ring{level: 1}},
		{o.ApplicationServices, ring{level: 2}},
	}
	for _, name := range sortedKeys(o.Adapters) {
		candidates = append(candidates, candidate{o.Adapters[name], ring{level: 3, adapter: name}})
	}
	out := map[string]ring{}
	for _, pkg := range s.packagePaths() {
		for _, c := range candidates {
			if arch.MatchesAny(c.patterns, pkg) {
				out[pkg] = c.ring
				break
			}
		}
	}
	return out
}

// sliceIsolation forbids imports between two slices of one pattern.
func (s *subject) sliceIsolation(sl *arch.Slices, i int) constraint {
	r := s.sliceRule(sl, i)
	slice := s.sliceOf(sl.Pattern)
	return constraint{
		rule: r, kind: arch.RuleSliceDependency,
		reads: func(pkg string) string {
			if v, ok := slice[pkg]; ok {
				return "slice " + v
			}
			return "no slice"
		},
		fix: "move what both slices need into a package outside them",
		violates: func(from, to string) string {
			a, okA := slice[from]
			b, okB := slice[to]
			if !okA || !okB || a == b {
				return ""
			}
			return fmt.Sprintf("slice %s may not depend on slice %s", a, b)
		},
	}
}

// sliceRule returns the one rule record of slices rule i, which both its
// isolation and its cycle check report to.
func (s *subject) sliceRule(sl *arch.Slices, i int) *rule {
	if r, ok := s.slices[i]; ok {
		return r
	}
	r := s.addRule("slices", sl.RuleOptions, arch.DefaultRuleName(sl.Name, "slices", i))
	r.result.Matched = len(s.sliceOf(sl.Pattern))
	s.slices[i] = r
	return r
}

// sliceOf maps each in-scope package the pattern matches to its slice: the
// pattern's captures, joined with "/".
func (s *subject) sliceOf(pattern string) map[string]string {
	out := map[string]string{}
	for _, pkg := range s.packagePaths() {
		if _, captures, ok := arch.MatchCaptures(pattern, pkg); ok {
			out[pkg] = strings.Join(captures, "/")
		}
	}
	return out
}

// dependents lets only the packages a rule names import its packages, which
// may lie outside the analysis, such as database/sql.
func (s *subject) dependents(d *arch.Dependents, i int) constraint {
	r := s.addRule("dependents", d.RuleOptions, arch.DefaultRuleName(d.Name, "dependents", i))
	targets := map[string]bool{}
	for _, imp := range s.p.RuleImports() {
		if arch.MatchesAny(d.Packages, imp.To) {
			targets[imp.To] = true
		}
	}
	r.result.Matched = len(targets) + s.matchingPackages(d.Packages)
	// Any package outside the rule's may be the importer it forbids.
	r.readsCode = everyPackage
	return constraint{
		rule: r, kind: arch.RuleDependents, anyTarget: true,
		reads:      func(pkg string) string { return matchBits(pkg, d.Packages, d.OnlyFrom) },
		allowances: dependentAllowances(d),
		fix:        "reach the package through one of the packages the rule allows",
		violates: func(from, to string) string {
			if !arch.MatchesAny(d.Packages, to) || arch.MatchesAny(d.Packages, from) ||
				arch.MatchesAny(d.OnlyFrom, from) {
				return ""
			}
			if len(d.OnlyFrom) == 0 {
				return "nothing outside it may import it"
			}
			return "only " + strings.Join(d.OnlyFrom, ", ") + " may import it"
		},
	}
}

// dependentAllowances are the patterns a dependents rule names as the only
// importers allowed.
func dependentAllowances(d *arch.Dependents) []allowance {
	out := make([]allowance, 0, len(d.OnlyFrom))
	for _, pat := range d.OnlyFrom {
		out = append(out, allowance{
			item: "onlyFrom " + pat,
			names: func(from, to string) bool {
				return arch.MatchesAny(d.Packages, to) && !arch.MatchesAny(d.Packages, from) &&
					arch.MatchesAny([]string{pat}, from)
			},
		})
	}
	return out
}

// matchBits says, for the whole of first and then each pattern of each, whether
// it matches pkg: what a rule over these patterns can tell about a package.
func matchBits(pkg string, first []string, each []string) string {
	bits := []byte{bit(arch.MatchesAny(first, pkg))}
	for _, pat := range each {
		bits = append(bits, bit(arch.MatchesAny([]string{pat}, pkg)))
	}
	return string(bits)
}

// bit is '1' for true and '0' for false.
func bit(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}

// sortedKeys returns a map's keys, sorted.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
