package fitness

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// existing checks the model's other rules for selectors that match nothing.
type existing struct {
	p        *arch.Program
	m        *arch.Model
	types    map[string]*arch.Type
	packages []string
	ledger   *ledger
}

// checkSlices reports each elementary cycle among the slices of a rule with
// freeOfCycles (ArchUnit's slices().should().beFreeOfCycles()), with the
// imports behind each edge as evidence.
func (s *subject) checkSlices() {
	for i := range s.m.Fitness.Slices {
		sl := &s.m.Fitness.Slices[i]
		if !sl.FreeOfCycles {
			continue
		}
		r := s.sliceRule(sl, i)
		slice := s.sliceOf(sl.Pattern)
		edges := map[[2]string][]arch.Import{}
		var graph []arch.Edge
		for _, imp := range s.p.Imports {
			a, okA := slice[imp.From]
			b, okB := slice[imp.To]
			if okA && okB && a != b {
				edges[[2]string{a, b}] = append(edges[[2]string{a, b}], imp)
				graph = append(graph, arch.Edge{From: a, To: b, Weight: 1})
			}
		}
		cycles, _ := arch.NewGraph(nil, graph).ElementaryCycles(maxCycles)
		for _, c := range cycles {
			s.violation(r, sliceCycle(r.result.Name, c, edges))
		}
	}
}

// sliceCycle is the finding of one cycle among slices.
func sliceCycle(rule string, cycle []string, edges map[[2]string][]arch.Import) *arch.Finding {
	path := strings.Join(append(cycle, cycle[0]), " → ")
	f := arch.NewFinding(
		arch.FindingID{Rule: ruleSliceCycle, Subject: rule, Object: path},
		arch.SeverityError,
	)
	f.Message = "cycle among slices: " + path
	for i, from := range cycle {
		to := cycle[(i+1)%len(cycle)]
		imports := edges[[2]string{from, to}]
		for _, imp := range imports[:min(len(imports), maxEdgeEvidence)] {
			f.Evidence = append(f.Evidence, fmt.Sprintf("%s imports %s at %s:%d",
				imp.From, imp.To, imp.Pos.File, imp.Pos.Line))
		}
		if f.Location == nil && len(imports) > 0 {
			f.Location = arch.SpanAt(imports[0].Pos)
		}
	}
	f.Fix = "cut the edge with the fewest imports, by moving what it needs into a slice both may use"
	return &f
}

// checkCalls reports calls of callees a rule forbids, or calls from outside
// the packages a rule allows.
func (s *subject) checkCalls() {
	for i := range s.m.Fitness.Calls {
		c := &s.m.Fitness.Calls[i]
		r := s.addRule("calls", c.RuleOptions, arch.DefaultRuleName(c.Name, "calls", i))
		kind := ruleForbiddenCall
		if c.Only {
			kind = ruleCallOnly
		} else {
			r.result.Matched = s.matchingPackages(c.From)
		}
		for j := range s.p.Facts.Calls {
			call := &s.p.Facts.Calls[j]
			if !matchesCallee(c.Callees, call.Callee) {
				continue
			}
			if c.Only {
				r.result.Matched++
			}
			if arch.MatchesAny(c.From, call.Site.Package) == c.Only {
				continue
			}
			s.violation(r, callFinding(kind, r.result.Name, call))
		}
	}
}

// callFinding is the finding of one function's calls of a restricted callee.
func callFinding(kind, rule string, call *arch.Call) *arch.Finding {
	caller := cmp.Or(call.Site.Func, call.Site.Package)
	f := arch.NewFinding(
		arch.FindingID{Rule: kind, Subject: caller, Object: rule + ": " + call.Callee},
		arch.SeverityError,
	)
	f.Location = arch.SpanAt(arch.Position{File: call.Site.File, Line: call.Site.Line})
	f.Message = fmt.Sprintf("%s calls %s", caller, call.Callee)
	if call.Count > 1 {
		f.Message += fmt.Sprintf(" %d times", call.Count)
	}
	f.Fix = "take what the call provides as a parameter, so the caller no longer needs it"
	return &f
}

