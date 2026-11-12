package arch

import "slices"

// ClusterFactor is the TurboMQ cluster factor of one module (docs/adr/0021):
// its internal edge weight against the weight of the edges crossing its
// boundary. It satisfies Briand, Morasca and Basili's four cohesion
// properties.
type ClusterFactor struct {
	// Module is the module's name.
	Module string `json:"module"`
	// Internal is μ, the summed weight of the edges between its members.
	Internal int `json:"internal"`
	// Boundary is Σ(εij + εji), the summed weight of the edges with exactly
	// one end in the module, in either direction.
	Boundary int `json:"boundary"`
	// Factor is 2μ ÷ (2μ + Σ(εij + εji)), or 0 when μ is 0, rounded to six
	// decimal places.
	Factor float64 `json:"factor"`
}

// DependencyProfile is Bouwers, van Deursen and Visser's dependency profile
// (docs/adr/0021): the size of the elements in each of four categories, by
// where their edges cross a module boundary. The categories partition the
// profiled elements.
type DependencyProfile struct {
	// Internal is the size of the elements with no edge crossing their
	// module's boundary: hidden inside it.
	Internal int `json:"internal"`
	// Inbound is the size of the elements only reached from outside their
	// module: its provided interface.
	Inbound int `json:"inbound"`
	// Outbound is the size of the elements that only reach outside their
	// module: its required interface.
	Outbound int `json:"outbound"`
	// Transit is the size of the elements both reached from and reaching
	// outside their module.
	Transit int `json:"transit"`
}

// ClusterFactorsOf returns the cluster factor of every module moduleOf
// names, sorted by module. An element moduleOf leaves out belongs to no
// module, so an edge to or from it crosses the boundary of the module at its
// other end.
func ClusterFactorsOf(g Graph, moduleOf map[string]string) []ClusterFactor {
	internal, boundary := map[string]int{}, map[string]int{}
	for _, e := range g.Edges() {
		if m, ok := withinModule(e, moduleOf); ok {
			internal[m] += e.Weight
			continue
		}
		if m, ok := moduleOf[e.From]; ok {
			boundary[m] += e.Weight
		}
		if m, ok := moduleOf[e.To]; ok {
			boundary[m] += e.Weight
		}
	}
	out := make([]ClusterFactor, 0)
	for _, m := range sortedModules(moduleOf) {
		cf := ClusterFactor{Module: m, Internal: internal[m], Boundary: boundary[m]}
		if cf.Internal > 0 {
			cf.Factor = round6(float64(2*cf.Internal) / float64(2*cf.Internal+cf.Boundary))
		}
		out = append(out, cf)
	}
	return out
}

// Total is the size of every profiled element.
func (d DependencyProfile) Total() int {
	return d.Internal + d.Inbound + d.Outbound + d.Transit
}

// DependencyProfileOf profiles the elements of g that moduleOf assigns to a
// module, weighing each by sizeOf (an element sizeOf leaves out weighs 0). An
// edge to or from an element moduleOf leaves out crosses the boundary.
func DependencyProfileOf(
	g Graph,
	moduleOf map[string]string,
	sizeOf map[string]int,
) DependencyProfile {
	inbound, outbound := map[string]bool{}, map[string]bool{}
	for _, e := range g.Edges() {
		if _, ok := withinModule(e, moduleOf); !ok {
			outbound[e.From] = true
			inbound[e.To] = true
		}
	}
	var d DependencyProfile
	for _, n := range g.Nodes() {
		if _, ok := moduleOf[n]; ok {
			d.add(inbound[n], outbound[n], sizeOf[n])
		}
	}
	return d
}

// add counts size toward the category its element's boundary crossings
// place it in.
func (d *DependencyProfile) add(inbound, outbound bool, size int) {
	switch {
	case inbound && outbound:
		d.Transit += size
	case inbound:
		d.Inbound += size
	case outbound:
		d.Outbound += size
	default:
		d.Internal += size
	}
}

// withinModule returns the module holding both ends of e, and whether there
// is one.
func withinModule(e Edge, moduleOf map[string]string) (string, bool) {
	from, okFrom := moduleOf[e.From]
	to, okTo := moduleOf[e.To]
	return from, okFrom && okTo && from == to
}

// sortedModules returns the distinct modules moduleOf names, sorted.
func sortedModules(moduleOf map[string]string) []string {
	modules := make([]string, 0, len(moduleOf))
	for _, m := range moduleOf {
		modules = append(modules, m)
	}
	slices.Sort(modules)
	return slices.Compact(modules)
}
