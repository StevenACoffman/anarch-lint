package fitness

import (
	"fmt"
	"go/token"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// invariantCheck compares one domain model's invariants with the code
// (docs/adr/0016). It embeds the model's domain check for what that has
// resolved, and holds what only the invariant checks read.
type invariantCheck struct {
	*domainCheck
	// ran is the functions a test ran, by ID; nil without a coverage
	// profile.
	ran map[string]bool
	// writes maps each type to the writes of its fields from other
	// packages, as outsideWrites lists them.
	writes map[string][]string
}

// governed returns the entities of model m whose state the invariant
// governs: the entity that declares it and each entity its statement names
// in backticks, as modelith asks authors to write them (docs/adr/0016). A
// backticked glossary term, or an entity of an imported model written as
// scope.Name, is not an entity of m and is left out.
//
// Ensures: the names are entities of m, sorted, with no duplicates.
func governed(inv *arch.DomainInvariant, m *arch.DomainModel) []string {
	names := []string{inv.Entity}
	// Splitting on backticks puts every quoted name at an odd index.
	for i, part := range strings.Split(inv.Statement, "`") {
		if i%2 == 1 {
			names = append(names, part)
		}
	}
	out := slices.DeleteFunc(names, func(name string) bool {
		return !slices.ContainsFunc(m.Entities, func(e arch.DomainEntity) bool {
			return e.Name == name
		})
	})
	slices.Sort(out)
	return slices.Compact(out)
}

// check checks each invariant of the model against the code: that
// something enforces it, that a test runs its enforcers when a coverage
// profile says, and that no governed entity's state is open to code
// outside its type's package.
//
// Requires: matched maps each entity matched to exactly one type.
func (c *invariantCheck) check(matched map[string]string) {
	guards := c.guardsByType(c.handlesByFunc())
	for i := range c.model.Invariants {
		inv := &c.model.Invariants[i]
		enforcers := slices.DeleteFunc(
			slices.Clone(c.program.Enforcers),
			func(e arch.Enforcer) bool {
				return e.Invariant != inv.ID || !c.inRule(e.Package)
			},
		)
		switch {
		case len(enforcers) == 0:
			c.unenforced(inv, guardsOf(governed(inv, c.model), matched, guards))
		case c.ran != nil:
			c.untested(inv, enforcers)
		}
		for _, name := range governed(inv, c.model) {
			// An entity matched to no type or several is
			// domain-entity-missing or domain-entity-ambiguous already.
			if id, ok := matched[name]; ok {
				c.exposed(inv, name, id)
			}
		}
	}
}

// unenforced reports an invariant that no "//arch:invariant" directive in
// the rule's packages names: nobody has said where the code keeps it. The
// error guards of the functions that handle its governed types are its
// evidence, places it may be kept that only a person can confirm, so they
// never enforce it themselves (docs/adr/0016).
func (c *invariantCheck) unenforced(inv *arch.DomainInvariant, guards []string) {
	fix := fmt.Sprintf("mark the function or test that keeps it with //arch:invariant %s", inv.ID)
	if len(guards) > 0 {
		fix += ", which may be one of the guards listed"
	}
	evidence := slices.Clip(guards[:min(len(guards), maxEdgeEvidence)])
	c.report(
		arch.FindingID{Rule: arch.RuleInvariantUnenforced, Subject: c.subjectOf(inv.ID)},
		fmt.Sprintf("invariant %s has no enforcer: no function or test of %s says it keeps %q",
			inv.ID, c.r.result.Name, inv.Statement),
		fix,
		appendMore(evidence, len(guards), "error guards", inv.ID)...,
	)
}

// handlesByFunc maps each function to the named types of its parameters
// and results.
func (c *invariantCheck) handlesByFunc() map[string][]string {
	handles := map[string][]string{}
	for i := range c.program.Facts.Signatures {
		sig := &c.program.Facts.Signatures[i]
		for _, t := range slices.Concat(sig.Params, sig.Results) {
			if t.Named != "" {
				handles[sig.Func] = append(handles[sig.Func], t.Named)
			}
		}
	}
	return handles
}

// handledBy returns the types a guard's function handles: its receiver's,
// and those handles gives it, sorted, with no duplicates and no "".
func handledBy(site *arch.Site, handles map[string][]string) []string {
	types := slices.Sorted(slices.Values(append([]string{site.Type}, handles[site.Func]...)))
	return slices.DeleteFunc(slices.Compact(types), func(t string) bool { return t == "" })
}

// guardLine names an error guard in evidence.
func guardLine(site *arch.Site) string {
	return fmt.Sprintf("%s guards at %s:%d", site.Func, site.File, site.Line)
}

// guardsByType maps each type to the error guards of the rule's functions
// that handle it, one line per guard: its methods, and the functions with a
// parameter or result of the type, constructors among them.
func (c *invariantCheck) guardsByType(handles map[string][]string) map[string][]string {
	out := map[string][]string{}
	for i := range c.program.Facts.ErrorGuards {
		site := &c.program.Facts.ErrorGuards[i].Site
		if !c.inRule(site.Package) {
			continue
		}
		for _, t := range handledBy(site, handles) {
			out[t] = append(out[t], guardLine(site))
		}
	}
	return out
}

// guardsOf returns the guards of the types the governed entities matched,
// sorted, with no duplicates: a function that handles two of them is listed
// once.
func guardsOf(entities []string, matched map[string]string, guards map[string][]string) []string {
	var out []string
	for _, name := range entities {
		if id, ok := matched[name]; ok {
			out = append(out, guards[id]...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// untested reports an invariant whose enforcers are all functions, none of
// which a test runs: the code says where it keeps the invariant, and no test
// shows it does. A test enforcer is run by go test, so one is enough.
//
// Requires: enforcers is not empty, and c.ran is not nil.
func (c *invariantCheck) untested(inv *arch.DomainInvariant, enforcers []arch.Enforcer) {
	funcs := make([]string, 0, len(enforcers))
	for _, e := range enforcers {
		if e.Test || c.ran[e.Func] {
			return
		}
		funcs = append(funcs, e.Func)
	}
	c.report(
		arch.FindingID{Rule: arch.RuleInvariantUntested, Subject: c.subjectOf(inv.ID)},
		fmt.Sprintf("invariant %s is enforced by %s, which no test runs",
			inv.ID, strings.Join(funcs, ", ")),
		"write a test that runs the enforcer, and mark it with //arch:invariant "+inv.ID,
		funcs...,
	)
}

// exposed reports a governed entity's type with an exported field: code
// outside the type's package can then change the state the invariant
// constrains without going through the actions that preserve it. Only an
// exported field can be written from another package, so the writes found
// there are the evidence and not a second condition. The type's own package
// may write its fields, so its constructors and decoders are no exposure.
func (c *invariantCheck) exposed(inv *arch.DomainInvariant, entity, id string) {
	writes := c.writes[id]
	exported := slices.DeleteFunc(slices.Clone(c.types[id].Fields), func(f string) bool {
		return !token.IsExported(f)
	})
	if len(exported) == 0 {
		return
	}
	evidence := []string{"exported fields: " + strings.Join(exported, ", ")}
	evidence = append(evidence, writes[:min(len(writes), maxEdgeEvidence)]...)
	evidence = appendMore(evidence, len(writes), "writes", id)
	c.report(
		arch.FindingID{Rule: arch.RuleInvariantExposed, Subject: c.subjectOf(inv.ID), Object: id},
		fmt.Sprintf("invariant %s governs entity %s, whose type %s has exported fields "+
			"that code outside %s can change", inv.ID, entity, id, c.types[id].Package),
		"unexport the fields, and change them only through methods that keep the invariant",
		evidence...,
	)
}

// outsideWrites maps each type to the writes of its fields from another
// package, one line per write naming the field and where it is written, in
// the order of uses.
func outsideWrites(uses []arch.Use) map[string][]string {
	out := map[string][]string{}
	for i := range uses {
		u := &uses[i]
		if u.Kind != arch.ObjectField || u.Writes == 0 || u.Site.Package == u.Package {
			continue
		}
		out[u.Owner] = append(out[u.Owner], fmt.Sprintf("%s written at %s:%d",
			u.Object, u.Site.File, u.Site.Line))
	}
	return out
}

// ranFuncs returns the functions with a statement a test ran, by ID; nil
// without a profile, so a caller can tell no profile from no tests. A
// function the profile does not instrument did not run.
func ranFuncs(cov *arch.Coverage) map[string]bool {
	if cov == nil {
		return nil
	}
	ran := map[string]bool{}
	for _, f := range cov.Funcs {
		if f.Covered > 0 {
			ran[f.Func] = true
		}
	}
	return ran
}
