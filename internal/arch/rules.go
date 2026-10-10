package arch

import (
	"fmt"
	"slices"
	"strings"
)

// ruleResult is what one predicate produces: its findings, and the counts of
// violations among candidates that an MMI percentage criterion divides.
type ruleResult struct {
	findings   []Finding
	violations int
	candidates int
}

// subject is the analysis a rule reads: the program, the model and the graphs
// built from them. Rules read it and never change it.
type subject struct {
	p *Program
	m *Model
	g *Graphs
}

// assignmentRules reports packages the model leaves unassigned or assigns
// ambiguously, and patterns that match nothing (MMI 1.1.1, 1.1.2, 1.1.6).
func assignmentRules(s *subject) []Finding {
	var fs []Finding
	check := func(kind string, declared bool, a Assignment) {
		if !declared {
			return
		}
		for _, pkg := range a.Unassigned {
			f := newFinding(RuleUnassignedPackage, SeverityWarning, pkg, kind)
			f.Message = fmt.Sprintf("package %s belongs to no %s", pkg, kind)
			f.Fix = fmt.Sprintf(
				"add a pattern matching %s to a %s in the model, or move the package",
				pkg,
				kind,
			)
			fs = append(fs, f)
		}
		for _, pkg := range a.Ambiguous {
			f := newFinding(RuleAmbiguousPackage, SeverityError, pkg, kind)
			f.Message = fmt.Sprintf("two %ss match package %s equally specifically", kind, pkg)
			f.Fix = "make one pattern more specific, for example an exact path instead of /..."
			fs = append(fs, f)
		}
		for _, pat := range a.Stale {
			f := newFinding(RuleStalePattern, SeverityWarning, pat, kind)
			f.Message = fmt.Sprintf("%s pattern %s matches no package", kind, pat)
			f.Fix = "remove or correct the pattern: the model no longer describes the code"
			fs = append(fs, f)
		}
	}
	check("subsystem", len(s.m.Subsystems) > 0, s.g.SubsystemAssignment)
	check("layer", len(s.m.Layers) > 0, s.g.LayerAssignment)
	return fs
}

// orderRule reports in-scope imports that run upward in an order of groups:
// from a group to one listed above it. rank gives a package's position, or
// false when it has none. Imports within one group are not candidates.
func orderRule(
	s *subject, rule, criterion, instance string, rank func(pkg string) (int, string, bool),
) ruleResult {
	var r ruleResult
	for _, imp := range s.inScopeImports() {
		rf, gf, okFrom := rank(imp.From)
		rt, gt, okTo := rank(imp.To)
		if !okFrom || !okTo || rf == rt {
			continue
		}
		r.candidates++
		if rf < rt {
			continue
		}
		r.violations++
		f := newFinding(rule, SeverityError, imp.From, imp.To)
		f.instance = instance
		f.Location = spanAt(imp.Pos)
		f.Criterion = criterion
		f.Message = fmt.Sprintf(
			"%s (%s) imports %s (%s), which sits above it",
			imp.From,
			gf,
			imp.To,
			gt,
		)
		f.Fix = fmt.Sprintf("invert the dependency: declare the interface %s needs in %s and "+
			"implement it in %s, or move the code into %s", imp.From, gf, gt, gf)
		r.findings = append(r.findings, f)
	}
	return r
}

// layerRule is criterion 2.1.1: imports against the layer order.
func layerRule(s *subject) ruleResult {
	if len(s.m.Layers) < 2 {
		return ruleResult{}
	}
	pos := map[string]int{}
	for i, l := range s.m.Layers {
		pos[l.Name] = i
	}
	return orderRule(s, RuleLayerOrder, "2.1.1", "layers", func(pkg string) (int, string, bool) {
		layer, ok := s.g.LayerAssignment.Of[pkg]
		return pos[layer], layer, ok
	})
}

// subsystemOrderRule is criterion 2.1.2: imports against the subsystem order.
func subsystemOrderRule(s *subject) ruleResult {
	if len(s.m.SubsystemOrder) < 2 {
		return ruleResult{}
	}
	pos := map[string]int{}
	for i, name := range s.m.SubsystemOrder {
		pos[name] = i
	}
	return orderRule(
		s,
		RuleSubsystemOrder,
		"2.1.2",
		"subsystemOrder",
		func(pkg string) (int, string, bool) {
			sub, assigned := s.g.SubsystemAssignment.Of[pkg]
			rank, ordered := pos[sub]
			return rank, sub, assigned && ordered
		},
	)
}

