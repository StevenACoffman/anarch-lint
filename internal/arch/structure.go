package arch

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// subsystemCycleRule reports each subsystem cycle group with the package
// imports to cut to break it (mmi.md §6, §7.5). In Go every subsystem cycle
// comes from aggregation alone, so the first question is whether a package
// belongs in its subsystem at all (§3, the dissertation's §6.5.2).
func subsystemCycleRule(s *subject) []Finding {
	var fs []Finding
	for _, grp := range s.g.Subsystems.CycleGroups() {
		cuts, _ := minimumCut(s.g.Subsystems.subgraph(grp))
		subject := strings.Join(grp, " ↔ ")
		f := NewFinding(
			FindingID{Rule: RuleSubsystemCycle, Subject: subject, Object: ""},
			SeverityError,
		)
		f.Criterion = "2.1.2"
		f.Message = fmt.Sprintf("subsystems %s form a cycle", subject)
		var steps []string
		for _, cut := range cuts {
			imps := s.importsBetween(cut.From, cut.To)
			for _, imp := range imps {
				f.Evidence = append(f.Evidence, fmt.Sprintf("%s imports %s at %s:%d",
					imp.From, imp.To, imp.Pos.File, imp.Pos.Line))
			}
			if f.Location == nil && len(imps) > 0 {
				f.Location = SpanAt(imps[0].Pos)
			}
			steps = append(
				steps,
				fmt.Sprintf("%s → %s (%s)", cut.From, cut.To, Plural(cut.Weight, "import")),
			)
		}
		f.Fix = "cut " + strings.Join(steps, "; ") + ": move each importing package to the " +
			"subsystem it serves, or invert the import through an interface"
		f.sites = s.subsystemCycleSites(grp)
		fs = append(fs, f)
	}
	return fs
}

// typeCycleRule reports each type cycle group larger than the threshold, with
// the references to cut (von Zitzewitz's rule of five, mmi.md §7.5).
func typeCycleRule(s *subject, limit int) []Finding {
	var fs []Finding
	refs := map[[2]string]Ref{}
	for _, r := range s.p.Refs {
		refs[[2]string{r.From, r.To}] = r
	}
	for _, grp := range s.g.Types.CycleGroups() {
		if len(grp) <= limit {
			continue
		}
		cuts, exact := minimumCut(s.g.Types.subgraph(grp))
		f := NewFinding(
			FindingID{Rule: RuleTypeCycleTooLarge, Subject: grp[0], Object: strconv.Itoa(len(grp))},
			SeverityError,
		)
		f.Criterion = "2.2.3"
		f.Message = fmt.Sprintf(
			"%d types form one cycle group, above the limit of %d",
			len(grp),
			limit,
		)
		var steps []string
		for _, cut := range cuts {
			r := refs[[2]string{cut.From, cut.To}]
			if f.Location == nil {
				f.Location = SpanAt(r.Pos)
			}
			steps = append(steps, fmt.Sprintf("%s → %s (%s%s, %s:%d)",
				cut.From, cut.To, Plural(cut.Weight, "use"), kindsText(r.Kinds),
				r.Pos.File, r.Pos.Line))
		}
		method := "a minimum"
		if !exact {
			method = "a heuristic"
		}
		f.Evidence = grp
		f.Fix = fmt.Sprintf("cut these references (%s set): %s; invert each through an interface "+
			"(dependency inversion) or move the code it needs", method, strings.Join(steps, "; "))
		f.sites = typeCycleSites(s.p.Refs, grp)
		fs = append(fs, f)
	}
	return fs
}

// sizeRules reports functions and types above the size and complexity limits
// of their package (MMI 1.3.1, 1.3.2, 1.3.4).
func sizeRules(s *subject, limits map[string]Thresholds, cohesion map[string]int) []Finding {
	var fs []Finding
	for i := range s.p.Funcs {
		fn := &s.p.Funcs[i]
		t := limitsOf(limits, fn.Package)
		if fn.Lines > t.LargeFuncLines {
			f := NewFinding(
				FindingID{Rule: RuleLargeFunc, Subject: fn.ID, Object: ""},
				SeverityWarning,
			)
			f.Criterion, f.Location = "1.3.2", &fn.Span
			f.Message = fmt.Sprintf(
				"%s has %d lines of code, above %d",
				fn.ID,
				fn.Lines,
				t.LargeFuncLines,
			)
			f.Fix = "extract the steps that need no shared state into functions of their own"
			fs = append(fs, f)
		}
		if cc := fn.Complexity.Of(t.ComplexityVariant); cc > t.HighComplexity {
			f := NewFinding(
				FindingID{Rule: RuleComplexFunc, Subject: fn.ID, Object: ""},
				SeverityWarning,
			)
			f.Criterion, f.Location = "1.3.4", &fn.Span
			f.Message = fmt.Sprintf("%s has %s cyclomatic complexity %d, above %d",
				fn.ID, t.ComplexityVariant, cc, t.HighComplexity)
			f.Fix = fmt.Sprintf(
				"split the decisions into named predicates; the deepest nesting is %d",
				fn.MaxNesting,
			)
			fs = append(fs, f)
		}
	}
	for i := range s.p.Types {
		typ := &s.p.Types[i]
		t := limitsOf(limits, typ.Package)
		if typ.Lines <= t.LargeTypeLines {
			continue
		}
		f := NewFinding(
			FindingID{Rule: RuleLargeType, Subject: typ.ID, Object: ""},
			SeverityWarning,
		)
		f.Criterion, f.Location = "1.3.1", &typ.Span
		f.Message = fmt.Sprintf("%s has %d lines of code with its methods, above %d",
			typ.ID, typ.Lines, t.LargeTypeLines)
		f.Fix = "split the type along the responsibilities its names and methods suggest"
		if lcom := cohesion[typ.ID]; lcom > 1 {
			f.Fix = fmt.Sprintf("split the type along its %d independent groups of methods and "+
				"fields (LCOM4 = %d)", lcom, lcom)
		}
		fs = append(fs, f)
	}
	return fs
}

