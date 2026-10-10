package arch

import (
	"fmt"
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
		f := newFinding(RuleSubsystemCycle, SeverityError, subject, "")
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
				f.Location = spanAt(imps[0].Pos)
			}
			steps = append(
				steps,
				fmt.Sprintf("%s → %s (%s)", cut.From, cut.To, plural(cut.Weight, "import")),
			)
		}
		f.Fix = "cut " + strings.Join(steps, "; ") + ": move each importing package to the " +
			"subsystem it serves, or invert the import through an interface"
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
		f := newFinding(RuleTypeCycleTooLarge, SeverityError, grp[0], strconv.Itoa(len(grp)))
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
				f.Location = spanAt(r.Pos)
			}
			steps = append(steps, fmt.Sprintf("%s → %s (%s, %s:%d)",
				cut.From, cut.To, plural(cut.Weight, "use"), r.Pos.File, r.Pos.Line))
		}
		method := "a minimum"
		if !exact {
			method = "a heuristic"
		}
		f.Evidence = grp
		f.Fix = fmt.Sprintf("cut these references (%s set): %s; invert each through an interface "+
			"(dependency inversion) or move the code it needs", method, strings.Join(steps, "; "))
		fs = append(fs, f)
	}
	return fs
}

// sizeRules reports functions and types above the size and complexity limits
// (MMI 1.3.1, 1.3.2, 1.3.4).
func sizeRules(s *subject, t Thresholds, cohesion map[string]int) []Finding {
	var fs []Finding
	for i := range s.p.Funcs {
		fn := &s.p.Funcs[i]
		if fn.Lines > t.LargeFuncLines {
			f := newFinding(RuleLargeFunc, SeverityWarning, fn.ID, "")
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
		if cc := variantOf(fn.Complexity, t.ComplexityVariant); cc > t.HighComplexity {
			f := newFinding(RuleComplexFunc, SeverityWarning, fn.ID, "")
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
		if typ.Lines <= t.LargeTypeLines {
			continue
		}
		f := newFinding(RuleLargeType, SeverityWarning, typ.ID, "")
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
// nested deeper than the indentation limit: von Zitzewitz's golden rules for
// files of at most 800 lines and indentation of at most 4 (mmi.md §7.3).
func lengthAndDepthRules(s *subject, t Thresholds) []Finding {
	var fs []Finding
	for i := range s.p.Files {
		file := &s.p.Files[i]
		if file.Lines <= t.LargeFileLines {
			continue
		}
		f := newFinding(RuleLargeFile, SeverityWarning, file.Path, "")
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
		if fn.MaxNesting <= t.MaxNesting {
			continue
		}
		f := newFinding(RuleDeepNesting, SeverityWarning, fn.ID, "")
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
	return fs
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

// variantOf returns the chosen variant of a function's complexity.
func variantOf(c Complexity, v CCVariant) int {
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

// plural returns "1 import" or "2 imports".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
