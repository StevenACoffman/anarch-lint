package text

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// WriteFitness writes a fitness report: the rule table first, as ArchUnit
// leads with its rules, then the findings, the metrics, the cycles and a
// count of the assumptions.
func WriteFitness(w io.Writer, r *fitness.Report) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "anarch-lint fitness (%s)\n", fitness.Schema)
	_, _ = fmt.Fprintf(tw, "patterns %s · model %s · %s\n",
		strings.Join(r.Inputs.Patterns, " "), short(r.Inputs.ModelHash), r.Inputs.GoVersion)
	writeRuleTable(tw, r.Rules)
	writeFindings(tw, r.Findings)
	writeLakos(tw, r.Lakos)
	_, _ = fmt.Fprintf(
		tw,
		"\nVISIBILITY (Dowalil)  ARV %.3f  GRV %.3f\n",
		r.Visibility.ARV,
		r.Visibility.GRV,
	)
	writeComponents(tw, r.ComponentDependencyMetrics)
	writeCycles(tw, r.Cycles)
	_, _ = fmt.Fprintf(tw, "\nASSUMPTIONS: %d, listed in the JSON report (--format=json)\n",
		len(r.Assumptions))
	if err := tw.Flush(); err != nil {
		return &arch.Error{Op: "text.WriteFitness", Err: err}
	}
	return nil
}

func writeRuleTable(w io.Writer, rules []fitness.RuleResult) {
	_, _ = fmt.Fprintln(w, "\nRULES")
	_, _ = fmt.Fprintln(w, "  rule\tkind\tgate\tmatched\tviolations\tstatus\treason\t")
	for _, r := range rules {
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%d\t%d\t%s\t%s\t\n",
			r.Name, r.Kind, r.Gate, r.Matched, r.Violations, r.Status, orNone(r.Reason))
	}
}

func writeLakos(w io.Writer, levels []fitness.Lakos) {
	_, _ = fmt.Fprintln(w, "\nLAKOS")
	_, _ = fmt.Fprintln(w, "  level\tnodes\tCCD\tACD\tRACD\tNCCD\t")
	for _, l := range levels {
		_, _ = fmt.Fprintf(
			w,
			"  %s\t%d\t%d\t%.3f\t%.3f\t%.3f\t\n",
			l.Level,
			l.Nodes,
			l.CCD,
			l.ACD,
			l.RACD,
			l.NCCD,
		)
	}
}

// writeComponents lists the packages furthest from the main sequence first.
func writeComponents(w io.Writer, cs []fitness.ComponentDependency) {
	_, _ = fmt.Fprintln(w, "\nCOMPONENT DEPENDENCY METRICS (ArchUnit's definitions)")
	_, _ = fmt.Fprintln(w, "  package\tCa\tCe\tI\tA\tD\t")
	for _, c := range cs {
		_, _ = fmt.Fprintf(w, "  %s\t%d\t%d\t%.3f\t%.3f\t%.3f\t\n",
			c.Package, c.Ca, c.Ce, c.Instability, c.Abstractness, c.NormalizedDistance)
	}
}

func writeCycles(w io.Writer, sets []fitness.CycleSet) {
	for _, set := range sets {
		more := ""
		if set.Truncated {
			more = ", truncated"
		}
		_, _ = fmt.Fprintf(w, "\nCYCLES (%s: %d%s)\n", set.Level, len(set.Cycles), more)
		for _, c := range set.Cycles {
			_, _ = fmt.Fprintf(w, "  %s → %s\n", strings.Join(c, " → "), c[0])
		}
	}
}
