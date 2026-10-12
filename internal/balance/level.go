package balance

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Integration levels, from the least knowledge shared to the most
// (connascence.md §9.2). LevelNone marks evidence that grades a level, such
// as a static connascence instance, without setting one.
const (
	LevelNone Level = iota
	LevelContract
	LevelModel
	LevelFunctional
	LevelIntrusive
)

// Level is an integration strength: contract, model, functional or
// intrusive coupling.
type Level int

// evidence is one reference or instance that couples two subsystems.
type evidence struct {
	from, to       string // downstream and upstream subsystems
	fromPkg, toPkg string
	level          Level
	basis          arch.Basis
	// kind is the connascence kind the evidence carries, a degree within the
	// level; 0 when it carries none.
	kind arch.Strength
	// symmetric marks a coupling without a direction, such as a clone.
	symmetric bool
	// nonImport marks a coupling that needs no import, which alone can cross
	// a deploy boundary (connascence.md §9.3).
	nonImport bool
	site      arch.Site
	note      string
}

// classifier finds the integration level of every coupling between two
// subsystems (connascence.md §9.2): what knowledge a reference or a
// connascence instance shares.
type classifier struct {
	p      *arch.Program
	m      *arch.Model
	g      *arch.Graphs
	c      arch.Containers
	roles  map[string]string
	sigs   map[string]*arch.Signature
	consts map[string]*arch.Constant
	types  map[string]*arch.Type
	slots  *slotBook
	// connascence is every instance the connascence measure found; those
	// whose fingerprint is in rejected are left out.
	connascence []arch.Connascence
	rejected    []string
}

// levelNames are the levels' names, indexed by level.
func levelNames() []string {
	return []string{"none", "contract", "model", "functional", "intrusive"}
}

// String returns the level's name.
func (l Level) String() string {
	if names := levelNames(); l >= 0 && int(l) < len(names) {
		return names[l]
	}
	return fmt.Sprintf("level(%d)", int(l))
}

// MarshalText writes the level's name.
func (l Level) MarshalText() ([]byte, error) { return []byte(l.String()), nil }

// evidence collects every cross-subsystem coupling of the program: the
// references it makes and the connascence it has.
func (k *classifier) evidence() []evidence {
	var ev []evidence
	ev = append(ev, k.bypassEvidence()...)
	ev = append(ev, k.intrusiveEvidence()...)
	ev = append(ev, k.useEvidence()...)
	ev = append(ev, k.connascenceEvidence()...)
	return ev
}

// bypassEvidence is the imports of packages a subsystem does not list in its
// interface: implementation details used for integration, intrusive
// coupling. A subsystem that declares no interface gives no evidence here,
// since what is internal to it is unknown; the MMI reports it.
func (k *classifier) bypassEvidence() []evidence {
	iface := map[string][]string{}
	for _, sub := range k.m.Subsystems {
		iface[sub.Name] = sub.Interface
	}
	var ev []evidence
	for _, imp := range k.p.Imports {
		if _, inScope := k.c.Module(imp.To); !inScope {
			continue
		}
		from, to := k.subsystemOf(imp.From), k.subsystemOf(imp.To)
		if from == to || len(iface[to]) == 0 || arch.MatchesAny(iface[to], imp.To) {
			continue
		}
		ev = append(ev, evidence{
			from: from, to: to, fromPkg: imp.From, toPkg: imp.To,
			level: LevelIntrusive, basis: arch.BasisFact,
			site: arch.Site{File: imp.Pos.File, Line: imp.Pos.Line, Package: imp.From},
			note: fmt.Sprintf("%s imports %s, which %s does not list as its interface",
				imp.From, imp.To, to),
		})
	}
	return ev
}

