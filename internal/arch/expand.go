package arch

import (
	"slices"
	"strings"
)

// Expand returns the model with each subsystem template, a subsystem whose
// name refers to a capture such as "$1", replaced by one subsystem per
// distinct capture among the packages: ArchUnit's slices for subsystems, so
// "example.com/app/$*/..." named "$1" makes a subsystem of each top-level
// area, and cycle and interface checks need no hand-written list (TODO.md,
// "Groups defined by a capture"). A template whose patterns match nothing is
// kept as it is, so its stale pattern is reported. The expanded model keeps
// the declared model's hash: a new area does not orphan the baseline.
// m is not modified.
func (m *Model) Expand(packages []string) Model {
	out := *m
	out.hash = m.Hash()
	out.Subsystems = make([]Subsystem, 0, len(m.Subsystems))
	for _, s := range m.Subsystems {
		if !s.IsTemplate() {
			out.Subsystems = append(out.Subsystems, s)
			continue
		}
		instances := s.instances(packages)
		if len(instances) == 0 {
			instances = []Subsystem{s}
		}
		out.Subsystems = append(out.Subsystems, instances...)
	}
	return out
}

// IsTemplate reports a subsystem whose name refers to a capture of its
// package patterns.
func (s *Subsystem) IsTemplate() bool { return highestReference(s.Name) > 0 }

// instances returns one subsystem per distinct capture of the template's
// package patterns among packages, sorted by name.
func (s *Subsystem) instances(packages []string) []Subsystem {
	seen := map[string]bool{}
	var out []Subsystem
	for _, pkg := range packages {
		for _, pat := range s.Packages {
			_, captures, ok := MatchCaptures(pat, pkg)
			if !ok {
				continue
			}
			if key := strings.Join(captures, "\x00"); !seen[key] {
				seen[key] = true
				out = append(out, s.instance(captures))
			}
			break
		}
	}
	slices.SortFunc(out, func(a, b Subsystem) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// instance is the template with its captures filled in: each "$*" or "$**"
// of a package pattern by the capture in its place, and each "$n" of the
// name, interface and owner by the capture it names.
func (s *Subsystem) instance(captures []string) Subsystem {
	inst := Subsystem{
		Name: Substitute(s.Name, captures), Owner: Substitute(s.Owner, captures),
	}
	for _, pat := range s.Packages {
		inst.Packages = append(inst.Packages, instantiate(pat, captures))
	}
	for _, pat := range s.Interface {
		inst.Interface = append(inst.Interface, instantiate(Substitute(pat, captures), captures))
	}
	return inst
}

// instantiate replaces the pattern's "$*" and "$**" segments, in order, with
// the captures; a segment past the last capture is left as it is.
func instantiate(pattern string, captures []string) string {
	root, rest := strings.CutSuffix(pattern, tokenRest)
	segs := strings.Split(root, "/")
	next := 0
	for i, seg := range segs {
		if (seg == tokenOne || seg == tokenMany) && next < len(captures) {
			segs[i] = captures[next]
			next++
		}
	}
	out := strings.Join(segs, "/")
	if rest {
		out += tokenRest
	}
	return out
}
