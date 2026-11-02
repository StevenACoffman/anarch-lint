package fitness

import (
	"fmt"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// maxCells bounds the cells the verification checks; a larger model is
// reported as not verified rather than checked in part (feelc's gridBudget).
const maxCells = 250_000

// Verification says whether the import rules were checked against each other,
// over how many classes of packages and cells, and why not when they were not.
type Verification struct {
	Verified bool   `json:"verified"`
	Classes  int    `json:"classes"`
	Cells    int    `json:"cells"`
	Reason   string `json:"reason,omitempty"`
}

// forbidder is one import rule as the verification sees it: a fitness
// constraint, or one of the model's forbidden imports.
type forbidder struct {
	// name is how findings name the rule; modelRule is where it is in the
	// model, as a finding's ModelRule gives it.
	name, modelRule string
	anyTarget       bool
	reads           func(pkg string) string
	violates        func(from, to string) bool
	allowances      []allowance
}

// class is a set of packages every rule reads alike, so one representative
// stands for all: the rules cannot tell its members apart.
type class struct {
	rep     string
	size    int
	inScope bool
}

// cell is an import from a package of one class to a package of another, or
// of the same class when it has two members.
type cell struct {
	from, to *class
}

// verifier holds one verification: the rules, the cells, and which rule
// forbids which cell.
type verifier struct {
	fs    []forbidder
	cells []cell
	// forbids[i][j] reports whether rule i forbids cell j.
	forbids [][]bool
}

// verify checks the import rules against each other over the packages there
// are (feelc's verifier, over classes instead of declared inputs): a rule
// whose every forbidden import another rule forbids too is redundant, and a
// name a rule allows imports by is unusable when other rules forbid every
// import it names. Both are warnings: the model still means what it says, it
// only says it twice or in vain. Isolations are left out, since what they
// allow depends on each package's own captures, which no class shares.
//
// Requires: inside and outside are sorted and disjoint.
func verify(
	cs []constraint, forbidden []arch.ForbiddenImport, inside, outside []string,
) (Verification, []arch.Finding) {
	fs := forbidders(cs, forbidden)
	in, all := classes(fs, inside, outside)
	v := Verification{Classes: len(all), Verified: true}
	if len(fs) == 0 {
		return v, nil
	}
	if n := len(in) * len(all); n > maxCells {
		v.Verified, v.Cells = false, n
		v.Reason = fmt.Sprintf("%d cells exceed the limit of %d", n, maxCells)
		return v, nil
	}
	vr := newVerifier(fs, cellsOf(in, all))
	v.Cells = len(vr.cells)
	return v, slices.Concat(vr.redundant(), vr.unusable())
}

// forbidders are the rules the verification checks: each fitness constraint
// that is not off, then each forbidden import, in declaration order.
func forbidders(cs []constraint, forbidden []arch.ForbiddenImport) []forbidder {
	var out []forbidder
	for i := range cs {
		c := &cs[i]
		if c.rule.off {
			continue
		}
		reads := c.reads
		if reads == nil {
			reads = eachAlone
		}
		out = append(out, forbidder{
			name: c.rule.result.Name, modelRule: c.rule.modelRule(), anyTarget: c.anyTarget,
			reads:      reads,
			violates:   func(from, to string) bool { return c.violates(from, to) != "" },
			allowances: c.allowances,
		})
	}
	for i := range forbidden {
		fi := &forbidden[i]
		out = append(out, forbidder{
			name: fmt.Sprintf("forbiddenImports[%d]", i), anyTarget: true,
			modelRule: fmt.Sprintf("forbiddenImports[%d]", i),
			reads: func(pkg string) string {
				return string([]byte{
					bit(arch.MatchesAny(fi.From, pkg)), bit(arch.MatchesAny(fi.Imports, pkg)),
				})
			},
			violates: func(from, to string) bool {
				return arch.MatchesAny(fi.From, from) && arch.MatchesAny(fi.Imports, to)
			},
		})
	}
	return out
}

// classes groups the packages inside the analysis, and then those outside it
// that something imports, by what every rule reads of them: in holds the
// inside classes, and all holds them followed by the outside ones, each in
// the order of its first, smallest, member.
//
// Ensures: no two classes of one kind are read alike by every rule.
func classes(fs []forbidder, inside, outside []string) (in, all []*class) {
	group := func(pkgs []string, inScope bool, into []*class) []*class {
		byKey := map[string]*class{}
		for _, pkg := range pkgs {
			parts := make([]string, len(fs))
			for i := range fs {
				parts[i] = fs[i].reads(pkg)
			}
			key := strings.Join(parts, "\x00")
			if c, ok := byKey[key]; ok {
				c.size++
				continue
			}
			c := &class{rep: pkg, size: 1, inScope: inScope}
			byKey[key] = c
			into = append(into, c)
		}
		return into
	}
	in = group(inside, true, nil)
	all = group(outside, false, slices.Clone(in))
	return in, all
}

// eachAlone is what a rule that does not say what it reads is taken to read:
// the package itself, which puts every package in a class of its own. That is
// exact, only slower.
func eachAlone(pkg string) string { return pkg }

// cellsOf returns every import from a class of from to a class of to. A class
// meets itself only when it has two members, since a package does not import
// itself; no rule tells two members of one class apart, so the representative
// stands for both ends.
func cellsOf(from, to []*class) []cell {
	var out []cell
	for _, f := range from {
		for _, t := range to {
			if f != t || f.size > 1 {
				out = append(out, cell{from: f, to: t})
			}
		}
	}
	return out
}

// newVerifier evaluates every rule on every cell. A rule that checks only
// imports under analysis forbids no cell outside it.
func newVerifier(fs []forbidder, cells []cell) *verifier {
	v := &verifier{fs: fs, cells: cells, forbids: make([][]bool, len(fs))}
	for i := range fs {
		v.forbids[i] = make([]bool, len(cells))
		for j, c := range cells {
			v.forbids[i][j] = (c.to.inScope || fs[i].anyTarget) &&
				fs[i].violates(c.from.rep, c.to.rep)
		}
	}
	return v
}

// redundant reports each rule whose every forbidden cell another rule forbids
// too. Of two rules that forbid the same cells, the later is the redundant
// one.
func (v *verifier) redundant() []arch.Finding {
	var out []arch.Finding
	for a := range v.fs {
		first := slices.Index(v.forbids[a], true)
		if first < 0 {
			continue
		}
		b := v.coverer(a)
		if b < 0 {
			continue
		}
		c := v.cells[first]
		f := arch.NewFinding(
			arch.FindingID{Rule: arch.RuleRedundant, Subject: v.fs[a].name, Object: v.fs[b].name},
			arch.SeverityWarning,
		)
		f.ModelRule = v.fs[a].modelRule
		f.Message = fmt.Sprintf(
			"every import %s forbids, %s forbids too",
			v.fs[a].name,
			v.fs[b].name,
		)
		f.Evidence = []string{fmt.Sprintf("for example %s → %s", c.from.rep, c.to.rep)}
		f.Fix = fmt.Sprintf("remove %s, or keep it and narrow %s if %s states the intent",
			v.fs[a].name, v.fs[b].name, v.fs[a].name)
		out = append(out, f)
	}
	return out
}

// coverer returns the first rule other than rule a that forbids every cell a
// forbids, or -1. Of two rules that forbid the same cells, only the earlier
// covers the later.
func (v *verifier) coverer(a int) int {
	for b := range v.fs {
		if b != a && subset(v.forbids[a], v.forbids[b]) &&
			(b < a || !slices.Equal(v.forbids[a], v.forbids[b])) {
			return b
		}
	}
	return -1
}

// unusable reports each name a rule allows imports by when other rules forbid
// every import it names: the allowance grants nothing.
func (v *verifier) unusable() []arch.Finding {
	var out []arch.Finding
	for a := range v.fs {
		for _, al := range v.fs[a].allowances {
			blocker, ok := v.blocker(a, al)
			if !ok {
				continue
			}
			f := arch.NewFinding(
				arch.FindingID{
					Rule:    arch.RuleUnusableAllowance,
					Subject: v.fs[a].name,
					Object:  al.item,
				},
				arch.SeverityWarning,
			)
			f.ModelRule = v.fs[a].modelRule
			f.Message = fmt.Sprintf("%s allows %s, but other rules forbid every import it allows",
				v.fs[a].name, al.item)
			f.Evidence = []string{"for example " + v.fs[blocker].name + " forbids it"}
			f.Fix = fmt.Sprintf("remove %s from %s, or narrow %s", al.item, v.fs[a].name,
				v.fs[blocker].name)
			out = append(out, f)
		}
	}
	return out
}

// blocker returns a rule other than rule a that forbids the first cell al
// names, and true when other rules forbid every cell it names, of which there
// is at least one.
func (v *verifier) blocker(a int, al allowance) (int, bool) {
	blocker := -1
	for j, c := range v.cells {
		if !al.names(c.from.rep, c.to.rep) {
			continue
		}
		by := v.forbiddenBy(j, a)
		if by < 0 {
			return -1, false
		}
		if blocker < 0 {
			blocker = by
		}
	}
	return blocker, blocker >= 0
}

// forbiddenBy returns the first rule other than except that forbids cell j,
// or -1.
func (v *verifier) forbiddenBy(j, except int) int {
	for i := range v.forbids {
		if i != except && v.forbids[i][j] {
			return i
		}
	}
	return -1
}

// subset reports whether every cell a forbids, b forbids too.
func subset(a, b []bool) bool {
	for i := range a {
		if a[i] && !b[i] {
			return false
		}
	}
	return true
}
