// Package balance measures Vlad Khononov's Balanced Coupling for every
// ordered pair of subsystems: how much knowledge flows between them
// (integration strength), how far apart they are (distance), and how likely
// they are to change (volatility), with the balance of the three
// (connascence.md §9). It is a pure functional core: the shell loads the
// program, the connascence instances, the owners and the churn.
package balance

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Schema names the Balanced Coupling report format.
const Schema = "anarch-lint.balance.v1"

// maxListed caps the evidence and dependents a record lists, with a count
// beside each that keeps the total, and the hotspots and coverage the report
// lists, with Capped saying so.
const maxListed = 20

// Input is what the measure reads beyond the program and the model.
type Input struct {
	*arch.Input
	// Connascence is every instance and candidate the connascence measure
	// finds, with all their sites; Rejected names the fingerprints of those a
	// reviewer rejected, which are left out.
	Connascence []arch.Connascence
	Rejected    []string
	// Owners maps a package to the owners a CODEOWNERS file gives it; nil
	// without a file. A subsystem's own Owner in the model takes precedence.
	Owners map[string][]string
	// CodeownersHash identifies the CODEOWNERS file read, "" without one.
	CodeownersHash string
	// Churn is the change history of a revision range, nil without one.
	Churn *arch.Churn
	// Coverage is a test coverage profile, nil without one.
	Coverage *arch.Coverage
}

// Report is the complete, deterministic result of the measure.
type Report struct {
	Schema string       `json:"schema"`
	Inputs ReportInputs `json:"inputs"`
	// Pairs are the ordered pairs of subsystems that share knowledge, from
	// the downstream subsystem to the upstream one.
	Pairs      []Pair                `json:"pairs"`
	Packages   []PackageBalance      `json:"packages"`
	Subsystems []SubsystemVolatility `json:"subsystems"`
	// CoChange lists the subsystems that change together, and Hotspots the
	// packages that change often and are complex; both need a revision range.
	CoChange []CoChange `json:"coChange"`
	Hotspots []Hotspot  `json:"hotspots"`
	// Coverage lists the packages a coverage profile instruments, riskiest
	// first; it needs --coverprofile.
	Coverage []PackageCoverage `json:"coverage"`
	// Capped names each list the report cut at maxListed entries, with its
	// whole length, such as "hotspots: 20 of 57".
	Capped []string `json:"capped"`
	// Unbound lists the packages no binary in scope contains, when some
	// binary is in scope; their deploy boundary is unknown.
	Unbound    []string       `json:"unbound"`
	Findings   []arch.Finding `json:"findings"`
	Candidates []Candidate    `json:"candidates"`
	Rejected   []Candidate    `json:"rejected"`
	// Suppressed lists what a verdict lifted: a tight coupling to a
	// supporting or generic subdomain, or a model two bounded contexts share
	// by design. Each names the slot and who attested it, for audit.
	Suppressed []Suppression `json:"suppressed"`
	// PackagePairs are the pairs of packages within one subsystem, when the
	// model asks for them: where distance is low, so weak coupling, not
	// strong, is the imbalance (connascence.md §9.8). Reported, never gated.
	PackagePairs []Pair `json:"packagePairs,omitempty"`
	// Contracts are the contracts the program provides and consumes, and
	// ContractPairs those it both provides and consumes, in different
	// packages: coupling through a contract (IMPROVEMENTS.md §5.1).
	Contracts     []arch.Contract     `json:"contracts"`
	ContractPairs []arch.ContractPair `json:"contractPairs"`
	// Judgements ask a reviewer for subdomains, bounded contexts,
	// integration patterns, contract types, asynchrony, and the candidates
	// that would gate if confirmed.
	Judgements []arch.JudgementSlot `json:"judgements"`
	// Skipped lists the slots not opened because no answer would change
	// anything, each with the reason.
	Skipped     []arch.SkippedSlot `json:"skipped"`
	Assumptions []arch.Assumption  `json:"assumptions"`
}

// Suppression is a finding, or a candidate, that a verdict lifted: the rule
// it would have broken, the pair, and the slot whose answer lifted it, with
// the events that attest that answer.
type Suppression struct {
	Rule      string                   `json:"rule"`
	Subject   string                   `json:"subject"`
	Object    string                   `json:"object"`
	Criterion string                   `json:"criterion"`
	Answer    string                   `json:"answer"`
	Events    []arch.VerificationEvent `json:"events"`
}

// ReportInputs are a report's inputs: the analysis inputs, and what bounds
// the owners and the churn.
type ReportInputs struct {
	arch.Inputs
	Codeowners string `json:"codeowners,omitempty"`
	Since      string `json:"since,omitempty"`
	Head       string `json:"head,omitempty"`
	Coverage   string `json:"coverage,omitempty"`
}

