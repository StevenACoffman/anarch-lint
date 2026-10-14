package arch_test

import (
	"cmp"
	"slices"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// clusters is two groups of three packages that all import each other, a and
// b, joined by one import, and m/b/glue, declared in b, that imports and is
// imported by every package of a.
func clusters() *arch.Program {
	p := &arch.Program{}
	add := func(from, to string) {
		p.Imports = append(p.Imports, arch.Import{From: from, To: to})
	}
	group := func(pkgs ...string) {
		for _, from := range pkgs {
			for _, to := range pkgs {
				if from != to {
					add(from, to)
				}
			}
		}
	}
	a := []string{"m/a/one", "m/a/two", "m/a/three"}
	b := []string{"m/b/one", "m/b/two", "m/b/three"}
	group(a...)
	group(b...)
	add("m/a/one", "m/b/one")
	for _, pkg := range a {
		add("m/b/glue", pkg)
		add(pkg, "m/b/glue")
	}
	for _, path := range slices.Concat(a, b, []string{"m/b/glue", "m/lone"}) {
		p.Packages = append(p.Packages, arch.Package{Path: path})
	}
	slices.SortFunc(p.Packages, func(x, y arch.Package) int { return cmp.Compare(x.Path, y.Path) })
	slices.SortFunc(p.Imports, func(x, y arch.Import) int {
		return cmp.Or(cmp.Compare(x.From, y.From), cmp.Compare(x.To, y.To))
	})
	return p
}

func TestCommunities(t *testing.T) {
	t.Parallel()
	m := arch.Model{Version: 1, Subsystems: []arch.Subsystem{
		{Name: "a", Packages: []string{"m/a/..."}},
		{Name: "b", Packages: []string{"m/b/..."}},
	}}
	r := arch.Analyze(&arch.Input{Program: *clusters(), Model: m})
	equals(t, arch.Communities{
		Groups: []arch.Community{
			{Packages: []string{"m/a/one", "m/a/three", "m/a/two", "m/b/glue"}, Subsystem: "a"},
			{Packages: []string{"m/b/one", "m/b/three", "m/b/two"}, Subsystem: "b"},
			{Packages: []string{"m/lone"}},
		},
		Misplaced: []arch.Misplaced{{Package: "m/b/glue", Subsystem: "b", Community: "a"}},
	}, r.Communities)
}

// A package that is a subsystem of its own has no siblings to sit apart
// from, so it is never misplaced.
func TestCommunitiesSkipOnePackageSubsystems(t *testing.T) {
	t.Parallel()
	m := arch.Model{Version: 1, Subsystems: []arch.Subsystem{
		{Name: "glue", Packages: []string{"m/b/glue"}},
		{Name: "a", Packages: []string{"m/a/..."}},
		{Name: "b", Packages: []string{"m/b/..."}},
	}}
	r := arch.Analyze(&arch.Input{Program: *clusters(), Model: m})
	equals(t, []arch.Misplaced{}, r.Communities.Misplaced)
}

// TestCommunitiesAreDeterministic: the same graph gives the same
// communities on every run.
func TestCommunitiesAreDeterministic(t *testing.T) {
	t.Parallel()
	first := arch.Analyze(&arch.Input{Program: *clusters(), Model: arch.Model{Version: 1}})
	for range 20 {
		r := arch.Analyze(&arch.Input{Program: *clusters(), Model: arch.Model{Version: 1}})
		equals(t, first.Communities, r.Communities)
	}
}
