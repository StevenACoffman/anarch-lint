package text

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
)

// WriteBalance writes a Balanced Coupling report: one row per subsystem pair
// naming every dimension, then the findings, the candidates, the subsystems'
// volatility, the exposed packages, the judgement slots and a count of the
// assumptions.
func WriteBalance(w io.Writer, r *balance.Report) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "anarch-lint balance (%s)\n", balance.Schema)
	_, _ = fmt.Fprintf(tw, "patterns %s · model %s · codeowners %s · churn %s · %s\n",
		strings.Join(r.Inputs.Patterns, " "), short(r.Inputs.ModelHash),
		orNone(short(r.Inputs.Codeowners)), churnRange(&r.Inputs), r.Inputs.GoVersion)
	writePairs(tw, r.Pairs)
	writeFindings(tw, r.Findings)
	writeBalanceCandidates(tw, r.Candidates, len(r.Rejected))
	writeSuppressed(tw, r.Suppressed)
	writeVolatility(tw, r.Subsystems)
	writeTemporal(tw, r)
	writeCoverage(tw, r)
	writeExposure(tw, r.Packages)
	if len(r.Unbound) > 0 {
		_, _ = fmt.Fprintf(tw, "\nNO DEPLOY UNIT: %s\n", strings.Join(r.Unbound, ", "))
	}
	writeSlots(tw, r.Judgements)
	writeSkipped(tw, r.Skipped)
	_, _ = fmt.Fprintf(tw, "\nASSUMPTIONS: %d, listed in the JSON report (--format=json)\n",
		len(r.Assumptions))
	if err := tw.Flush(); err != nil {
		return &arch.Error{Op: "text.WriteBalance", Err: err}
	}
	return nil
}

func writePairs(w io.Writer, pairs []balance.Pair) {
	_, _ = fmt.Fprintln(
		w,
		"\nPAIRS (downstream -> upstream: strength, distance, volatility, balance)",
	)
	_, _ = fmt.Fprintln(w, "  pair\tstrength\tdistance\tvolatility\tquadrant\tbalance\t")
	for i := range pairs {
		p := &pairs[i]
		strength := p.Level.String()
		if p.Kind != "" {
			strength += " (" + p.Kind + ")"
		}
		if p.Basis == arch.BasisCandidate {
			strength += " candidate"
		}
		arrow := " -> "
		if p.Symmetric {
			arrow = " <-> "
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%d\t\n", p.From+arrow+p.To, strength,
			distanceText(p), p.Volatility, p.Quadrant, p.Score)
	}
}

// distanceText is a pair's distance ordinal with the boundaries behind it.
func distanceText(p *balance.Pair) string {
	parts := []string{strconv.Itoa(p.Distance), p.Locality.String()}
	for _, b := range []struct {
		on   bool
		name string
	}{{p.OwnerBoundary, "owners"}, {p.DeployBoundary, "deploy"}, {p.Async, "async"}} {
		if b.on {
			parts = append(parts, b.name)
		}
	}
	return strings.Join(parts, " ")
}

