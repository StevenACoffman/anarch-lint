package arch

import (
	"fmt"
	"slices"
	"strings"
)

// forbiddenSink names the synthetic node that stands for a forbidden package
// in the graph a transitive rule searches.
const forbiddenSink = "\x00forbidden"

// transitiveForbiddenRule reports packages that reach a forbidden package
// through other in-scope packages, for rules marked transitive. Each finding
// carries the shortest import path as evidence and, when every path passes
// through one package, names it: cutting that one dependency removes them all.
// A direct import is the plain forbidden-import rule's finding, not this one's.
func transitiveForbiddenRule(s *subject) []Finding {
	var fs []Finding
	for i := range s.m.ForbiddenImports {
		fi := &s.m.ForbiddenImports[i]
		if !fi.Transitive {
			continue
		}
		for _, forbidden := range s.forbiddenTargets(fi) {
			fs = append(fs, s.reachFindings(fi, instanceName("forbiddenImports", i), forbidden)...)
		}
	}
	return fs
}

// forbiddenTargets returns the packages, in scope or not, that some in-scope
// package imports and that a rule forbids, sorted.
func (s *subject) forbiddenTargets(fi *ForbiddenImport) []string {
	var targets []string
	for _, imp := range s.p.Imports {
		if MatchesAny(fi.Imports, imp.To) {
			targets = append(targets, imp.To)
		}
	}
	slices.Sort(targets)
	return slices.Compact(targets)
}

// reachFindings reports every package matching the rule that reaches
// forbidden only indirectly.
func (s *subject) reachFindings(fi *ForbiddenImport, instance, forbidden string) []Finding {
	reach := s.withSink(forbidden)
	var fs []Finding
	for _, pkg := range packagePaths(s.p) {
		if !MatchesAny(fi.From, pkg) || s.importsDirectly(pkg, forbidden) {
			continue
		}
		path := reach.shortestPath(pkg, forbiddenSink)
		if path == nil {
			continue
		}
		path = append(path[:len(path)-1], forbidden)
		f := NewFinding(
			FindingID{Rule: RuleForbiddenImport, Subject: pkg, Object: forbidden},
			SeverityError,
		)
		f.ModelRule = instance
		f.Criterion = "3.4"
		f.Message = fmt.Sprintf("%s reaches %s through %d other packages: %s",
			pkg, forbidden, len(path)-2, fi.Reason)
		f.Evidence = []string{"import path: " + strings.Join(path, " → ")}
		f.Fix = reachFix(reach.Dominators(pkg)[forbiddenSink], pkg, forbidden)
		if imp, ok := s.firstImport(pkg, path[1]); ok {
			f.Location = SpanAt(imp.Pos)
			f.Provenance = imp.Provenance()
		}
		fs = append(fs, f)
	}
	return fs
}

// reachFix says how to cut every path from pkg to forbidden, given the
// immediate dominator of the forbidden package as seen from pkg.
func reachFix(dominator, pkg, forbidden string) string {
	if dominator == "" || dominator == pkg {
		return fmt.Sprintf("several independent import paths lead from %s to %s; cut each one, "+
			"starting with the shortest", pkg, forbidden)
	}
	return fmt.Sprintf("every path from %s to %s passes through %s: make %s independent of %s, "+
		"or stop %s depending on it, and every path is cut at once", pkg, forbidden, dominator,
		dominator, forbidden, pkg)
}

// withSink returns the package graph with an extra node standing for
// forbidden, reached by every in-scope package that imports it directly.
func (s *subject) withSink(forbidden string) Graph {
	edges := s.g.Packages.Edges()
	for _, imp := range s.p.Imports {
		if imp.To == forbidden {
			edges = append(edges, Edge{From: imp.From, To: forbiddenSink, Weight: 1})
		}
	}
	return NewGraph(append(s.g.Packages.Nodes(), forbiddenSink), edges)
}

// importsDirectly reports whether pkg imports target itself, in the build
// or in a file it excludes: either is the plain forbidden-import finding.
func (s *subject) importsDirectly(pkg, target string) bool {
	return slices.ContainsFunc(s.p.RuleImports(), func(imp Import) bool {
		return imp.From == pkg && imp.To == target
	})
}

