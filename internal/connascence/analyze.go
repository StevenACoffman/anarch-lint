// Package connascence measures the connascence of a Go program
// deterministically: what it can count exactly, what it can only flag as a
// candidate, and what a reviewer must judge (connascence.md). It is a pure
// functional core over the facts of [arch.Program]; the shell loads them.
package connascence

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Schema names the connascence report format.
const Schema = "anarch-lint.connascence.v1"

// maxReportedSites caps the sites an instance lists; SiteCount keeps the
// total. The first sites in sorted order are kept, so the cap is
// deterministic (connascence.md §6).
const maxReportedSites = 20

// Report is the complete, deterministic result of measuring a program's
// connascence (connascence.md §1–§7).
type Report struct {
	Schema string      `json:"schema"`
	Inputs arch.Inputs `json:"inputs"`
	// Summary counts the instances of each kind at each locality.
	Summary []arch.ConnascenceCount `json:"summary"`
	// Instances are the facts: sharing that is itself the coupling.
	Instances []arch.Connascence `json:"instances"`
	// Candidates are evidence that may be coincidence, ranked strongest,
	// farthest and widest first. Rejected are those a reviewer rejected.
	Candidates []arch.Connascence `json:"candidates"`
	Rejected   []arch.Connascence `json:"rejected"`
	// Findings are the instances over the model's limits, and the candidates
	// a reviewer confirmed that are over them.
	Findings []arch.Finding `json:"findings"`
	// Judgements ask a reviewer about each candidate a limit would gate.
	Judgements  []arch.JudgementSlot `json:"judgements"`
	Assumptions []arch.Assumption    `json:"assumptions"`
}

// subject is what the detectors read: the program, the model and its
// graphs, the containers of locality, and lookups built once from the facts.
type subject struct {
	p *arch.Program
	m *arch.Model
	g *arch.Graphs
	c arch.Containers
	// decl maps an object ID to its declaration, where one is known.
	decl map[string]arch.Site
	// usesOf lists every Use of an object, in the facts' order.
	usesOf map[string][]arch.Use
}

// instanceSpec is what a detector knows about an instance before its axes
// are computed.
type instanceSpec struct {
	kind       arch.Strength
	detector   string
	subject    string
	object     string
	basis      arch.Basis
	confidence arch.Confidence
	sites      []arch.Site
	message    string
	fix        string
}

// Analyze measures the connascence of in's program under its model, filling
// the judgement slots from its verdicts. It is a pure function: the same
// input always gives the same report, and the input is not modified.
//
// Requires: in.Program is sorted, as [arch.Program] documents, and carries
// its [arch.Facts].
func Analyze(input *arch.Input) Report {
	p, m, verdicts, in := &input.Program, &input.Model, input.Verdicts, &input.Inputs
	s := newSubject(p, m)
	inputs := *in
	inputs.ModelHash, inputs.Thresholds = m.Hash(), m.Limits()
	inputs.Tags = append(make([]string, 0, len(in.Tags)), in.Tags...)
	inputs.Patterns = append(make([]string, 0, len(in.Patterns)), in.Patterns...)

	r := Report{
		Schema:      Schema,
		Inputs:      inputs,
		Instances:   make([]arch.Connascence, 0),
		Candidates:  make([]arch.Connascence, 0),
		Rejected:    make([]arch.Connascence, 0),
		Findings:    make([]arch.Finding, 0),
		Judgements:  make([]arch.JudgementSlot, 0),
		Assumptions: assumptions(),
	}
	all := s.instances()
	for i := range all {
		r.place(&all[i], m, verdicts)
	}
	slices.SortFunc(r.Candidates, func(a, b arch.Connascence) int { return rankInstances(&a, &b) })
	slices.SortFunc(r.Rejected, func(a, b arch.Connascence) int { return rankInstances(&a, &b) })
	arch.SortFindings(r.Findings)
	r.Summary = summarize(all)
	return r
}

// RejectedFingerprints returns the fingerprints of the candidates a reviewer
// rejected, for a measure that reads [Instances] and must leave them out.
func (r *Report) RejectedFingerprints() []string {
	out := make([]string, len(r.Rejected))
	for i := range r.Rejected {
		out[i] = r.Rejected[i].Fingerprint
	}
	return out
}

// AddTo adds the report's findings to the gate, and ratchets the degree of
// every fact of meaning or stronger at a subsystem boundary or beyond: a new
// site of an instance keeps its fingerprint, so growth is caught as a
// regression instead (connascence.md §6). Name and type are left out:
// another use of an exported name is ordinary work, not degradation (§2.1,
// §2.2).
func (r *Report) AddTo(in *arch.GateInput) {
	var metrics []arch.Metric
	for i := range r.Instances {
		if c := &r.Instances[i]; c.Locality >= arch.LocalitySubsystem &&
			c.Kind >= arch.StrengthMeaning {
			metrics = append(metrics, arch.Metric{
				Name: "connascence.degree/" + c.Fingerprint, Value: float64(c.Degree),
			})
		}
	}
	in.Add(r.Findings, metrics)
}