func writeBalanceCandidates(w io.Writer, cs []balance.Candidate, rejected int) {
	_, _ = fmt.Fprintf(w, "\nCANDIDATES (%d, %d rejected)\n", len(cs), rejected)
	for i := range cs {
		c := &cs[i]
		subject := c.Subject
		if c.Object != "" {
			subject += " -> " + c.Object
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s\n", c.Rule, subject)
		_, _ = fmt.Fprintf(w, "  \t%s\n", c.Message)
		_, _ = fmt.Fprintf(w, "  \tfix: %s\n", c.Fix)
	}
}

// writeSuppressed lists what verdicts lifted, and who attested each, so a
// lift is never silent.
func writeSuppressed(w io.Writer, ss []balance.Suppression) {
	if len(ss) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "\nSUPPRESSED BY VERDICTS (%d)\n", len(ss))
	for i := range ss {
		s := &ss[i]
		by := make([]string, len(s.Events))
		for j, e := range s.Events {
			by[j] = string(e.Role)
			if e.Actor != "" {
				by[j] += " " + e.Actor
			}
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s -> %s\n", s.Rule, s.Subject, s.Object)
		_, _ = fmt.Fprintf(w, "  \t%s answered %s, attested by %s\n",
			s.Criterion, s.Answer, strings.Join(by, ", "))
	}
}

// writeSkipped counts the slots not opened, with why each would decide
// nothing.
func writeSkipped(w io.Writer, skipped []balance.SkippedSlot) {
	_, _ = fmt.Fprintf(w, "\nSKIPPED SLOTS (%d; an answer would change nothing)\n", len(skipped))
	for _, s := range skipped {
		_, _ = fmt.Fprintf(w, "  %s\t%s\n", s.Criterion, s.Reason)
	}
}

func writeVolatility(w io.Writer, subs []balance.SubsystemVolatility) {
	_, _ = fmt.Fprintln(w, "\nVOLATILITY (judged subdomain; churn is evidence, not volatility)")
	for _, v := range subs {
		commits := "-"
		if v.Commits != nil {
			commits = strconv.Itoa(*v.Commits) + " commits"
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s\tbounded context %s\t%s\t\n",
			v.Name, v.Subdomain, orNone(v.BoundedContext), commits)
	}
}

// writeTemporal lists the subsystems that change together and the hotspots,
// which only a revision range gives.
func writeTemporal(w io.Writer, r *balance.Report) {
	if r.Inputs.Since == "" {
		return
	}
	_, _ = fmt.Fprintf(w, "\nCO-CHANGE (%d pairs; temporal coupling the imports may not show)\n",
		len(r.CoChange))
	for _, c := range r.CoChange {
		_, _ = fmt.Fprintf(
			w,
			"  %s ↔ %s\t%s shared\tscore %g\t\n",
			c.A,
			c.B,
			arch.Plural(c.Shared, "commit"),
			c.Score,
		)
	}
	_, _ = fmt.Fprintf(w, "\nHOTSPOTS (%d; commits × cyclomatic complexity)\n", len(r.Hotspots))
	for _, h := range r.Hotspots {
		_, _ = fmt.Fprintf(
			w,
			"  %s\t%s\tcomplexity %d\t\n",
			h.Package,
			arch.Plural(h.Commits, "commit"),
			h.Complexity,
		)
	}
}

// writeCoverage lists the packages a coverage profile instruments, riskiest
// first, when the report read one.
func writeCoverage(w io.Writer, r *balance.Report) {
	if r.Inputs.Coverage == "" {
		return
	}
	_, _ = fmt.Fprintf(w, "\nCOVERAGE (%d packages; Ca × uncovered statements)\n", len(r.Coverage))
	for _, c := range r.Coverage {
		_, _ = fmt.Fprintf(w, "  %s\tCa %d\t%d of %d statements\t%.0f%%\t\n",
			c.Package, c.Ca, c.Covered, c.Statements, 100*c.Share)
	}
}

// writeExposure lists the packages whose upstream dependency is functional or
// stronger, with their weighted instability.
func writeExposure(w io.Writer, pkgs []balance.PackageBalance) {
	_, _ = fmt.Fprintln(
		w,
		"\nEXPOSURE (packages with a functional or intrusive upstream dependency)",
	)
	for i := range pkgs {
		p := &pkgs[i]
		if p.Upstream < balance.LevelFunctional {
			continue
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s %s\tCa %d\tdependents %d\tweighted instability %.3f\t\n",
			p.Path, p.Upstream, p.UpstreamBasis, p.Ca, p.Dependents, p.WeightedInstability)
	}
}

// churnRange is the revision range of the churn, or "none".
func churnRange(in *balance.ReportInputs) string {
	if in.Since == "" {
		return "none"
	}
	return short(in.Since) + ".." + short(in.Head)
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