// Candidate is evidence of imbalance that may be coincidence: it gates only
// once a reviewer confirms it, and only when its rule gates at all.
type Candidate struct {
	Fingerprint string     `json:"fingerprint"`
	Rule        string     `json:"rule"`
	Subject     string     `json:"subject"`
	Object      string     `json:"object,omitempty"`
	Message     string     `json:"message"`
	Fix         string     `json:"fix"`
	Evidence    []string   `json:"evidence"`
	Site        *arch.Site `json:"site,omitempty"`
}

// subject is what the measure's parts read: the input, the graphs, the
// containers of locality, and lookups built once.
type subject struct {
	in    *Input
	p     *arch.Program
	m     *arch.Model
	g     arch.Graphs
	c     arch.Containers
	roles map[string]string
	types map[string]*arch.Type
	slots *slotBook
	// domains holds the domain models by the subsystem each describes.
	domains *domainIndex
}

// Analyze measures the Balanced Coupling of in's program under its model. It
// is a pure function: the same input always gives the same report, and the
// input is not modified.
//
// Requires: in.Program is sorted, as [arch.Program] documents, and carries
// its [arch.Facts]; in.Connascence comes from the same program and model.
func Analyze(in *Input) Report {
	s := newSubject(in)
	r := Report{
		Schema:      Schema,
		Inputs:      s.reportInputs(),
		Unbound:     make([]string, 0),
		Findings:    make([]arch.Finding, 0),
		Candidates:  make([]Candidate, 0),
		Rejected:    make([]Candidate, 0),
		Suppressed:  make([]Suppression, 0),
		Assumptions: assumptions(),
		Capped:      make([]string, 0),
	}
	r.Contracts = append(make([]arch.Contract, 0), arch.ContractsOf(&s.p.Facts)...)
	pairs, _ := arch.MatchContracts(r.Contracts)
	r.ContractPairs = append(make([]arch.ContractPair, 0), pairs...)
	ev := s.classifier().evidence()
	owners, ownerFindings := s.subsystemOwners()
	deploys := s.deployUnits()
	r.Unbound = deploys.unbound
	async := s.asyncPackages()
	groups := groupPairs(ev)
	var skippedSubdomains []arch.SkippedSlot
	r.Subsystems, skippedSubdomains = s.volatility(ev, groups)
	volatility := map[string]string{}
	for _, v := range r.Subsystems {
		volatility[v.Name] = v.Subdomain
	}
	r.Pairs = s.pairs(groups, &pairContext{
		owners: owners, deploys: deploys, async: async, volatility: volatility,
		base: distanceSubsystem, locality: arch.LocalitySystem,
	})
	if s.m.Balance.Packages {
		r.PackagePairs = s.packagePairs(&pairContext{
			owners: in.Owners, deploys: deploys, async: async,
			volatility: s.packageVolatility(volatility),
			base:       distancePackage, locality: arch.LocalitySubsystem,
		})
	}
	r.Skipped = slices.Concat(
		make([]arch.SkippedSlot, 0),
		skippedSubdomains,
		s.boundedContexts(&r, ev),
	)
	view := &packageView{p: s.p, g: &s.g, severity: s.severity()}
	r.Packages = view.packages(ev)
	r.Findings = append(r.Findings, ownerFindings...)
	r.Findings = append(r.Findings, view.exposureFindings(r.Packages)...)
	r.Candidates = append(r.Candidates, s.churnCandidates(r.Subsystems)...)
	s.historyAndCoverage(&r)
	(&judge{p: s.p, slots: s.slots, domains: s.domains, severity: s.severity()}).apply(&r)
	r.Judgements = s.slots.all()
	arch.SortFindings(r.Findings)
	sortCandidates(r.Candidates)
	sortCandidates(r.Rejected)
	return r
}

// packagePairs pairs the packages of each subsystem by the evidence
// between them, which the classifier groups by package.
func (s *subject) packagePairs(ctx *pairContext) []Pair {
	k := s.classifier()
	k.group = func(pkg string) string { return pkg }
	all := k.evidence()
	var within []evidence
	for i := range all {
		if s.subsystemOf(all[i].fromPkg) == s.subsystemOf(all[i].toPkg) {
			within = append(within, all[i])
		}
	}
	return s.pairs(groupPairs(within), ctx)
}

// packageVolatility gives each package its subsystem's volatility.
func (s *subject) packageVolatility(bySubsystem map[string]string) map[string]string {
	out := map[string]string{}
	for _, pkg := range s.p.Packages {
		out[pkg.Path] = bySubsystem[s.subsystemOf(pkg.Path)]
	}
	return out
}