// lengthAndDepthRules reports source files above the length limit and functions
// nested deeper than the indentation limit of their package: von
// Zitzewitz's golden rules for files of at most 800 lines and indentation of
// at most 4 (mmi.md §7.3).
func lengthAndDepthRules(s *subject, limits map[string]Thresholds) []Finding {
	var fs []Finding
	for i := range s.p.Files {
		file := &s.p.Files[i]
		t := limitsOf(limits, file.Package)
		if file.Lines <= t.LargeFileLines {
			continue
		}
		f := NewFinding(
			FindingID{Rule: RuleLargeFile, Subject: file.Path, Object: ""},
			SeverityWarning,
		)
		f.Location = &Span{File: file.Path, Start: 1, End: 1}
		f.Message = fmt.Sprintf(
			"%s has %d lines of code, above %d",
			file.Path,
			file.Lines,
			t.LargeFileLines,
		)
		f.Fix = "split the file by the concepts it holds: one major concept per file"
		fs = append(fs, f)
	}
	for i := range s.p.Funcs {
		fn := &s.p.Funcs[i]
		t := limitsOf(limits, fn.Package)
		if fn.MaxNesting <= t.MaxNesting {
			continue
		}
		f := NewFinding(
			FindingID{Rule: RuleDeepNesting, Subject: fn.ID, Object: ""},
			SeverityWarning,
		)
		f.Location = &fn.Span
		f.Message = fmt.Sprintf(
			"%s nests %d levels deep, above %d",
			fn.ID,
			fn.MaxNesting,
			t.MaxNesting,
		)
		f.Fix = "return early, and extract the innermost loop or branch into a named function"
		fs = append(fs, f)
	}
	return append(fs, cognitiveRule(s, limits)...)
}

// cognitiveRule reports functions above the cognitive-complexity limit:
// hard to read rather than hard to test, which cyclomatic complexity
// measures (SonarSource, "Cognitive Complexity", 2017).
func cognitiveRule(s *subject, limits map[string]Thresholds) []Finding {
	var fs []Finding
	for i := range s.p.Funcs {
		fn := &s.p.Funcs[i]
		t := limitsOf(limits, fn.Package)
		if fn.Cognitive <= t.HighCognitive {
			continue
		}
		f := NewFinding(FindingID{Rule: RuleCognitive, Subject: fn.ID}, SeverityWarning)
		f.Location = &fn.Span
		f.Message = fmt.Sprintf("%s has cognitive complexity %d, above %d",
			fn.ID, fn.Cognitive, t.HighCognitive)
		f.Fix = "flatten nested decisions with early returns, and extract nested branches " +
			"and long boolean chains into named functions"
		fs = append(fs, f)
	}
	return fs
}

// limitsOf returns a package's limits.
//
// Requires: limits has every package of the program.
func limitsOf(limits map[string]Thresholds, pkg string) *Thresholds {
	t := limits[pkg]
	return &t
}

// importsBetween returns the in-scope imports from packages of one subsystem
// to packages of another, in program order.
func (s *subject) importsBetween(from, to string) []Import {
	var imps []Import
	for _, imp := range s.inScopeImports() {
		if s.g.SubsystemOf[imp.From] == from && s.g.SubsystemOf[imp.To] == to {
			imps = append(imps, imp)
		}
	}
	return imps
}

// Of returns the chosen variant of a function's complexity; the modified
// variant when none is chosen.
func (c Complexity) Of(v CCVariant) int {
	switch v {
	case CCPlain:
		return c.Plain
	case CCExtended:
		return c.Extended
	case CCModified:
		return c.Modified
	default:
		return c.Modified
	}
}

// Plural returns "1 import" or "2 imports".
func Plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// subsystemCycleSites returns where each import between two subsystems of a
// cycle group is: a change to any of them may have closed the cycle.
func (s *subject) subsystemCycleSites(grp []string) []Span {
	in := map[string]bool{}
	for _, sub := range grp {
		in[sub] = true
	}
	var at []Position
	for _, imp := range s.inScopeImports() {
		from, to := s.g.SubsystemAssignment.Of[imp.From], s.g.SubsystemAssignment.Of[imp.To]
		if from != to && in[from] && in[to] {
			at = append(at, imp.Pos)
		}
	}
	return spansAt(at)
}

// typeCycleSites returns where the first use of each reference within a type
// cycle group is.
func typeCycleSites(refs []Ref, grp []string) []Span {
	in := map[string]bool{}
	for _, t := range grp {
		in[t] = true
	}
	var at []Position
	for _, r := range refs {
		if in[r.From] && in[r.To] {
			at = append(at, r.Pos)
		}
	}
	return spansAt(at)
}

// spansAt returns the one-line spans of positions, sorted and without
// duplicates.
func spansAt(at []Position) []Span {
	spans := make([]Span, 0, len(at))
	for _, p := range at {
		spans = append(spans, *SpanAt(p))
	}
	slices.SortFunc(spans, func(a, b Span) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Start, b.Start))
	})
	return slices.Compact(spans)
}

// kindsText names where a reference's uses are, " in field and body", for a
// fix to say how hard the cut is; "" when the kinds are unknown.
func kindsText(kinds []RefKind) string {
	if len(kinds) == 0 {
		return ""
	}
	names := make([]string, len(kinds))
	for i, k := range kinds {
		names[i] = string(k)
	}
	if len(names) == 1 {
		return " in " + names[0]
	}
	return " in " + strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
