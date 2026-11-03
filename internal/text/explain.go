package text

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// WriteExplanation writes which rules examined one import and what each
// decided, or that the import does not exist or no rule examines it.
func WriteExplanation(w io.Writer, e *arch.ImportExplanation) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	switch {
	case e.Location == nil && len(e.Rules) == 0:
		_, _ = fmt.Fprintf(tw, "%s does not import %s; no declared rule would examine it\n",
			e.From, e.To)
	case e.Location == nil:
		_, _ = fmt.Fprintf(tw, "%s does not import %s; if it did:\n", e.From, e.To)
		writeRules(tw, e.Rules)
	case len(e.Rules) == 0:
		_, _ = fmt.Fprintf(tw, "%s imports %s at %s:%d; no declared rule examines it\n",
			e.From, e.To, e.Location.File, e.Location.Start)
	default:
		inactive := ""
		if e.Inactive {
			inactive = ", in a file the build excludes"
		}
		_, _ = fmt.Fprintf(tw, "%s imports %s at %s:%d%s\n",
			e.From, e.To, e.Location.File, e.Location.Start, inactive)
		writeRules(tw, e.Rules)
	}
	if err := tw.Flush(); err != nil {
		return &arch.Error{Op: "text.WriteExplanation", Err: err}
	}
	return nil
}

// writeRules writes one line per rule's decision.
func writeRules(w io.Writer, rules []arch.RuleExplanation) {
	for _, r := range rules {
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", r.Outcome, r.ModelRule, r.Why)
	}
}
