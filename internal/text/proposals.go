package text

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
)

// WriteProposals writes what each proposed rule would do: declared by the
// model already, or its outcome on the code, then the violations of the
// undeclared ones.
func WriteProposals(w io.Writer, r *fitness.ProposalsReport) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "anarch-lint proposals (%s)\n", fitness.ProposalsSchema)
	_, _ = fmt.Fprintf(tw, "from %s · patterns %s · model %s · %s\n", orNone(r.Source),
		strings.Join(r.Inputs.Patterns, " "), short(r.Inputs.ModelHash), r.Inputs.GoVersion)
	_, _ = fmt.Fprintln(tw, "\nPROPOSALS")
	_, _ = fmt.Fprintln(tw, "  id\trule\tkind\tmatched\tviolations\tstatus\tevidence\t")
	for _, p := range r.Proposals {
		matched, violations, status := "-", "-", "declared"
		if p.Result != nil {
			matched, violations = strconv.Itoa(p.Result.Matched), strconv.Itoa(p.Result.Violations)
			status = string(p.Result.Status)
		}
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\t%d\t\n",
			p.ID, p.Rule, p.Kind, matched, violations, status, p.Evidence)
	}
	writeFindings(tw, r.Findings)
	if err := tw.Flush(); err != nil {
		return &arch.Error{Op: "text.WriteProposals", Err: err}
	}
	return nil
}
