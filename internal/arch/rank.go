package arch

import "strings"

// Rank sets each finding's Reach: how many packages under analysis depend,
// directly or through others, on the package the finding is about, so a
// reader can fix first what most code rests on (IMPROVEMENTS.md §6.2). A
// finding about no package, such as one about the model, has none.
func (in *GateInput) Rank(p *Program) {
	importers := map[string][]string{}
	for _, imp := range p.Imports {
		importers[imp.To] = append(importers[imp.To], imp.From)
	}
	reach := map[string]int{}
	fileOf := map[string]string{}
	for _, f := range p.Files {
		fileOf[f.Path] = f.Package
	}
	packages := map[string]bool{}
	for _, pkg := range p.Packages {
		packages[pkg.Path] = true
		reach[pkg.Path] = len(reachable(pkg.Path, importers)) - 1
	}
	for i := range in.Findings {
		f := &in.Findings[i]
		f.Reach = reach[findingPackage(f, fileOf, packages)]
	}
}

// findingPackage returns the package a finding is about: its location's,
// its subject when that is a package, or the package its subject's ID
// starts with; "" when it is about none.
func findingPackage(f *Finding, fileOf map[string]string, packages map[string]bool) string {
	if f.Location != nil {
		if pkg, ok := fileOf[f.Location.File]; ok {
			return pkg
		}
	}
	if packages[f.Subject] {
		return f.Subject
	}
	best := ""
	for pkg := range packages {
		if strings.HasPrefix(f.Subject, pkg+".") && len(pkg) > len(best) {
			best = pkg
		}
	}
	return best
}

// reachable returns from and every node that reaches it along edges, sorted
// by discovery.
func reachable(from string, edges map[string][]string) []string {
	seen := map[string]bool{from: true}
	queue := []string{from}
	for i := 0; i < len(queue); i++ {
		for _, next := range edges[queue[i]] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return queue
}