// interfaceRule is criterion 1.2.1: cross-subsystem imports that bypass the
// target subsystem's interface packages. A subsystem that declares no
// interface has none, so every import into it counts (mmi.md §7.2).
func interfaceRule(s *subject) ruleResult {
	var r ruleResult
	if len(s.m.Subsystems) == 0 {
		return r
	}
	iface := map[string][]string{}
	for _, sub := range s.m.Subsystems {
		iface[sub.Name] = sub.Interface
		if len(sub.Interface) == 0 {
			f := newFinding(RuleSubsystemWithoutIface, SeverityWarning, sub.Name, "")
			f.Criterion = "1.2.2"
			f.Message = fmt.Sprintf("subsystem %s declares no interface package", sub.Name)
			f.Fix = "list the packages other subsystems may import as its interface; " +
				"Siedersleben's advice is its topmost package (mmi.md §7.6)"
			r.findings = append(r.findings, f)
		}
	}
	for _, imp := range s.inScopeImports() {
		from, okFrom := s.g.SubsystemAssignment.Of[imp.From]
		to, okTo := s.g.SubsystemAssignment.Of[imp.To]
		if !okFrom || !okTo || from == to {
			continue
		}
		r.candidates++
		if matchesAny(iface[to], imp.To) {
			continue
		}
		r.violations++
		f := newFinding(RuleInterfaceBypass, SeverityError, imp.From, imp.To)
		f.instance = "interfaces"
		f.Location = spanAt(imp.Pos)
		f.Criterion = "1.2.1"
		f.Message = fmt.Sprintf("%s imports %s, which is not an interface package of subsystem %s",
			imp.From, imp.To, to)
		f.Fix = bypassFix(to, iface[to], imp.To)
		r.findings = append(r.findings, f)
	}
	return r
}

// bypassFix says how to stop importing pkg from outside subsystem to, whose
// interface packages are iface.
func bypassFix(to string, iface []string, pkg string) string {
	if len(iface) == 0 {
		return fmt.Sprintf("subsystem %s declares no interface: declare the packages others may "+
			"import, such as %s, as its interface in the model", to, pkg)
	}
	return fmt.Sprintf(
		"import an interface package of %s (%s) instead, or declare %s part of its interface",
		to,
		strings.Join(iface, ", "),
		pkg,
	)
}

// forbiddenRule is the predicate behind criterion 3.4: imports the model
// forbids, including imports of packages outside the analysis.
func forbiddenRule(s *subject) ruleResult {
	var r ruleResult
	for _, imp := range s.p.Imports {
		for i := range s.m.ForbiddenImports {
			fi := &s.m.ForbiddenImports[i]
			if !matchesAny(fi.From, imp.From) || !matchesAny(fi.Imports, imp.To) {
				continue
			}
			r.violations++
			f := newFinding(RuleForbiddenImport, SeverityError, imp.From, imp.To)
			f.instance = instanceName("forbiddenImports", i)
			f.Location = spanAt(imp.Pos)
			f.Criterion = "3.4"
			f.Message = fmt.Sprintf("%s imports %s: %s", imp.From, imp.To, fi.Reason)
			f.Fix = fmt.Sprintf("move the use of %s into an adapter package and depend on an "+
				"interface declared where %s needs it", imp.To, imp.From)
			r.findings = append(r.findings, f)
			break
		}
	}
	return r
}

// isolationRule reports in-scope imports an isolation does not allow: the
// importing package matches From, and the imported one matches none of
// MayImport after substitution.
func isolationRule(s *subject) []Finding {
	var fs []Finding
	for _, imp := range s.inScopeImports() {
		for j := range s.m.Isolations {
			iso := &s.m.Isolations[j]
			_, captures, ok := matchCaptures(iso.From, imp.From)
			if !ok || isolationAllows(iso, captures, imp.To) {
				continue
			}
			allowed := make([]string, len(iso.MayImport))
			for i, pat := range iso.MayImport {
				allowed[i] = substitute(pat, captures)
			}
			f := newFinding(RuleIsolation, SeverityError, imp.From, imp.To)
			f.instance = instanceName("isolations", j)
			f.Location = spanAt(imp.Pos)
			f.Message = fmt.Sprintf("%s imports %s: %s", imp.From, imp.To, iso.Reason)
			f.Fix = fmt.Sprintf("%s may import only %s; move the shared code there, or depend on "+
				"an interface declared in %s", imp.From, strings.Join(allowed, ", "), imp.From)
			fs = append(fs, f)
			break
		}
	}
	return fs
}

// isolationAllows reports whether iso, with the importer's captures, allows
// importing pkg.
func isolationAllows(iso *Isolation, captures []string, pkg string) bool {
	return slices.ContainsFunc(iso.MayImport, func(pat string) bool {
		_, ok := matchPattern(substitute(pat, captures), pkg)
		return ok
	})
}

// inScopeImports returns the imports whose target is under analysis.
func (s *subject) inScopeImports() []Import {
	var imps []Import
	for _, imp := range s.p.Imports {
		if _, ok := s.g.SubsystemOf[imp.To]; ok {
			imps = append(imps, imp)
		}
	}
	return imps
}

// matchesAny reports whether any of the patterns matches path.
func matchesAny(patterns []string, path string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool {
		_, ok := matchPattern(p, path)
		return ok
	})
}