// intrusiveEvidence is the linknames into, and unsafe conversions of, another
// subsystem's code.
func (k *classifier) intrusiveEvidence() []evidence {
	var ev []evidence
	add := func(site arch.Site, target, note string) {
		if _, inScope := k.c.Module(target); !inScope {
			return
		}
		from, to := k.subsystemOf(site.Package), k.subsystemOf(target)
		if from == to {
			return
		}
		ev = append(ev, evidence{
			from: from, to: to, fromPkg: site.Package, toPkg: target,
			level: LevelIntrusive, basis: arch.BasisFact, nonImport: true, site: site, note: note,
		})
	}
	for _, l := range k.p.Facts.Linknames {
		add(l.Site, l.Package, fmt.Sprintf("%s links %s to %s, bypassing its interface",
			l.Site.Package, l.Local, l.Target))
	}
	for _, u := range k.p.Facts.UnsafeConvs {
		add(
			u.Site,
			u.Package,
			fmt.Sprintf("%s converts %s through unsafe.Pointer", u.Site.Func, u.Type),
		)
	}
	return ev
}

// useEvidence classifies each reference to another subsystem's object as
// model or contract coupling: model when the object exposes the upstream's
// own types, contract when it exposes only basic types, types from outside
// the analysis, or types whose role is contract (connascence.md §9.2).
func (k *classifier) useEvidence() []evidence {
	var ev []evidence
	for i := range k.p.Facts.Uses {
		u := &k.p.Facts.Uses[i]
		from, to := k.subsystemOf(u.Site.Package), k.subsystemOf(u.Package)
		if from == to {
			continue
		}
		level, why := k.useLevel(u, to)
		ev = append(ev, evidence{
			from: from, to: to, fromPkg: u.Site.Package, toPkg: u.Package,
			level: level, basis: arch.BasisFact, site: u.Site,
			note: fmt.Sprintf("%s uses %s %s: %s", siteName(u.Site), u.Kind, u.Object, why),
		})
	}
	return ev
}

// useLevel says whether a use is model or contract coupling, and why.
func (k *classifier) useLevel(u *arch.Use, upstream string) (Level, string) {
	var exposed []string
	switch u.Kind {
	case arch.ObjectType:
		exposed = []string{u.Object}
	case arch.ObjectField, arch.ObjectVar:
		return LevelModel, "it reaches into the upstream's state"
	case arch.ObjectConst:
		if k, ok := k.consts[u.Object]; ok {
			exposed = []string{k.Type}
		}
	default:
		exposed = k.signatureTypes(u.Object)
	}
	for _, t := range exposed {
		if k.isModelType(t, upstream) {
			return LevelModel, "it exposes the model type " + t
		}
	}
	return LevelContract, "it exposes no model type of " + upstream
}

// signatureTypes returns the named types a function's or method's signature
// mentions.
func (k *classifier) signatureTypes(fn string) []string {
	sig, ok := k.sigs[fn]
	if !ok {
		return nil
	}
	var out []string
	for _, r := range slices.Concat(sig.Params, sig.Results) {
		if r.Named != "" {
			out = append(out, r.Named)
		}
	}
	return out
}

// isModelType reports a type of the upstream subsystem that is part of its
// model: not a contract by role or by verdict, and not an interface whose
// methods expose only contract types. A type it cannot see is no model type.
func (k *classifier) isModelType(typ, upstream string) bool {
	t, ok := k.types[typ]
	if !ok || k.subsystemOf(t.Package) != upstream || k.isContract(typ) {
		return false
	}
	if t.Kind != arch.KindInterface {
		return true
	}
	prefix := typ + "."
	for id := range k.sigs {
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		for _, n := range k.signatureTypes(id) {
			if n != typ && k.isModelType(n, upstream) {
				return true
			}
		}
	}
	return false
}

// isContract reports a type whose role is contract, or that an approved
// verdict calls one.
func (k *classifier) isContract(typ string) bool {
	if k.roles[typ] == "contract" {
		return true
	}
	return k.slots.ask("contract-type/"+typ, k.typeEvidence(typ)) == "contract"
}

// typeEvidence is what a reviewer weighs to tell a model type from a
// contract: the type, its kind and fields, and its role.
func (k *classifier) typeEvidence(typ string) []string {
	t := k.types[typ]
	ev := []string{fmt.Sprintf("type %s (%s) in package %s", typ, t.Kind, t.Package)}
	if len(t.Fields) > 0 {
		ev = append(ev, "fields: "+strings.Join(t.Fields, ", "))
	}
	if role := k.roles[typ]; role != "" {
		ev = append(ev, "role: "+role)
	}
	return ev
}