// firstImport returns pkg's import of target.
func (s *subject) firstImport(pkg, target string) (Import, bool) {
	for _, imp := range s.p.Imports {
		if imp.From == pkg && imp.To == target {
			return imp, true
		}
	}
	return Import{}, false
}

// hiddenDependencyRule reports, for forbidden-import rules marked injected,
// every call that hands a package a value of a type from a package it may not
// depend on. The finding is about the receiving package; its location is the
// call that injects the value.
func hiddenDependencyRule(s *subject) []Finding {
	var fs []Finding
	seen := map[string]bool{}
	for _, inj := range s.p.Injections {
		concretePkg := s.g.PackageOf[inj.Concrete]
		for i := range s.m.ForbiddenImports {
			fi := &s.m.ForbiddenImports[i]
			if !fi.Injected || !MatchesAny(fi.From, inj.CalleePackage) ||
				!MatchesAny(fi.Imports, concretePkg) {
				continue
			}
			f := NewFinding(
				FindingID{
					Rule:    RuleHiddenDependency,
					Subject: inj.CalleePackage,
					Object:  inj.Concrete,
				},
				SeverityError,
			)
			if seen[f.Fingerprint] {
				break
			}
			seen[f.Fingerprint] = true
			f.ModelRule = instanceName("forbiddenImports", i)
			f.Criterion = "3.4"
			f.Location = SpanAt(inj.Pos)
			f.Provenance = ProvenanceInjected
			f.Message = fmt.Sprintf(
				"%s receives a %s through an interface, a run-time dependency on %s: %s",
				inj.CalleePackage,
				inj.Concrete,
				concretePkg,
				fi.Reason,
			)
			f.Evidence = []string{fmt.Sprintf("%s passes a %s to %s at %s:%d",
				inj.Caller, inj.Concrete, inj.Callee, inj.Pos.File, inj.Pos.Line)}
			f.Fix = fmt.Sprintf(
				"pass %s an implementation from a package it may depend on, or narrow "+
					"the rule if %s is meant to be injected here",
				inj.Callee,
				inj.Concrete,
			)
			fs = append(fs, f)
			break
		}
	}
	return append(fs, dispatchFindings(s, seen)...)
}

// dispatchFindings reports, for forbidden-import rules marked injected, each
// call a package makes through an interface that the VTA call graph resolves
// to a method of a type from a package it may not depend on. The finding is
// about the calling package; seen holds the fingerprints already reported,
// so an injection and the call it enables are reported once.
func dispatchFindings(s *subject, seen map[string]bool) []Finding {
	var fs []Finding
	for _, d := range s.p.Dispatches {
		concretePkg := s.g.PackageOf[d.Concrete]
		for i := range s.m.ForbiddenImports {
			fi := &s.m.ForbiddenImports[i]
			if !fi.Injected || !MatchesAny(fi.From, d.CallerPackage) ||
				!MatchesAny(fi.Imports, concretePkg) {
				continue
			}
			f := NewFinding(FindingID{
				Rule: RuleHiddenDependency, Subject: d.CallerPackage, Object: d.Concrete,
			}, SeverityError)
			if seen[f.Fingerprint] {
				break
			}
			seen[f.Fingerprint] = true
			f.ModelRule, f.Criterion = instanceName("forbiddenImports", i), "3.4"
			f.Location, f.Provenance = SpanAt(d.Pos), ProvenanceDispatch
			f.Message = fmt.Sprintf(
				"%s calls %s through an interface at run time, a dependency on %s: %s",
				d.CallerPackage, d.Callee, concretePkg, fi.Reason)
			f.Evidence = []string{
				fmt.Sprintf("the VTA call graph resolves the call in %s at %s:%d to %s",
					d.Caller, d.Pos.File, d.Pos.Line, d.Callee),
			}
			f.Fix = fmt.Sprintf("wire %s with an implementation from a package it may depend "+
				"on, or narrow the rule if %s is meant to be reached here",
				d.CallerPackage, d.Concrete)
			fs = append(fs, f)
			break
		}
	}
	return fs
}