// matchesCallee reports a full name that one of the patterns names exactly,
// or whose package a "pkg.*" pattern names.
func matchesCallee(patterns []string, callee string) bool {
	for _, p := range patterns {
		if p == callee {
			return true
		}
		if pkg, ok := strings.CutSuffix(p, ".*"); ok && arch.FuncPackage(callee) == pkg {
			return true
		}
	}
	return false
}

// checkInterfacesOnly reports uses of a type that is not an interface, from
// a package that may use only the interfaces of the type's package.
func (s *subject) checkInterfacesOnly() {
	for i := range s.m.Fitness.InterfacesOnly {
		io := &s.m.Fitness.InterfacesOnly[i]
		r := s.addRule(
			"interfacesOnly",
			io.RuleOptions,
			arch.DefaultRuleName(io.Name, "interfacesOnly", i),
		)
		r.result.Matched = s.matchingPackages(io.From)
		for j := range s.p.Facts.Uses {
			u := &s.p.Facts.Uses[j]
			t, ok := s.types[u.Object]
			if u.Kind != arch.ObjectType || !ok || t.Kind == arch.KindInterface ||
				!arch.MatchesAny(io.From, u.Site.Package) || !arch.MatchesAny(io.To, u.Package) ||
				arch.MatchesAny(io.To, u.Site.Package) {
				continue
			}
			user := cmp.Or(u.Site.Func, u.Site.Type, u.Site.Package)
			f := arch.NewFinding(arch.FindingID{
				Rule: ruleInterfaces, Subject: user, Object: r.result.Name + ": " + u.Object,
			}, arch.SeverityError)
			f.Location = arch.SpanAt(arch.Position{File: u.Site.File, Line: u.Site.Line})
			f.Message = fmt.Sprintf("%s uses %s, %s, where only interfaces are allowed",
				user, u.Object, kindText(t.Kind))
			f.Fix = "depend on an interface the package exports, or declare one where it is used"
			s.violation(r, &f)
		}
	}
}

// checkAssigned reports packages outside every subsystem, and outside every
// layer when layers exist (ArchUnit's
// ensureAllClassesAreContainedInArchitecture).
func (s *subject) checkAssigned() {
	if !s.m.Fitness.RequireAssigned {
		return
	}
	r := s.addRule("requireAssigned", arch.RuleOptions{}, "requireAssigned")
	r.result.Matched = len(s.p.Packages)
	report := func(pkg, what string) {
		f := arch.NewFinding(
			arch.FindingID{Rule: ruleOutside, Subject: pkg, Object: what},
			arch.SeverityError,
		)
		f.Message = fmt.Sprintf("%s belongs to no %s", pkg, what)
		f.Fix = "add the package to the model's " + what + "s"
		s.violation(r, &f)
	}
	for _, pkg := range s.g.SubsystemAssignment.Unassigned {
		report(pkg, "subsystem")
	}
	if len(s.m.Layers) > 0 {
		for _, pkg := range s.g.LayerAssignment.Unassigned {
			report(pkg, "layer")
		}
	}
}