// connascenceEvidence turns each connascence instance spanning subsystems
// into evidence for every pair it spans. Dynamic kinds, algorithms and clones
// are duplicated or run-time knowledge: functional coupling. Name, type,
// meaning and position grade a level the references set (§9.2).
func (k *classifier) connascenceEvidence() []evidence {
	rejected := map[string]bool{}
	for _, fp := range k.rejected {
		rejected[fp] = true
	}
	var ev []evidence
	for i := range k.connascence {
		c := &k.connascence[i]
		if rejected[c.Fingerprint] || c.Kind == arch.StrengthContranascence {
			continue
		}
		ev = append(ev, k.instanceEvidence(c)...)
	}
	return ev
}

// instanceEvidence is the evidence one instance gives the pairs of the
// subsystems its sites span. An instance about one object, such as a shared
// variable or a constant, couples every other subsystem to the object's own:
// knowledge flows from its owner. An instance about no object, such as a
// clone or a repeated pattern, couples every pair it spans, the way an
// import points when one exists and without a direction otherwise.
func (k *classifier) instanceEvidence(c *arch.Connascence) []evidence {
	bySub := map[string]arch.Site{}
	for _, site := range c.Sites {
		sub := k.subsystemOf(site.Package)
		if _, seen := bySub[sub]; !seen {
			bySub[sub] = site
		}
	}
	level := LevelNone
	if c.Kind.Dynamic() || c.Kind == arch.StrengthAlgorithm {
		level = LevelFunctional
	}
	item := func(from, to string, symmetric bool) evidence {
		return evidence{
			from: from, to: to, fromPkg: bySub[from].Package, toPkg: bySub[to].Package,
			level: level, basis: c.Basis, kind: c.Kind, symmetric: symmetric,
			nonImport: true, site: bySub[from],
			note: fmt.Sprintf("%s %s: %s", c.Detector, c.Basis, c.Message),
		}
	}
	subs := slices.Sorted(maps.Keys(bySub))
	pkg := k.ownerPackage(c.Object)
	if pkg == "" {
		return k.pairwise(subs, item)
	}
	owner := k.subsystemOf(pkg)
	if _, ok := bySub[owner]; !ok {
		bySub[owner] = arch.Site{Package: pkg}
	}
	var ev []evidence
	for _, sub := range subs {
		if sub != owner {
			ev = append(ev, item(sub, owner, false))
		}
	}
	return ev
}

// pairwise is the evidence of an instance about no object: one item for
// every pair of the subsystems it spans.
func (k *classifier) pairwise(
	subs []string,
	item func(from, to string, symmetric bool) evidence,
) []evidence {
	var ev []evidence
	for i, a := range subs {
		for _, b := range subs[i+1:] {
			ev = append(ev, item(k.direction(a, b)))
		}
	}
	return ev
}

// ownerPackage returns the in-scope package that declares an object, the
// longest package path its ID begins with; "" for no object, or one declared
// outside the analysis.
func (k *classifier) ownerPackage(object string) string {
	best := ""
	for _, pkg := range k.p.Packages {
		if object != "" && strings.HasPrefix(object, pkg.Path+".") && len(pkg.Path) > len(best) {
			best = pkg.Path
		}
	}
	return best
}

// direction orders two subsystems as the subsystem graph does: from the one
// that imports to the one imported. Without an import either way, the order
// is by name and the coupling is symmetric.
func (k *classifier) direction(a, b string) (from, to string, symmetric bool) {
	switch {
	case k.g.Subsystems.Weight(a, b) > 0:
		return a, b, false
	case k.g.Subsystems.Weight(b, a) > 0:
		return b, a, false
	default:
		return a, b, true
	}
}

// siteName is the function a site lies in, or its file and line.
func siteName(s arch.Site) string {
	if s.Func != "" {
		return s.Func
	}
	return fmt.Sprintf("%s:%d", s.File, s.Line)
}

// subsystemOf returns a package's subsystem, or the package itself.
func (k *classifier) subsystemOf(pkg string) string {
	return k.c.SubsystemOf(arch.Site{Package: pkg})
}
