package balance

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Rules of the findings about distance.
const ruleOwnersDiffer = "subsystem-owners-differ"

// deployUnits is which binaries contain each package (connascence.md §9.3).
type deployUnits struct {
	// in maps a package to the main packages whose binaries contain it.
	in map[string][]string
	// binaries reports whether the scope holds any main package.
	binaries bool
	unbound  []string
}

// subsystemOwners returns each subsystem's owners: the model's Owner, or else
// the owners CODEOWNERS gives its packages. A subsystem whose packages
// resolve to different owners is a finding against the model; its owners are
// then all of them, and its owner boundary is still known.
func (s *subject) subsystemOwners() (map[string][]string, []arch.Finding) {
	out := map[string][]string{}
	var fs []arch.Finding
	explicit := map[string]string{}
	for _, sub := range s.m.Subsystems {
		explicit[sub.Name] = sub.Owner
	}
	perSub := map[string]map[string]bool{}
	for _, pkg := range s.p.Packages {
		sub := s.subsystemOf(pkg.Path)
		if owners, ok := s.in.Owners[pkg.Path]; ok && explicit[sub] == "" {
			if perSub[sub] == nil {
				perSub[sub] = map[string]bool{}
			}
			perSub[sub][strings.Join(owners, " ")] = true
			out[sub] = append(out[sub], owners...)
		}
	}
	for sub, owner := range explicit {
		if owner != "" {
			out[sub] = []string{owner}
		}
	}
	for sub, sets := range perSub {
		slices.Sort(out[sub])
		out[sub] = slices.Compact(out[sub])
		if len(sets) > 1 {
			fs = append(fs, s.ownersDiffer(sub, slices.Sorted(maps.Keys(sets))))
		}
	}
	return out, fs
}

// ownersDiffer is the finding that one subsystem's packages have different
// owners.
func (s *subject) ownersDiffer(sub string, sets []string) arch.Finding {
	f := arch.NewFinding(
		arch.FindingID{Rule: ruleOwnersDiffer, Subject: sub, Object: ""},
		s.severity(),
	)
	f.Message = fmt.Sprintf(
		"the packages of subsystem %s have %d different sets of owners in CODEOWNERS",
		sub,
		len(sets),
	)
	for _, set := range sets {
		f.Evidence = append(f.Evidence, "owners: "+set)
	}
	f.Fix = "give the subsystem one owner in the model, or align CODEOWNERS with the subsystem"
	return f
}

// deployUnits finds the binaries in scope and the packages each contains:
// everything a main package reaches through in-scope imports.
func (s *subject) deployUnits() deployUnits {
	d := deployUnits{in: map[string][]string{}, unbound: make([]string, 0)}
	imports := map[string][]string{}
	for _, e := range s.g.Packages.Edges() {
		imports[e.From] = append(imports[e.From], e.To)
	}
	for _, pkg := range s.p.Packages {
		if pkg.Name != "main" {
			continue
		}
		d.binaries = true
		for _, contained := range append(reachable(pkg.Path, imports), pkg.Path) {
			d.in[contained] = append(d.in[contained], pkg.Path)
		}
	}
	for _, pkg := range s.p.Packages {
		if d.binaries && len(d.in[pkg.Path]) == 0 {
			d.unbound = append(d.unbound, pkg.Path)
		}
	}
	return d
}

// reachable returns the nodes reachable from start along next, without start
// itself, sorted.
func reachable(start string, next map[string][]string) []string {
	seen := map[string]bool{start: true}
	var out []string
	for queue := []string{start}; len(queue) > 0; queue = queue[1:] {
		for _, n := range next[queue[0]] {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
				queue = append(queue, n)
			}
		}
	}
	slices.Sort(out)
	return out
}

// separate reports two packages that binaries contain, but never together.
func (d *deployUnits) separate(a, b string) bool {
	ia, ib := d.in[a], d.in[b]
	if len(ia) == 0 || len(ib) == 0 {
		return false
	}
	for _, x := range ia {
		if slices.Contains(ib, x) {
			return false
		}
	}
	return true
}

// asyncPackages returns the packages that import a messaging client the
// model names and that a reviewer confirmed integrate asynchronously. Each
// such package gets a slot; only an approved "async" counts (§9.3).
func (s *subject) asyncPackages() map[string]bool {
	out := map[string]bool{}
	if len(s.m.Balance.Messaging) == 0 {
		return out
	}
	clients := map[string][]string{}
	for _, imp := range s.p.Imports {
		if arch.MatchesAny(s.m.Balance.Messaging, imp.To) {
			clients[imp.From] = append(clients[imp.From], imp.To)
		}
	}
	for pkg, via := range clients {
		ev := []string{fmt.Sprintf("%s imports %s", pkg, strings.Join(via, ", "))}
		if s.slots.ask("async/"+pkg, ev) == "async" {
			out[pkg] = true
		}
	}
	return out
}
