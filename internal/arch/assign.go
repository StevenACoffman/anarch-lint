package arch

import "slices"

// Assignment maps packages to the groups of a model — subsystems, layers, or
// the packages a role or rule names — by import-path pattern.
type Assignment struct {
	// Of maps each assigned package to its group.
	Of map[string]string
	// Unassigned lists the packages no pattern matches, sorted.
	Unassigned []string
	// Ambiguous lists the packages two groups match equally specifically,
	// sorted; such a package is left unassigned. Each is a model error.
	Ambiguous []string
	// Stale lists the patterns that match no package, as "group: pattern",
	// sorted (mmi.md §7.4, the stale-model completeness rule).
	Stale []string
}

// group is a named list of patterns.
type group struct {
	name     string
	patterns []string
}

// assign assigns every package to the group whose pattern matches it most
// specifically (go-arch-lint's tie-break; mmi-ref-plan.md §4.4). An exact path
// beats a "/..." pattern, and a longer pattern beats a shorter one.
//
// Requires: packages is sorted.
// Ensures: every package is in exactly one of Of, Unassigned and Ambiguous.
func assign(groups []group, packages []string) Assignment {
	a := Assignment{Of: map[string]string{}}
	used := map[string]bool{}
	for _, pkg := range packages {
		best, score, tie := bestGroup(groups, pkg, used)
		switch {
		case score < 0:
			a.Unassigned = append(a.Unassigned, pkg)
		case tie:
			a.Ambiguous = append(a.Ambiguous, pkg)
		default:
			a.Of[pkg] = best
		}
	}
	for _, g := range groups {
		for _, pat := range g.patterns {
			if key := g.name + ": " + pat; !used[key] {
				a.Stale = append(a.Stale, key)
			}
		}
	}
	slices.Sort(a.Stale)
	a.Stale = slices.Compact(a.Stale)
	return a
}

// bestGroup returns the group whose pattern matches pkg most specifically and
// that pattern's score, or a score of -1 when none matches. tie reports that a
// different group matched equally specifically. Every matching pattern is
// recorded in used, as "group: pattern".
func bestGroup(
	groups []group,
	pkg string,
	used map[string]bool,
) (best string, score int, tie bool) {
	score = -1
	for _, g := range groups {
		for _, pat := range g.patterns {
			s, ok := matchPattern(pat, pkg)
			if !ok {
				continue
			}
			used[g.name+": "+pat] = true
			switch {
			case s > score:
				best, score, tie = g.name, s, false
			case s == score && g.name != best:
				tie = true
			}
		}
	}
	return best, score, tie
}

// subsystemGroups returns the model's subsystems as groups.
func (m *Model) subsystemGroups() []group {
	gs := make([]group, len(m.Subsystems))
	for i, s := range m.Subsystems {
		gs[i] = group{name: s.Name, patterns: s.Packages}
	}
	return gs
}

// layerGroups returns the model's layers as groups.
func (m *Model) layerGroups() []group {
	gs := make([]group, len(m.Layers))
	for i, l := range m.Layers {
		gs[i] = group{name: l.Name, patterns: l.Packages}
	}
	return gs
}
