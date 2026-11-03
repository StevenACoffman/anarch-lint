package arch

import (
	"cmp"
	"path"
	"slices"
	"strconv"
)

// StarterModel drafts a model for a module that has none, from the
// communities of its import graph (IMPROVEMENTS.md §9.3): each community
// is a subsystem listing its packages, named after its shortest package
// path, and when the subsystems depend on each other without a cycle, the
// subsystem order runs from the dependents down. It is a draft for a person
// to rename, merge and give reasons and layers: communities shift with
// small edits, so it says how the code is coupled today, not how it should
// be.
//
// Requires: p is sorted, as [Program] documents.
func StarterModel(p *Program) Model {
	g := BuildGraphs(p, &Model{Version: 1})
	parts := louvainParts(&g.Packages)
	slices.SortFunc(parts, func(a, b []string) int { return cmp.Compare(a[0], b[0]) })
	m := Model{Version: 1}
	subsystemOf := map[string]string{}
	taken := map[string]int{}
	for _, part := range parts {
		name := starterName(part, taken)
		for _, pkg := range part {
			subsystemOf[pkg] = name
		}
		m.Subsystems = append(m.Subsystems, Subsystem{Name: name, Packages: part})
	}
	var edges []Edge
	for _, e := range g.Packages.Edges() {
		edges = append(edges, Edge{From: subsystemOf[e.From], To: subsystemOf[e.To], Weight: 1})
	}
	names := make([]string, len(m.Subsystems))
	for i, s := range m.Subsystems {
		names[i] = s.Name
	}
	m.SubsystemOrder = topDown(NewGraph(names, edges))
	return m
}

// starterName names a community after its shortest package path's last
// element, the first such path by name, with a number after a name another
// community took.
func starterName(part []string, taken map[string]int) string {
	shortest := slices.MinFunc(part, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(a), len(b)), cmp.Compare(a, b))
	})
	name := path.Base(shortest)
	taken[name]++
	if n := taken[name]; n > 1 {
		return name + "-" + strconv.Itoa(n)
	}
	return name
}

// topDown orders an acyclic graph's nodes so that each comes before every
// node it depends on, ties by name; nil when the graph has a cycle, since
// then no order holds.
func topDown(g Graph) []string {
	if len(g.CycleGroups()) > 0 {
		return nil
	}
	indegree := make([]int, g.Len())
	for _, arcs := range g.out {
		for _, a := range arcs {
			indegree[a.to]++
		}
	}
	var ready []int
	for v, d := range indegree {
		if d == 0 {
			ready = append(ready, v)
		}
	}
	var out []string
	for len(ready) > 0 {
		v := ready[0]
		ready = ready[1:]
		out = append(out, g.nodes[v])
		for _, a := range g.out[v] {
			indegree[a.to]--
			if indegree[a.to] == 0 {
				ready = append(ready, a.to)
			}
		}
		slices.Sort(ready) // nodes are sorted by name, so positions are too
	}
	return out
}
