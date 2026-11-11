package fitness

import (
	"cmp"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Constructs a check cannot see through: each is a reason a passing rule may
// still be broken, a defeater a reviewer can eliminate (IMPROVEMENTS.md
// §10.3).
const (
	// unseenLinkname is a //go:linkname directive, which reaches a symbol of
	// another package without importing it.
	unseenLinkname = "linkname"
	// unseenPlugin is an import of plugin, which loads code at run time.
	unseenPlugin = "plugin"
	// unseenReflection is a reflect lookup of a field or method by name.
	unseenReflection = "reflection-by-name"
	// unseenUnsafe is a conversion between unsafe.Pointer and a pointer to a
	// named type.
	unseenUnsafe = "unsafe-conversion"
	// unseenCgo is an import of C, whose calls go to code no rule reads.
	unseenCgo = "cgo"
	// unseenUnresolved is a call through an interface or a function value
	// that the VTA call graph resolves to nothing, such as one through a
	// value only a caller outside the analysis supplies. Only a calls rule
	// that reads dynamic calls counts it.
	unseenUnresolved = "unresolved-dynamic-call"
)

// Unseen is one construct no check sees through, where it is.
type Unseen struct {
	Construct string `json:"construct"`
	Package   string `json:"package"`
	File      string `json:"file"`
	Line      int    `json:"line"`
}

// blindSpots lists, by rule kind, the constructs that can hide what the
// kind's check reads: a linkname or a plugin hides a dependency from every
// kind, and may be the dependency an expects rule finds missing; reflection
// by name hides a call, a field access or a use; an unsafe conversion hides
// a field access or a use; cgo hides a call and the libraries C links. Kinds
// that read no code, such as layers, are absent.
func blindSpots() map[string][]string {
	return map[string][]string{
		"calls":          {unseenCgo, unseenLinkname, unseenPlugin, unseenReflection},
		"fields":         {unseenPlugin, unseenReflection, unseenUnsafe},
		"interfacesOnly": {unseenPlugin, unseenReflection, unseenUnsafe},
		"unusedExports":  {unseenLinkname, unseenPlugin, unseenReflection},
		"libraries":      {unseenCgo, unseenLinkname, unseenPlugin},
		"fanOut":         {unseenLinkname, unseenPlugin},
		"dependents":     {unseenLinkname, unseenPlugin},
		"expects":        {unseenLinkname, unseenPlugin},
	}
}

// unseenOf returns the constructs of the program's build that no check sees
// through, sorted by package, file, line and construct.
func unseenOf(p *arch.Program) []Unseen {
	out := make([]Unseen, 0)
	add := func(construct string, s *arch.Site) {
		out = append(
			out,
			Unseen{Construct: construct, Package: s.Package, File: s.File, Line: s.Line},
		)
	}
	for i := range p.Facts.Linknames {
		add(unseenLinkname, &p.Facts.Linknames[i].Site)
	}
	for i := range p.Facts.ReflectByNames {
		add(unseenReflection, &p.Facts.ReflectByNames[i].Site)
	}
	for i := range p.Facts.UnresolvedCalls {
		add(unseenUnresolved, &p.Facts.UnresolvedCalls[i])
	}
	for i := range p.Facts.UnsafeConvs {
		add(unseenUnsafe, &p.Facts.UnsafeConvs[i].Site)
	}
	for _, imp := range p.Imports {
		construct := map[string]string{"C": unseenCgo, "plugin": unseenPlugin}[imp.To]
		if construct != "" && !imp.Inactive && !imp.Test {
			add(construct, &arch.Site{Package: imp.From, File: imp.Pos.File, Line: imp.Pos.Line})
		}
	}
	slices.SortFunc(out, func(a, b Unseen) int {
		return cmp.Or(cmp.Compare(a.Package, b.Package), cmp.Compare(a.File, b.File),
			cmp.Compare(a.Line, b.Line), cmp.Compare(a.Construct, b.Construct))
	})
	return out
}

// unseenBy counts, by construct, those of unseen that the rule cannot see
// through and that lie in a package whose code it reads; nil for a rule
// that reads no code, or where there are none.
func (r *rule) unseenBy(unseen []Unseen) map[string]int {
	if r.readsCode == nil || len(r.blind) == 0 {
		return nil
	}
	var out map[string]int
	for _, u := range unseen {
		if slices.Contains(r.blind, u.Construct) && r.readsCode(u.Package) {
			if out == nil {
				out = map[string]int{}
			}
			out[u.Construct]++
		}
	}
	return out
}

// everyPackage reads the code of every package: a rule whose violations can
// come from anywhere, such as one naming who alone may call.
func everyPackage(string) bool { return true }

// matching reads the code of the packages the patterns match.
func matching(patterns []string) func(string) bool {
	return func(pkg string) bool { return arch.MatchesAny(patterns, pkg) }
}
