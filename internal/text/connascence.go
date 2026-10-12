package text

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
)

// maxListedCandidates is how many candidates the text form lists; the JSON
// report has all of them.
const maxListedCandidates = 20

// WriteConnascence writes a connascence report: the summary matrix of kinds
// by locality, the findings, the strongest candidates, the judgement slots,
// and a count of the assumptions.
func WriteConnascence(w io.Writer, r *connascence.Report) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "anarch-lint connascence (%s)\n", connascence.Schema)
	_, _ = fmt.Fprintf(tw, "patterns %s · model %s · %s\n",
		strings.Join(r.Inputs.Patterns, " "), short(r.Inputs.ModelHash), r.Inputs.GoVersion)
	writeSummary(tw, r.Summary)
	writeFindings(tw, r.Findings)
	writeCandidates(tw, r.Candidates, len(r.Rejected))
	writeSlots(tw, r.Judgements)
	_, _ = fmt.Fprintf(tw, "\nASSUMPTIONS: %d, listed in the JSON report (--format=json)\n",
		len(r.Assumptions))
	if err := tw.Flush(); err != nil {
		return &arch.Error{Op: "text.WriteConnascence", Err: err}
	}
	return nil
}

// writeSummary writes each kind's facts and candidates at each locality, as
// "facts+candidates".
func writeSummary(w io.Writer, counts []arch.ConnascenceCount) {
	_, _ = fmt.Fprintln(w, "\nSUMMARY (facts+candidates at each locality)")
	header := []string{"  kind"}
	for l := arch.LocalityFunction; l <= arch.LocalityExternal; l++ {
		header = append(header, l.String())
	}
	_, _ = fmt.Fprintln(w, strings.Join(header, "\t")+"\t")
	cells := map[[2]int]arch.ConnascenceCount{}
	for _, c := range counts {
		cells[[2]int{int(c.Kind), int(c.Locality)}] = c
	}
	for k := arch.StrengthContranascence; k <= arch.StrengthIdentity; k++ {
		if row, ok := summaryRow(k, cells); ok {
			_, _ = fmt.Fprintln(w, row)
		}
	}
}

// summaryRow returns the row of kind k, and false when k has no instance.
func summaryRow(k arch.Strength, cells map[[2]int]arch.ConnascenceCount) (string, bool) {
	row := []string{fmt.Sprintf("  %d %s", int(k), k)}
	seen := false
	for l := arch.LocalityFunction; l <= arch.LocalityExternal; l++ {
		cell := "-"
		if c, ok := cells[[2]int{int(k), int(l)}]; ok {
			cell, seen = fmt.Sprintf("%d+%d", c.Facts, c.Candidates), true
		}
		row = append(row, cell)
	}
	return strings.Join(row, "\t") + "\t", seen
}

// writeCandidates writes the strongest candidates, already ranked.
func writeCandidates(w io.Writer, cs []arch.Connascence, rejected int) {
	_, _ = fmt.Fprintf(
		w,
		"\nCANDIDATES (%d, %d rejected; the first %d, strongest and farthest first)\n",
		len(cs),
		rejected,
		min(len(cs), maxListedCandidates),
	)
	for i := range cs[:min(len(cs), maxListedCandidates)] {
		c := &cs[i]
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", c.Detector, c.Subject, siteWhere(c.Sites[0]))
		_, _ = fmt.Fprintf(w, "  \t%s\n", c.Message)
		_, _ = fmt.Fprintf(
			w,
			"  \tstrength %d (%s), locality %s, degree %d, %d sites, confidence %s\n",
			c.Strength,
			c.Kind,
			c.Locality,
			c.Degree,
			c.SiteCount,
			c.Confidence,
		)
		_, _ = fmt.Fprintf(w, "  \tfix: %s\n", c.Fix)
	}
}

// writeSlots writes the judgement slots of a measure whose approved verdicts
// confirm or reject candidates.
func writeSlots(w io.Writer, slots []arch.JudgementSlot) {
	_, _ = fmt.Fprintln(
		w,
		"\nJUDGEMENTS (for a human expert or an adversarial LLM; an approved verdict confirms or rejects)",
	)
	for i := range slots {
		s := &slots[i]
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", s.Criterion, s.Status, s.Question)
		_, _ = fmt.Fprintf(w, "  \t\tanswers: %s\n", strings.Join(s.Answers, ", "))
		for _, e := range s.Evidence {
			_, _ = fmt.Fprintf(w, "  \t\tevidence: %s\n", e)
		}
		_, _ = fmt.Fprintf(w, "  \t\tevidence hash: %s\n", s.EvidenceHash)
	}
}

// siteWhere is a site's file and line.
func siteWhere(s arch.Site) string { return fmt.Sprintf("%s:%d", s.File, s.Line) }
