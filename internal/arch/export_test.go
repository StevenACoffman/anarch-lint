package arch

// Locality and degree are computed on the way to a connascence report; these
// expose them to the external tests of their tables (connascence.md §1.2,
// §1.3).

// LocalityOf is Containers.LocalityOf over explicit subsystem and module maps.
func LocalityOf(sites []Site, subsystemOf, moduleOf map[string]string) Locality {
	return Containers{subsystemOf: subsystemOf, moduleOf: moduleOf}.LocalityOf(sites)
}

// DegreeOf is Containers.DegreeOf over explicit subsystem and module maps.
func DegreeOf(sites []Site, l Locality, subsystemOf, moduleOf map[string]string) int {
	return Containers{subsystemOf: subsystemOf, moduleOf: moduleOf}.DegreeOf(sites, l)
}
