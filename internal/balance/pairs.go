package balance

import (
	"cmp"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Quadrants of the binary Balanced Coupling model (connascence.md §9.5).
const (
	QuadrantLooseCoupling Quadrant = "loose-coupling"
	QuadrantHighCohesion  Quadrant = "high-cohesion"
	QuadrantLowCohesion   Quadrant = "low-cohesion"
	QuadrantTightCoupling Quadrant = "tight-coupling"
)

// Distance ordinals, after archfit's (connascence.md §9.5): a package
// boundary within one subsystem, a subsystem boundary within one team and
// one binary, a module boundary, an owner
// boundary or asynchronous integration, and a deploy boundary. A distance of
// highDistance or more is high in the binary model.
const (
	distancePackage   = 2
	distanceSubsystem = 4
	distanceModule    = 7
	distanceOwner     = 7
	distanceAsync     = 7
	distanceDeploy    = 9
	highDistance      = 7
)

// Quadrant names one of the four combinations of strength and distance.
type Quadrant string

// Pair is the Balanced Coupling of one ordered pair of subsystems: how much
// knowledge flows from the upstream To to the downstream From, how far apart
// they are, and how likely To is to change.
type Pair struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Level is the strongest integration level the evidence shows, and
	// Basis says whether a fact or only a candidate shows it.
	Level Level      `json:"level"`
	Basis arch.Basis `json:"basis"`
	// Kind is the strongest connascence at that level, its degree within
	// the level; "" for intrusive coupling, which has none.
	Kind string `json:"kind,omitempty"`
	// Symmetric marks a pair coupled only without a direction, such as by
	// clones, ordered by name.
	Symmetric bool `json:"symmetric"`
	// Distance: the locality of the pair's sites, the owner and deploy
	// boundaries, asynchronous integration, and the resulting ordinal.
	Locality       arch.Locality `json:"locality"`
	OwnerBoundary  bool          `json:"ownerBoundary"`
	DeployBoundary bool          `json:"deployBoundary"`
	Async          bool          `json:"async"`
	Distance       int           `json:"distance"`
	// Volatility is the upstream's judged subdomain, or unjudged.
	Volatility string   `json:"volatility"`
	Quadrant   Quadrant `json:"quadrant"`
	// Score is archfit's balance, max(|S−D|, 10−V)+1, from 1 (critical) to
	// 10 (balanced); reported, never gated.
	Score int `json:"score"`
	// LevelEvidence counts the evidence at the pair's level, EvidenceCount
	// all of it.
	LevelEvidence int      `json:"levelEvidence"`
	EvidenceCount int      `json:"evidenceCount"`
	Evidence      []string `json:"evidence"`
	// Site is where the strongest evidence lies.
	Site arch.Site `json:"site"`
}

// pairContext is what the pairs read beyond the evidence: the owners and
// the volatility of each end, by subsystem or, for pairs of packages, by
// package; and the distance and locality of two ends with no other boundary
// between them.
type pairContext struct {
	owners     map[string][]string
	deploys    deployUnits
	async      map[string]bool
	volatility map[string]string
	base       int
	locality   arch.Locality
}

// pairKey names an ordered pair of subsystems, downstream first.
type pairKey struct{ from, to string }

// groupPairs groups the evidence by ordered pair, leaving out a pair whose
// evidence only grades a level, which is no coupling on its own.
func groupPairs(ev []evidence) map[pairKey][]evidence {
	byPair := map[pairKey][]evidence{}
	for i := range ev {
		k := pairKey{ev[i].from, ev[i].to}
		byPair[k] = append(byPair[k], ev[i])
	}
	for k, items := range byPair {
		if gradesOnly(items) {
			delete(byPair, k)
		}
	}
	return byPair
}