// Instances returns every instance and candidate the detectors find in p
// under m, sorted, each with all of its sites: what another measure, such as
// Balanced Coupling, reads. A report lists only the first sites of each.
//
// Requires: p is sorted, as [arch.Program] documents, and carries its
// [arch.Facts].
func Instances(p *arch.Program, m *arch.Model) []arch.Connascence {
	return newSubject(p, m).instances()
}

// instances runs every detector and sorts what they find.
func (s *subject) instances() []arch.Connascence {
	var all []arch.Connascence
	for _, detect := range detectors() {
		all = append(all, detect(s)...)
	}
	slices.SortFunc(all, func(a, b arch.Connascence) int { return compareInstances(&a, &b) })
	return all
}

// capped returns c with only its first sites, as a report lists it.
func capped(c *arch.Connascence) arch.Connascence {
	out := *c
	out.Sites = c.Sites[:min(len(c.Sites), maxReportedSites)]
	return out
}

// place files one instance as a fact or a candidate, and decides whether it
// is a finding: a fact over the limits always is; a candidate over them is
// once a reviewer confirms it, and a rejected one is set aside.
func (r *Report) place(c *arch.Connascence, m *arch.Model, verdicts []arch.Verdict) {
	reason := overLimits(c, &m.Connascence)
	if c.Basis == arch.BasisFact {
		r.Instances = append(r.Instances, capped(c))
		if reason != "" {
			r.Findings = append(r.Findings, connascenceFinding(c, reason, m))
		}
		return
	}
	if reason == "" {
		r.Candidates = append(r.Candidates, capped(c))
		return
	}
	slot := arch.NewSlot(&arch.SlotRequest{
		Criterion: "connascence/" + c.Fingerprint,
		Evidence:  candidateEvidence(c),
		Context:   []string{"over the limits: " + reason},
		Undecided: fmt.Sprintf("a detector found the shape of connascence, with %s confidence "+
			"that it is not chance, but not that the places must change together", c.Confidence),
	}, verdicts)
	r.Judgements = append(r.Judgements, slot)
	switch slot.Answer() {
	case "confirmed":
		r.Candidates = append(r.Candidates, capped(c))
		r.Findings = append(r.Findings, connascenceFinding(c, reason, m))
	case "rejected":
		r.Rejected = append(r.Rejected, capped(c))
	default:
		r.Candidates = append(r.Candidates, capped(c))
	}
}

// overLimits says how an instance exceeds the model's limits at its
// locality, or returns "" when it does not.
func overLimits(c *arch.Connascence, limits *arch.ConnascenceLimits) string {
	var over []string
	if limit := limits.StrengthLimit(c.Locality); c.Strength > limit {
		over = append(over, fmt.Sprintf("above maxStrength.%s %d", c.Locality, limit))
	}
	if limit, ok := limits.DegreeLimit(c.Locality); ok && c.Degree > limit {
		over = append(over, fmt.Sprintf("above maxDegree.%s %d", c.Locality, limit))
	}
	return strings.Join(over, " and ")
}

// connascenceFinding turns an instance over the limits into a finding. Its
// fingerprint is the instance's, so a waiver or a baseline names both.
func connascenceFinding(c *arch.Connascence, reason string, m *arch.Model) arch.Finding {
	sev := arch.SeverityWarning
	if m.Connascence.Enabled() {
		sev = arch.SeverityError
	}
	f := arch.NewFinding(
		arch.FindingID{
			Rule:    c.Kind.Rule(),
			Subject: c.Subject,
			Object:  c.Detector,
		},
		sev,
	)
	f.Location = arch.SpanAt(arch.Position{File: c.Sites[0].File, Line: c.Sites[0].Line})
	f.Message = c.Message
	f.Fix = c.Fix
	f.Evidence = append([]string{axes(c) + ": " + reason}, siteLines(c)...)
	return f
}

// candidateEvidence is what a judge weighs for one candidate: what it is
// and where, never the limits it exceeds, which say what a confirmation
// would set off.
func candidateEvidence(c *arch.Connascence) []string {
	ev := []string{c.Message, axes(c)}
	return append(ev, siteLines(c)...)
}

// axes describes an instance's three axes, as connascence.md §7 shows them.
func axes(c *arch.Connascence) string {
	return fmt.Sprintf("strength %d (%s), locality %s, degree %d",
		c.Strength, c.Kind, c.Locality, c.Degree)
}

// siteLines lists the reported sites, and how many more there are.
func siteLines(c *arch.Connascence) []string {
	shown := c.Sites[:min(len(c.Sites), maxReportedSites)]
	lines := make([]string, 0, len(shown)+1)
	for _, s := range shown {
		lines = append(lines, "site: "+siteText(s))
	}
	if more := c.SiteCount - len(shown); more > 0 {
		lines = append(lines, "and "+count(more, "more site", "more sites"))
	}
	return lines
}

