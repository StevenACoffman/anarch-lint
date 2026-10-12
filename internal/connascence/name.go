package connascence

import (
	"fmt"
	"maps"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// nameDetector reports each object referred to by name from another
// subsystem (connascence.md §2.1). Connascence of name is the weakest kind
// and unavoidable, so it is never a finding by itself unless a model limits
// strength to nothing; its degree is the cost of renaming the object.
func nameDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	for _, object := range slices.Sorted(maps.Keys(s.usesOf)) {
		uses := s.usesOf[object]
		home := s.c.SubsystemOf(arch.Site{Package: uses[0].Package})
		crosses := slices.ContainsFunc(
			uses,
			func(u arch.Use) bool { return s.c.SubsystemOf(u.Site) != home },
		)
		if !crosses {
			continue
		}
		sites := s.usesFrom(object)
		out = append(out, s.instance(&instanceSpec{
			kind:     arch.StrengthName,
			detector: "name.cross-subsystem",
			subject:  object,
			object:   object,
			basis:    arch.BasisFact,
			sites:    sites,
			message: fmt.Sprintf("%d places in %d packages refer to %s by name",
				len(sites), distinct(sites, sitePackage), object),
			fix: fmt.Sprintf("connascence of name is the weakest kind; keep %s stable, and "+
				"reach it only through its subsystem's interface", lastName(object)),
		}))
	}
	return out
}

// typeDetector reports each in-scope named type that crosses a package
// boundary in an exported signature or a struct field (connascence.md §2.2).
func typeDetector(s *subject) []arch.Connascence {
	crossings := s.typeCrossings()
	var out []arch.Connascence
	for _, named := range slices.Sorted(maps.Keys(crossings)) {
		sites := crossings[named]
		if d, ok := s.decl[named]; ok {
			sites = append(sites, d)
		}
		out = append(out, s.instance(&instanceSpec{
			kind:     arch.StrengthType,
			detector: "type.cross-package",
			subject:  named,
			object:   named,
			basis:    arch.BasisFact,
			sites:    sites,
			message: fmt.Sprintf(
				"%s appears in the signatures or fields of %s in other packages",
				named,
				count(len(crossings[named]), "place", "places"),
			),
			fix: "a domain type crossing a boundary is connascence of type, which is desirable; " +
				"keep the type small and stable, or cross the boundary with a contract type",
		}))
	}
	return out
}

// typeCrossings maps each in-scope named type to the places in other packages
// whose exported signatures or struct fields mention it.
func (s *subject) typeCrossings() map[string][]arch.Site {
	crossings := map[string][]arch.Site{}
	s.signatureCrossings(crossings)
	for i := range s.p.Facts.Uses {
		u := &s.p.Facts.Uses[i]
		if u.Kind == arch.ObjectType && u.Site.Func == "" && u.Site.Type != "" &&
			u.Site.Package != u.Package {
			crossings[u.Object] = append(crossings[u.Object], u.Site)
		}
	}
	return crossings
}

// signatureCrossings adds the exported signatures that mention another
// in-scope package's named type.
func (s *subject) signatureCrossings(crossings map[string][]arch.Site) {
	for i := range s.p.Facts.Signatures {
		sig := &s.p.Facts.Signatures[i]
		if !sig.Exported {
			continue
		}
		for _, ref := range slices.Concat(sig.Params, sig.Results) {
			if ref.Named != "" && ref.Package != sig.Site.Package && s.inScope(ref.Package) {
				crossings[ref.Named] = append(crossings[ref.Named], sig.Site)
			}
		}
	}
}
