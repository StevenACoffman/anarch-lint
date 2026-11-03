package arch

import (
	"fmt"
	"slices"
	"strings"
)

// cutStep describes cutting one edge between two subsystems of a cycle
// group: what the edge carries, aggregated by file and kind so the size of
// the cut shows (IMPROVEMENTS.md §2.5), and the shortest cycle it closes,
// so the fix names the whole loop (§2.4).
func (s *subject) cutStep(cut Edge, imps []Import, sub Graph) string {
	files := make([]string, 0, len(imps))
	for _, imp := range imps {
		if imp.Pos.File != "" {
			files = append(files, imp.Pos.File)
		}
	}
	slices.Sort(files)
	files = slices.Compact(files)
	what := Plural(len(imps), "import")
	if len(files) > 0 {
		what += " in " + strings.Join(files, ", ")
	}
	if uses := s.typeUses(cut.From, cut.To); uses != "" {
		what += "; " + uses
	}
	return fmt.Sprintf("%s → %s (%s)", cut.From, cut.To, what) + closesText(sub, cut)
}

// closesText names the shortest cycle a cut edge closes, ", which closes a
// → b → a"; "" when it closes none.
func closesText(g Graph, cut Edge) string {
	c := g.ClosingCycle(cut.From, cut.To)
	if c == nil {
		return ""
	}
	return ", which closes " + strings.Join(append(c, c[0]), " → ")
}

// typeUses counts the uses of types of the subsystem to by types of the
// subsystem from, by the kinds of use: "9 type uses, as call and field";
// "" when none.
func (s *subject) typeUses(from, to string) string {
	uses := 0
	var kinds []RefKind
	for _, r := range s.p.Refs {
		if s.g.SubsystemOf[FuncPackage(r.From)] == from &&
			s.g.SubsystemOf[FuncPackage(r.To)] == to {
			uses += r.Weight
			kinds = append(kinds, r.Kinds...)
		}
	}
	if uses == 0 {
		return ""
	}
	slices.Sort(kinds)
	if kinds = slices.Compact(kinds); len(kinds) == 0 {
		return Plural(uses, "type use")
	}
	return Plural(uses, "type use") + ", as " + kindsList(kinds)
}