// siteText is a site as reports print it.
func siteText(s arch.Site) string {
	where := s.File + ":" + strconv.Itoa(s.Line)
	if who := cmp.Or(s.Func, s.Type); who != "" {
		return where + " (" + who + ")"
	}
	return where
}

// newSubject builds the graphs and the lookups the detectors share.
func newSubject(p *arch.Program, m *arch.Model) *subject {
	g := arch.BuildGraphs(p, m)
	s := &subject{
		p:      p,
		m:      m,
		g:      &g,
		c:      arch.NewContainers(p, &g),
		decl:   map[string]arch.Site{},
		usesOf: map[string][]arch.Use{},
	}
	for i := range s.p.Types {
		t := &s.p.Types[i]
		s.decl[t.ID] = arch.Site{
			File:    t.Span.File,
			Line:    t.Span.Start,
			Package: t.Package,
			Type:    t.ID,
		}
	}
	for i := range s.p.Funcs {
		f := &s.p.Funcs[i]
		s.decl[f.ID] = arch.Site{
			File: f.Span.File, Line: f.Span.Start, Package: f.Package, Func: f.ID, Type: f.Receiver,
		}
	}
	for i := range s.p.Facts.Constants {
		c := &s.p.Facts.Constants[i]
		s.decl[c.ID] = c.Site
	}
	for i := range s.p.Facts.Uses {
		u := &s.p.Facts.Uses[i]
		s.usesOf[u.Object] = append(s.usesOf[u.Object], *u)
	}
	return s
}

// instance computes an instance's axes from its sites.
//
// Requires: sp.sites is not empty.
func (s *subject) instance(sp *instanceSpec) arch.Connascence {
	sites := slices.Clone(sp.sites)
	slices.SortFunc(sites, arch.CompareSites)
	sites = slices.Compact(sites)
	locality := s.c.LocalityOf(sites)
	f := arch.NewFinding(
		arch.FindingID{
			Rule:    sp.kind.Rule(),
			Subject: sp.subject,
			Object:  sp.detector,
		},
		arch.SeverityWarning,
	)
	return arch.Connascence{
		Fingerprint: f.Fingerprint,
		Kind:        sp.kind,
		Strength:    int(sp.kind),
		Detector:    sp.detector,
		Subject:     sp.subject,
		Object:      sp.object,
		Basis:       sp.basis,
		Confidence:  sp.confidence,
		Locality:    locality,
		Degree:      s.c.DegreeOf(sites, locality),
		SiteCount:   len(sites),
		Sites:       sites,
		Message:     sp.message,
		Fix:         sp.fix,
	}
}

// usesFrom returns the sites of an object's uses, with its declaration when
// known: everything that must agree on the object.
func (s *subject) usesFrom(object string) []arch.Site {
	var sites []arch.Site
	if d, ok := s.decl[object]; ok {
		sites = append(sites, d)
	}
	for i := range s.usesOf[object] {
		u := &s.usesOf[object][i]
		sites = append(sites, u.Site)
	}
	return sites
}

// compareInstances orders instances by kind, subject, detector and first
// site.
func compareInstances(a, b *arch.Connascence) int {
	return cmp.Or(
		cmp.Compare(a.Kind, b.Kind),
		cmp.Compare(a.Subject, b.Subject),
		cmp.Compare(a.Detector, b.Detector),
		arch.CompareSites(a.Sites[0], b.Sites[0]),
	)
}

// rankInstances orders candidates strongest, farthest and widest first.
func rankInstances(a, b *arch.Connascence) int {
	return cmp.Or(
		cmp.Compare(b.Strength, a.Strength),
		cmp.Compare(b.Locality, a.Locality),
		cmp.Compare(b.Degree, a.Degree),
		cmp.Compare(a.Fingerprint, b.Fingerprint),
	)
}

// summarize counts the instances of each kind at each locality, in order.
func summarize(all []arch.Connascence) []arch.ConnascenceCount {
	type cell struct {
		kind     arch.Strength
		locality arch.Locality
	}
	counts := map[cell]*arch.ConnascenceCount{}
	for i := range all {
		c := &all[i]
		k := cell{c.Kind, c.Locality}
		if counts[k] == nil {
			counts[k] = &arch.ConnascenceCount{Kind: c.Kind, Locality: c.Locality}
		}
		if c.Basis == arch.BasisFact {
			counts[k].Facts++
		} else {
			counts[k].Candidates++
		}
		counts[k].Degree += c.Degree
	}
	out := make([]arch.ConnascenceCount, 0, len(counts))
	for _, c := range counts {
		out = append(out, *c)
	}
	slices.SortFunc(out, func(a, b arch.ConnascenceCount) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Locality, b.Locality))
	})
	return out
}