// check reports the selectors of the model's other rules that match
// nothing: forbidden imports, isolations, subsystem interfaces, roles and
// balance's messaging clients (ArchUnit fails an empty rule by default).
// Stale subsystem and layer patterns are analyze's findings already. The
// findings are errors when the model declares fitness rules, and warnings
// otherwise.
func (e *existing) check() {
	targets := map[string]bool{}
	for _, imp := range e.p.RuleImports() {
		targets[imp.To] = true
	}
	targetList := slices.Sorted(maps.Keys(targets))
	matchesTarget := func(patterns []string) bool {
		for t := range targets {
			if arch.MatchesAny(patterns, t) {
				return true
			}
		}
		return false
	}
	for i, fi := range e.m.ForbiddenImports {
		name := fmt.Sprintf("forbiddenImports[%d]", i)
		e.emptyExisting(name, "from", countMatching(e.packages, fi.From) > 0,
			arch.Corrections(fi.From, e.packages)...)
		e.emptyExisting(name, "imports", matchesTarget(fi.Imports),
			arch.Corrections(fi.Imports, targetList)...)
	}
	for i := range e.m.Isolations {
		iso := &e.m.Isolations[i]
		name := fmt.Sprintf("isolations[%d]", i)
		from := []string{iso.From}
		if !e.emptyExisting(name, "from", countMatching(e.packages, from) > 0,
			arch.Corrections(from, e.packages)...) {
			e.emptyExisting(name, "mayImport", e.mayImportMatches(iso))
		}
	}
	// An interface that matches nothing makes every import into the
	// subsystem from another one an interface bypass.
	for _, sub := range e.m.Subsystems {
		if len(sub.Interface) > 0 {
			e.emptyExisting("subsystems."+sub.Name, "interface",
				countMatching(e.packages, sub.Interface) > 0,
				arch.Corrections(sub.Interface, e.packages)...)
		}
	}
	if len(e.m.Balance.Messaging) > 0 {
		e.emptyExisting("balance", "messaging", matchesTarget(e.m.Balance.Messaging),
			arch.Corrections(e.m.Balance.Messaging, targetList)...)
	}
	e.emptyRoles()
}

// mayImportMatches reports whether some package an isolation constrains may
// import some package under analysis: whether, with the captures of a
// package matching From, one of the MayImport patterns matches a package.
func (e *existing) mayImportMatches(iso *arch.Isolation) bool {
	for _, from := range e.packages {
		_, captures, ok := arch.MatchCaptures(iso.From, from)
		if !ok {
			continue
		}
		for _, pat := range iso.MayImport {
			if countMatching(e.packages, []string{arch.Substitute(pat, captures)}) > 0 {
				return true
			}
		}
	}
	return false
}

// emptyRoles reports the role selectors that match nothing.
func (e *existing) emptyRoles() {
	typeIDs := slices.Sorted(maps.Keys(e.types))
	for i := range e.m.Roles {
		ro := &e.m.Roles[i]
		name := "roles." + ro.Name
		if len(ro.Packages) > 0 {
			e.emptyExisting(name, "packages", countMatching(e.packages, ro.Packages) > 0,
				arch.Corrections(ro.Packages, e.packages)...)
		}
		if len(ro.Embeds) > 0 {
			e.emptyExisting(name, "embeds", e.anyTypeExists(ro.Embeds),
				arch.Corrections(ro.Embeds, typeIDs)...)
		}
		if len(ro.Implements) > 0 {
			e.emptyExisting(name, "implements", e.anyTypeExists(ro.Implements),
				arch.Corrections(ro.Implements, typeIDs)...)
		}
	}
}

// emptyExisting reports one selector of an existing rule that matches
// nothing, with evidence such as the corrections of its patterns, and
// returns whether it reported it.
func (e *existing) emptyExisting(rule, selector string, matched bool, evidence ...string) bool {
	if matched {
		return false
	}
	sev := arch.SeverityWarning
	if e.m.Fitness.Enabled() {
		sev = arch.SeverityError
	}
	f := arch.NewFinding(
		arch.FindingID{Rule: ruleEmptySelector, Subject: rule, Object: selector},
		sev,
	)
	f.ModelRule = rule
	f.Message = fmt.Sprintf(
		"the %s of %s match nothing, so the rule checks nothing",
		selector,
		rule,
	)
	f.Fix = "correct the patterns, or remove the rule"
	f.Evidence = evidence
	e.ledger.findings = append(e.ledger.findings, f)
	return true
}

// anyTypeExists reports whether any of the type IDs is a type of the program.
func (e *existing) anyTypeExists(ids []string) bool {
	for _, id := range ids {
		if _, ok := e.types[id]; ok {
			return true
		}
	}
	return false
}

// kindText names a type kind with its article.
func kindText(k arch.TypeKind) string {
	if k == arch.KindStruct {
		return "a struct"
	}
	return "a defined type"
}
