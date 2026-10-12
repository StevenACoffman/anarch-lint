package arch

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
)

// Localities, from the nearest container to the farthest (connascence.md
// §1.2). The order matters: an instance's locality is the first of them that
// holds every site. A type and a file are not nested, since a type's methods
// may span files and a file may hold several types; taking the first level
// that holds every site keeps locality well defined.
const (
	LocalityFunction Locality = iota
	LocalityType
	LocalityFile
	LocalityPackage
	LocalitySubsystem
	LocalitySystem
	LocalityExternal
)

// Locality is how far apart the sites of one coupling lie: the nearest
// container that holds all of them. "System" is one Go module across
// subsystems; "external" spans several Go modules.
type Locality int

// Containers says which subsystem and which Go module each package belongs
// to: the two levels of locality a site does not carry itself.
type Containers struct {
	subsystemOf map[string]string
	moduleOf    map[string]string
}

// localityNames are the text forms of the localities, in order.
func localityNames() []string {
	return []string{"function", "type", "file", "package", "subsystem", "system", "external"}
}

// String returns the locality's name, as the model and the reports spell it.
func (l Locality) String() string {
	if names := localityNames(); l >= 0 && int(l) < len(names) {
		return names[l]
	}
	return fmt.Sprintf("locality(%d)", int(l))
}

// MarshalText writes the locality's name, so a map keyed by locality encodes
// as {"package": 9}.
func (l Locality) MarshalText() ([]byte, error) {
	if l < LocalityFunction || l > LocalityExternal {
		return nil, &Error{Code: EINVALID, Message: fmt.Sprintf("%s is not a locality", l)}
	}
	return []byte(l.String()), nil
}

// UnmarshalText reads a locality's name; any other text is EINVALID.
func (l *Locality) UnmarshalText(text []byte) error {
	i := slices.Index(localityNames(), string(text))
	if i < 0 {
		return &Error{Code: EINVALID, Message: fmt.Sprintf("%q is not a locality", text)}
	}
	*l = Locality(i)
	return nil
}

// NewContainers returns the Containers of p's packages under the graphs g.
// A package no subsystem claims is its own subsystem, as in the subsystem
// graph.
func NewContainers(p *Program, g *Graphs) Containers {
	c := Containers{subsystemOf: g.SubsystemOf, moduleOf: map[string]string{}}
	for _, pkg := range p.Packages {
		c.moduleOf[pkg.Path] = pkg.Module
	}
	return c
}

// LocalityOf returns the nearest container that holds every site: the first
// level at which every site has the same key. A site outside any function or
// type has no key at those two levels.
//
// Requires: sites is not empty.
func (c Containers) LocalityOf(sites []Site) Locality {
	levels := []struct {
		locality Locality
		key      func(Site) string
	}{
		{LocalityFunction, func(s Site) string { return s.Func }},
		{LocalityType, func(s Site) string { return s.Type }},
		{LocalityFile, func(s Site) string { return "file " + s.File }},
		{LocalityPackage, func(s Site) string { return "package " + s.Package }},
		{LocalitySubsystem, func(s Site) string { return "subsystem " + c.SubsystemOf(s) }},
		{LocalitySystem, func(s Site) string { return "module " + c.moduleOf[s.Package] }},
	}
	for _, lv := range levels {
		if sameKey(sites, lv.key) {
			return lv.locality
		}
	}
	return LocalityExternal
}

// sameKey reports whether every site has the first site's key, and that key
// is not empty.
func sameKey(sites []Site, key func(Site) string) bool {
	first := key(sites[0])
	if first == "" {
		return false
	}
	for _, s := range sites[1:] {
		if key(s) != first {
			return false
		}
	}
	return true
}

// DegreeOf counts the distinct entities one level below locality l among the
// sites: the number of things that must change together (connascence.md
// §1.3). Inside one function, every site counts.
func (c Containers) DegreeOf(sites []Site, l Locality) int {
	entity := func(s Site) string {
		switch l {
		case LocalityFunction:
			return s.File + ":" + strconv.Itoa(s.Line)
		case LocalityType:
			return s.Func // the declaration itself has none
		case LocalityFile:
			return cmp.Or(s.Func, s.Type, "package-level")
		case LocalityPackage:
			return s.File
		case LocalitySubsystem:
			return s.Package
		case LocalitySystem:
			return c.SubsystemOf(s)
		default:
			return c.moduleOf[s.Package]
		}
	}
	seen := map[string]bool{}
	for _, s := range sites {
		seen[entity(s)] = true
	}
	return len(seen)
}

// SubsystemOf returns the subsystem of a site's package: its own path
// when no subsystem claims it.
func (c Containers) SubsystemOf(s Site) string {
	if sub, ok := c.subsystemOf[s.Package]; ok {
		return sub
	}
	return s.Package
}

// Module returns the Go module of an in-scope package, and false for a
// package outside the analysis.
func (c Containers) Module(pkg string) (string, bool) {
	m, ok := c.moduleOf[pkg]
	return m, ok
}