// newSubject builds the graphs and lookups every part reads.
func newSubject(in *Input) *subject {
	p, m := &in.Program, &in.Model
	g := arch.BuildGraphs(p, m)
	s := &subject{
		in:      in,
		p:       p,
		m:       m,
		g:       g,
		c:       arch.NewContainers(p, &g),
		roles:   arch.TypeRoles(p, m),
		types:   map[string]*arch.Type{},
		slots:   &slotBook{verdicts: in.Verdicts, byID: map[string]arch.JudgementSlot{}},
		domains: newDomainIndex(m, in.Domains),
	}
	for i := range p.Types {
		s.types[p.Types[i].ID] = &p.Types[i]
	}
	return s
}

// classifier returns the classifier of the subject's couplings.
func (s *subject) classifier() *classifier {
	sigs := map[string]*arch.Signature{}
	for i := range s.p.Facts.Signatures {
		sigs[s.p.Facts.Signatures[i].Func] = &s.p.Facts.Signatures[i]
	}
	consts := map[string]*arch.Constant{}
	for i := range s.p.Facts.Constants {
		consts[s.p.Facts.Constants[i].ID] = &s.p.Facts.Constants[i]
	}
	return &classifier{
		p: s.p, m: s.m, g: &s.g, c: s.c, roles: s.roles, sigs: sigs, consts: consts,
		types: s.types, slots: s.slots, connascence: s.in.Connascence, rejected: s.in.Rejected,
		entities: s.in.EntityTypes(&s.g), apis: s.in.APIs,
	}
}

// reportInputs copies the analysis inputs with the model hash and the bounds
// of the owners and the churn.
func (s *subject) reportInputs() ReportInputs {
	in := s.in.Inputs
	in.ModelHash, in.Thresholds = s.m.Hash(), s.m.Limits()
	in.Tags = append(make([]string, 0, len(in.Tags)), in.Tags...)
	in.Patterns = append(make([]string, 0, len(in.Patterns)), in.Patterns...)
	out := ReportInputs{Inputs: in, Codeowners: s.in.CodeownersHash}
	if s.in.Churn != nil {
		out.Since, out.Head = s.in.Churn.Since, s.in.Churn.Head
	}
	if s.in.Coverage != nil {
		out.Coverage = s.in.Coverage.Hash
	}
	return out
}

// severity is error when the model opts the findings into the gate.
func (s *subject) severity() arch.Severity {
	if s.m.Balance.Enabled() {
		return arch.SeverityError
	}
	return arch.SeverityWarning
}

// subsystemOf returns a package's subsystem, or the package itself.
func (s *subject) subsystemOf(pkg string) string {
	return s.c.SubsystemOf(arch.Site{Package: pkg})
}

// sortCandidates orders candidates by rule, subject, object and fingerprint.
func sortCandidates(cs []Candidate) {
	slices.SortFunc(cs, func(a, b Candidate) int {
		return cmp.Or(
			cmp.Compare(a.Rule, b.Rule),
			cmp.Compare(a.Subject, b.Subject),
			cmp.Compare(a.Object, b.Object),
			cmp.Compare(a.Fingerprint, b.Fingerprint),
		)
	})
}

// AddTo adds the report's findings to the gate, and ratchets, for every pair
// coupled at the functional or intrusive level on facts, how many pieces of
// evidence show that level: more of them is more strong coupling across the
// same boundary.
func (r *Report) AddTo(in *arch.GateInput) {
	var metrics []arch.Metric
	for i := range r.Pairs {
		if p := &r.Pairs[i]; p.Level >= LevelFunctional && p.Basis == arch.BasisFact {
			metrics = append(metrics, arch.Metric{
				Name: "balance.strongRefs/" + p.From + "->" + p.To, Value: float64(p.LevelEvidence),
			})
		}
	}
	in.Add(r.Findings, metrics)
}

// listed returns the first maxListed items, and records in r.Capped when
// the cap left some out.
func listed[T any](r *Report, name string, items []T) []T {
	if len(items) <= maxListed {
		return items
	}
	r.Capped = append(r.Capped, fmt.Sprintf("%s: %d of %d", name, maxListed, len(items)))
	return items[:maxListed]
}

// historyAndCoverage adds what the revision range and the coverage profile
// show, when the input has them: co-change, hotspots and coverage, and the
// candidates drawn from them.
func (s *subject) historyAndCoverage(r *Report) {
	h := &history{churn: s.in.Churn, p: s.p, g: &s.g, variant: s.m.Limits().ComplexityVariant}
	r.CoChange = h.coChange()
	r.Hotspots = listed(r, "hotspots", h.hotspots())
	r.Candidates = append(r.Candidates, h.hiddenCouplings(r.CoChange)...)
	coverage := coverageOf(s.in.Coverage, s.p, r.Packages)
	r.Candidates = append(r.Candidates, untestedDependencies(coverage)...)
	crossings := crossingCheck{in: s.in, p: s.p, c: s.c, subsystemOf: s.subsystemOf}
	r.Candidates = append(r.Candidates, crossings.untestedCrossings()...)
	r.Coverage = listed(r, "coverage", coverage)
}