// pairs builds one record per ordered pair of grouped evidence.
func (s *subject) pairs(groups map[pairKey][]evidence, ctx *pairContext) []Pair {
	out := make([]Pair, 0, len(groups))
	for k, items := range groups {
		out = append(out, s.pair(k.from, k.to, items, ctx))
	}
	slices.SortFunc(out, func(a, b Pair) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
	return out
}

// pair builds one pair's record from its evidence.
func (s *subject) pair(from, to string, items []evidence, ctx *pairContext) Pair {
	slices.SortFunc(items, func(a, b evidence) int { return compareEvidence(&a, &b) })
	// The strongest evidence comes first, a fact before a candidate.
	p := Pair{
		From: from, To: to, Level: max(items[0].level, LevelContract),
		Basis: items[0].basis, Site: items[0].site, Symmetric: true,
	}
	var kind arch.Strength
	for i := range items {
		p.Symmetric = p.Symmetric && items[i].symmetric
		if e := &items[i]; e.kind > kind && kindGrades(e.kind, p.Level) {
			kind = e.kind
		}
	}
	if kind > 0 {
		p.Kind = kind.String()
	}
	s.distance(&p, items, ctx)
	p.Volatility = cmp.Or(ctx.volatility[to], subdomainUnjudged)
	p.Quadrant, p.Score = quadrantOf(&p)
	p.EvidenceCount = len(items)
	for i := range items {
		if max(items[i].level, LevelContract) == p.Level {
			p.LevelEvidence++
		}
	}
	for i := range items[:min(len(items), maxListed)] {
		p.Evidence = append(p.Evidence, items[i].note)
	}
	return p
}

// gradesOnly reports evidence that only grades a level: static connascence
// candidates, such as two packages writing the same literal, with no
// reference or fact behind them. They grade a coupling the references show;
// alone, they are no coupling (connascence.md §9.2).
func gradesOnly(items []evidence) bool {
	for i := range items {
		if items[i].level != LevelNone || items[i].basis == arch.BasisFact {
			return false
		}
	}
	return true
}

// kindGrades reports whether a connascence kind is a degree of a level:
// static kinds grade contract and model coupling, dynamic kinds functional
// coupling, and nothing grades intrusive coupling (connascence.md §9.2).
func kindGrades(k arch.Strength, l Level) bool {
	switch l {
	case LevelContract, LevelModel:
		return !k.Dynamic()
	case LevelFunctional:
		return k.Dynamic() || k == arch.StrengthAlgorithm
	default:
		return false
	}
}

// distance sets a pair's distance facts and ordinal. The locality is the
// context's, two subsystems or two packages of one, unless the ends lie in
// different Go modules, which makes it external.
func (s *subject) distance(p *Pair, items []evidence, ctx *pairContext) {
	modules := s.boundaries(p, items, ctx)
	p.Locality = ctx.locality
	if modules > 1 {
		p.Locality = arch.LocalityExternal
	}
	a, b := ctx.owners[p.From], ctx.owners[p.To]
	p.OwnerBoundary = len(a) > 0 && len(b) > 0 && strings.Join(a, " ") != strings.Join(b, " ")
	p.Distance = ctx.base
	for _, d := range []struct {
		applies bool
		ordinal int
	}{
		{p.Locality == arch.LocalityExternal, distanceModule},
		{p.OwnerBoundary, distanceOwner},
		{p.Async, distanceAsync},
		{p.DeployBoundary, distanceDeploy},
	} {
		if d.applies {
			p.Distance = max(p.Distance, d.ordinal)
		}
	}
}

// boundaries sets the pair's asynchrony and deploy boundary from its
// evidence, and counts the Go modules its packages belong to.
func (s *subject) boundaries(p *Pair, items []evidence, ctx *pairContext) int {
	modules := map[string]bool{}
	for i := range items {
		e := &items[i]
		for _, pkg := range []string{e.fromPkg, e.toPkg} {
			if m, ok := s.c.Module(pkg); ok {
				modules[m] = true
			}
		}
		p.Async = p.Async || ctx.async[e.fromPkg] || e.async
		p.DeployBoundary = p.DeployBoundary ||
			(e.nonImport && ctx.deploys.separate(e.fromPkg, e.toPkg))
	}
	return len(modules)
}

// strengthOrdinal is archfit's ordinal of a level: contract 1, model 3,
// functional 8, symmetric functional coupling (duplicated code) 9, and
// intrusive 10.
func strengthOrdinal(p *Pair) int {
	switch p.Level {
	case LevelIntrusive:
		return 10
	case LevelFunctional:
		if p.Symmetric {
			return 9
		}
		return 8
	case LevelModel:
		return 3
	default:
		return 1
	}
}

// quadrantOf places a pair in the binary model and scores it with archfit's
// formula.
func quadrantOf(p *Pair) (Quadrant, int) {
	strong, far := p.Level >= LevelFunctional, p.Distance >= highDistance
	q := map[[2]bool]Quadrant{
		{false, true}:  QuadrantLooseCoupling,
		{true, false}:  QuadrantHighCohesion,
		{false, false}: QuadrantLowCohesion,
		{true, true}:   QuadrantTightCoupling,
	}[[2]bool{strong, far}]
	modularity := strengthOrdinal(p) - p.Distance
	if modularity < 0 {
		modularity = -modularity
	}
	return q, max(modularity, 10-volatilityOrdinal(p.Volatility)) + 1
}

// compareEvidence orders evidence strongest first, then facts before
// candidates, then by site and note.
func compareEvidence(a, b *evidence) int {
	return cmp.Or(
		cmp.Compare(b.level, a.level),
		cmp.Compare(basisRank(a.basis), basisRank(b.basis)),
		cmp.Compare(a.site.File, b.site.File),
		cmp.Compare(a.site.Line, b.site.Line),
		cmp.Compare(a.note, b.note),
	)
}

func basisRank(b arch.Basis) int {
	if b == arch.BasisFact {
		return 0
	}
	return 1
}
