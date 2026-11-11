package fitness

import (
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// export is one exported package-level identifier and where it is
// declared.
type export struct {
	id, pkg string
	at      arch.Span
}

// checkUnusedExports reports, for each unusedExports rule, every exported
// type, function and constant of its packages that no other package and no
// external test uses, directly or as part of the API it does use.
func checkUnusedExports(l *ledger, rules []arch.UnusedExports, p *arch.Program) {
	used := usedAPI(p)
	exports := exportsOf(p)
	for i := range rules {
		ue := &rules[i]
		r := l.addRule("unusedExports", ue.RuleOptions,
			arch.DefaultRuleName(ue.Name, "unusedExports", i))
		// A use that makes an export used can come from any package.
		r.readsCode = everyPackage
		for _, e := range exports {
			if !arch.MatchesAny(ue.Packages, e.pkg) {
				continue
			}
			r.result.Matched++
			if used[e.id] {
				continue
			}
			f := arch.NewFinding(arch.FindingID{
				Rule: arch.RuleUnusedExport, Subject: e.id, Object: r.result.Name,
			}, arch.SeverityError)
			f.Location = &e.at
			f.Message = fmt.Sprintf(
				"%s is exported, but no other package or external test uses it",
				e.id,
			)
			f.Fix = "unexport it, or delete it if nothing in its own package uses it either"
			l.violation(r, &f)
		}
	}
}

// exportsOf lists the exported package-level types, functions and constants
// of every package but main, in the program's order.
func exportsOf(p *arch.Program) []export {
	mains := map[string]bool{}
	for _, pkg := range p.Packages {
		mains[pkg.Path] = pkg.Name == "main"
	}
	var out []export
	add := func(id, pkg, name string, at arch.Span) {
		if isExported(name) && !mains[pkg] {
			out = append(out, export{id, pkg, at})
		}
	}
	for i := range p.Types {
		t := &p.Types[i]
		add(t.ID, t.Package, t.Name, t.Span)
	}
	for i := range p.Funcs {
		if fn := &p.Funcs[i]; fn.Receiver == "" {
			add(fn.ID, fn.Package, fn.Name, fn.Span)
		}
	}
	for i := range p.Facts.Constants {
		c := &p.Facts.Constants[i]
		name := c.ID[len(c.Site.Package)+1:]
		add(
			c.ID,
			c.Site.Package,
			name,
			arch.Span{File: c.Site.File, Start: c.Site.Line, End: c.Site.Line},
		)
	}
	return out
}

// usedAPI returns the identifiers another package or an external test uses,
// and what they need exported: using a method or field uses its type; a
// used type's fields, embeddings and method signatures, and a used
// function's parameters and results, are used with it; and a constant of a
// used type, a member of its enum, is part of its API.
func usedAPI(p *arch.Program) map[string]bool {
	used := map[string]bool{}
	for i := range p.Facts.Uses {
		if u := &p.Facts.Uses[i]; u.Site.Package != u.Package {
			used[u.Object], used[u.Owner] = true, true
		}
	}
	for _, id := range p.Facts.TestUses {
		used[id] = true
	}
	for spreadAPI(p, used) { // until the used API exposes nothing new
	}
	for i := range p.Facts.Constants {
		if c := &p.Facts.Constants[i]; used[c.Type] {
			used[c.ID] = true
		}
	}
	return used
}

// spreadAPI marks what the used identifiers expose, and reports whether it
// marked anything new: the types a used type's fields, embeddings and
// method signatures name, and those a used function takes or returns.
func spreadAPI(p *arch.Program, used map[string]bool) bool {
	changed := false
	mark := func(id string) {
		if id != "" && !used[id] {
			used[id], changed = true, true
		}
	}
	for i := range p.Refs {
		if r := &p.Refs[i]; used[r.From] && partOfAPI(r.Kinds) {
			mark(r.To)
		}
	}
	for i := range p.Facts.Signatures {
		sig := &p.Facts.Signatures[i]
		if !used[sig.Func] {
			continue
		}
		for _, t := range slices.Concat(sig.Params, sig.Results) {
			mark(t.Named)
		}
	}
	return changed
}

// partOfAPI reports a reference that a type's users see: an embedding, a
// field's type or a method's signature.
func partOfAPI(kinds []arch.RefKind) bool {
	return slices.ContainsFunc(kinds, func(k arch.RefKind) bool {
		return k == arch.RefEmbed || k == arch.RefField || k == arch.RefSignature
	})
}
