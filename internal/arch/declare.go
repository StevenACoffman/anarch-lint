package arch

import "slices"

// Declare returns the model with what the packages' directives declare: a
// package naming a subsystem joins it, a subsystem the model lacks is made,
// and a package that exposes itself joins its subsystem's interface, the
// one it names or, without a name, the one the model's patterns give it.
// The model keeps its hash, as Expand's does: the directives are code, and a
// change to them changes what is measured, not the bars. m is not modified.
//
// Requires: packages is sorted by path.
func (m *Model) Declare(packages []Package) Model {
	out := *m
	out.hash = m.Hash()
	out.Subsystems = make([]Subsystem, len(m.Subsystems))
	for i, s := range m.Subsystems {
		out.Subsystems[i] = Subsystem{
			Name: s.Name, Owner: s.Owner,
			Packages: slices.Clone(s.Packages), Interface: slices.Clone(s.Interface),
		}
	}
	groups := make([]group, len(m.Subsystems))
	for i, s := range m.Subsystems {
		groups[i] = group{name: s.Name, patterns: s.Packages}
	}
	paths := make([]string, len(packages))
	for i := range packages {
		paths[i] = packages[i].Path
	}
	assigned := assign(groups, paths)
	for i := range packages {
		out.declarePackage(&packages[i], assigned.Of[packages[i].Path])
	}
	return out
}

// declarePackage applies one package's directives, assigned being the
// subsystem the model's patterns give it.
func (m *Model) declarePackage(p *Package, assigned string) {
	name := p.Subsystem
	if name == "" && p.Exposes {
		name = assigned
	}
	if name == "" {
		return
	}
	s := m.subsystemNamed(name)
	if p.Subsystem != "" && !MatchesAny(s.Packages, p.Path) {
		s.Packages = append(s.Packages, p.Path)
	}
	if p.Exposes && !MatchesAny(s.Interface, p.Path) {
		s.Interface = append(s.Interface, p.Path)
	}
}

// subsystemNamed returns the subsystem of that name, appending an empty one
// when there is none.
func (m *Model) subsystemNamed(name string) *Subsystem {
	for i := range m.Subsystems {
		if m.Subsystems[i].Name == name {
			return &m.Subsystems[i]
		}
	}
	m.Subsystems = append(m.Subsystems, Subsystem{Name: name})
	return &m.Subsystems[len(m.Subsystems)-1]
}
