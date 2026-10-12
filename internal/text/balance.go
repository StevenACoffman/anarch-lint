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
	writeVolatility(tw, r.Subsystems)
	writeExposure(tw, r.Packages)
	if len(r.Unbound) > 0 {
		_, _ = fmt.Fprintf(tw, "\nNO DEPLOY UNIT: %s\n", strings.Join(r.Unbound, ", "))
	}
	writeSlots(tw, r.Judgements)
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
