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
	// passes are the imports the rule examined and allowed, as findings that
	// are not reported, so the model's gates apply to them alike; they
	// explain a pass (ExplainImport).
	passes []Finding
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
	paths := make([]string, 0, len(s.p.Packages))
	for _, pkg := range s.p.Packages {
		paths = append(paths, pkg.Path)
	}
	check := func(kind string, declared bool, a Assignment) {
		if !declared {
			return
		}
		for _, pkg := range a.Unassigned {
			f := NewFinding(
				FindingID{Rule: RuleUnassignedPackage, Subject: pkg, Object: kind},
				SeverityWarning,
			)
			f.Message = fmt.Sprintf("package %s belongs to no %s", pkg, kind)
			f.Fix = fmt.Sprintf(
				"add a pattern matching %s to a %s in the model, or move the package",
				pkg,
				kind,
			)
			fs = append(fs, f)
		}
		for _, pkg := range a.Ambiguous {
			f := NewFinding(
				FindingID{Rule: RuleAmbiguousPackage, Subject: pkg, Object: kind},
				SeverityError,
			)
			f.Message = fmt.Sprintf("two %ss match package %s equally specifically", kind, pkg)
			f.Fix = "make one pattern more specific, for example an exact path instead of /..."
			fs = append(fs, f)
		}
		for _, pat := range a.Stale {
			f := NewFinding(
				FindingID{Rule: RuleStalePattern, Subject: pat, Object: kind},
				SeverityWarning,
			)
			f.Message = fmt.Sprintf("%s pattern %s matches no package", kind, pat)
			f.Fix = "remove or correct the pattern: the model no longer describes the code"
			_, pattern, _ := strings.Cut(pat, ": ") // Stale holds "group: pattern"
			f.Evidence = Corrections([]string{pattern}, paths)
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
			r.passes = append(r.passes, pass(rule, instance,
				fmt.Sprintf("%s (%s) sits above %s (%s)", imp.From, gf, imp.To, gt)))
			continue
		}
		r.violations++
		f := NewFinding(FindingID{Rule: rule, Subject: imp.From, Object: imp.To}, SeverityError)
		f.ModelRule = instance
		f.Location = SpanAt(imp.Pos)
		f.Provenance = imp.Provenance()
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
			f := NewFinding(
				FindingID{Rule: RuleSubsystemWithoutIface, Subject: sub.Name, Object: ""},
				SeverityWarning,
			)
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
		if MatchesAny(iface[to], imp.To) {
			r.passes = append(r.passes, pass(RuleInterfaceBypass, "interfaces",
				fmt.Sprintf("%s is an interface package of subsystem %s", imp.To, to)))
			continue
		}
		r.violations++
		f := NewFinding(
			FindingID{Rule: RuleInterfaceBypass, Subject: imp.From, Object: imp.To},
			SeverityError,
		)
		f.ModelRule = "interfaces"
		f.Location = SpanAt(imp.Pos)
		f.Provenance = imp.Provenance()
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
	for _, imp := range slices.Concat(s.p.RuleImports(), s.p.TestImports) {
		for i := range s.m.ForbiddenImports {
			fi := &s.m.ForbiddenImports[i]
			if (imp.Test && !fi.Tests) || !MatchesAny(fi.From, imp.From) {
				continue
			}
			if !MatchesAny(fi.Imports, imp.To) {
				r.passes = append(
					r.passes,
					pass(RuleForbiddenImport, instanceName("forbiddenImports", i),
						imp.To+" is not among the packages it forbids "+imp.From+" to import"),
				)
				continue
			}
			r.violations++
			f := NewFinding(
				FindingID{Rule: RuleForbiddenImport, Subject: imp.From, Object: imp.objectID()},
				SeverityError,
			)
			f.ModelRule = instanceName("forbiddenImports", i)
			f.Location = SpanAt(imp.Pos)
			f.Criterion = "3.4"
			f.Message = fmt.Sprintf(
				"%s%s imports %s: %s",
				imp.From,
				imp.inTest(),
				imp.To,
				fi.Reason,
			)
			f.Fix = fmt.Sprintf("move the use of %s into an adapter package and depend on an "+
				"interface declared where %s needs it", imp.To, imp.From)
			f.Provenance = imp.Provenance()
			r.findings = append(r.findings, f)
			break
		}
	}
	return r
}

// isolationRule reports in-scope imports an isolation does not allow: the
// importing package matches From, and the imported one matches none of
// MayImport after substitution.
func isolationRule(s *subject) ruleResult {
	var r ruleResult
	for _, imp := range s.inScope(slices.Concat(s.p.RuleImports(), s.p.TestImports)) {
		for j := range s.m.Isolations {
			iso := &s.m.Isolations[j]
			captures, ok := iso.governs(&imp)
			if !ok {
				continue
			}
			if isolationAllows(iso, captures, imp.To) {
				r.passes = append(r.passes, pass(RuleIsolation, instanceName("isolations", j),
					imp.To+" is among the packages "+imp.From+" may import"))
				continue
			}
			allowed := make([]string, len(iso.MayImport))
			for i, pat := range iso.MayImport {
				allowed[i] = Substitute(pat, captures)
			}
			f := NewFinding(
				FindingID{Rule: RuleIsolation, Subject: imp.From, Object: imp.objectID()},
				SeverityError,
			)
			f.ModelRule = instanceName("isolations", j)
			f.Location = SpanAt(imp.Pos)
			f.Message = fmt.Sprintf(
				"%s%s imports %s: %s",
				imp.From,
				imp.inTest(),
				imp.To,
				iso.Reason,
			)
			f.Fix = fmt.Sprintf("%s may import only %s; move the shared code there, or depend on "+
				"an interface declared in %s", imp.From, strings.Join(allowed, ", "), imp.From)
			f.Provenance = imp.Provenance()
			r.findings = append(r.findings, f)
			break
		}
	}
	return r
}

// pass records that the rule named modelRule, whose findings are rule's,
// examined an import and allows it, for the reason why.
func pass(rule, modelRule, why string) Finding {
	return Finding{Rule: rule, ModelRule: modelRule, Message: why}
}

// isolationAllows reports whether iso, with the importer's captures, allows
// importing pkg.
func isolationAllows(iso *Isolation, captures []string, pkg string) bool {
	return slices.ContainsFunc(iso.MayImport, func(pat string) bool {
		_, ok := matchPattern(Substitute(pat, captures), pkg)
		return ok
	})
}

// inScopeImports returns the imports whose target is under analysis.
func (s *subject) inScopeImports() []Import {
	return s.inScope(s.p.Imports)
}

// inScope returns the imports of imps whose target is under analysis.
func (s *subject) inScope(imps []Import) []Import {
	var out []Import
	for _, imp := range imps {
		if _, ok := s.g.SubsystemOf[imp.To]; ok {
			out = append(out, imp)
		}
	}
	return out
}

// governs reports whether the isolation checks an import, with the
// captures its From takes from the importer: an import of a test only when
// the isolation opts in with Tests.
func (iso *Isolation) governs(imp *Import) ([]string, bool) {
	if imp.Test && !iso.Tests {
		return nil, false
	}
	_, captures, ok := MatchCaptures(iso.From, imp.From)
	return captures, ok
}

// MatchesAny reports whether any of the patterns matches path.
func MatchesAny(patterns []string, path string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool {
		_, ok := matchPattern(p, path)
		return ok
	})
}
