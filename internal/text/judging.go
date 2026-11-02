package text

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/judging"
)

// WriteJudging writes what a judging run admitted: the canaries, the gold
// set's agreement, the tallies, the rejected replies and the notes.
func WriteJudging(w io.Writer, r *judging.Result, agreements []judging.Agreement) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "JUDGING (%d tallies, %d rejected replies, %d missing)\n",
		len(r.Tallies), len(r.Rejected), len(r.Missing))
	writeCanaries(tw, r.Canaries)
	writeAgreements(tw, agreements)
	writeTallies(tw, r.Tallies)
	if len(r.Rejected) > 0 {
		_, _ = fmt.Fprintln(tw, "\nREJECTED REPLIES (each counts as no answer)")
		for _, rej := range r.Rejected {
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", rej.Key, rej.ID, rej.Config,
				strings.Join(rej.Defects, "; "))
		}
	}
	for _, n := range r.Notes {
		_, _ = fmt.Fprintf(tw, "\nnote: %s\n", n)
	}
	if err := tw.Flush(); err != nil {
		return &arch.Error{Op: "text.WriteJudging", Err: err}
	}
	return nil
}

// writeCanaries writes whether each configuration passed each family's
// canaries.
func writeCanaries(w io.Writer, canaries []judging.CanaryResult) {
	if len(canaries) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "\nCANARIES")
	for _, c := range canaries {
		status := "passed"
		if !c.Passed {
			status = "missed " + strings.Join(
				c.Misses,
				", ",
			) + "; its answers in the family are refused"
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", c.Family, c.Config, status)
	}
}

// writeAgreements writes the gold set's agreement and calibration.
func writeAgreements(w io.Writer, agreements []judging.Agreement) {
	if len(agreements) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "\nGOLD SET (reported, never gated)")
	for _, a := range agreements {
		half := "tuning"
		if a.HeldOut {
			half = "held out"
		}
		_, _ = fmt.Fprintf(
			w,
			"  %s\t%s\t%s\t%d/%d agree, 95%% interval %.2f–%.2f\tECE %.3f, Brier %.3f\n",
			a.Family,
			a.Config,
			half,
			a.Correct,
			a.Total,
			a.Low,
			a.High,
			a.ECE,
			a.Brier,
		)
	}
}

// writeTallies writes what each slot's answers came to.
func writeTallies(w io.Writer, tallies []judging.Tally) {
	if len(tallies) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "\nTALLIES (a judge's verdict applies once a human attests it)")
	for _, t := range tallies {
		var counts []string
		for _, answer := range slices.Sorted(maps.Keys(t.Counts)) {
			counts = append(counts, fmt.Sprintf("%s %d", answer, t.Counts[answer]))
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\tby %s\n", t.Criterion, t.Outcome,
			strings.Join(counts, ", "), strings.Join(t.Configs, ", "))
	}
}
