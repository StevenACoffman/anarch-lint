package text

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// WriteValidation writes a validation: the history read, then each
// metric's correlation with the fixes and its verdict.
func WriteValidation(w io.Writer, v *arch.Validation) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "anarch-lint validate (%s)\n", v.Schema)
	_, _ = fmt.Fprintf(tw, "metrics at %s · %d commits after it, to %s · %d fixes · %d packages\n",
		short(v.History.Since), v.History.Commits, short(v.History.Until),
		v.History.FixCommits, v.Packages)
	_, _ = fmt.Fprintln(tw, "\nMETRICS (Spearman with fixes, lines of code held constant)")
	_, _ = fmt.Fprintln(tw, "  metric\trho\tp\tverdict\t")
	for _, m := range v.Metrics {
		_, _ = fmt.Fprintf(tw, "  %s\t%.3f\t%.3f\t%s\t\n", m.Metric, m.Rho, m.P, m.Verdict)
	}
	_, _ = fmt.Fprintf(tw, "\nASSUMPTIONS: %d, listed in the JSON report (--format=json)\n",
		len(v.Assumptions))
	if err := tw.Flush(); err != nil {
		return &arch.Error{Op: "text.WriteValidation", Err: err}
	}
	return nil
}
