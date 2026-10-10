package arch

// LineRange is a range of lines in one file, both ends inclusive.
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// ChangedLines maps a file path, relative to the analysis root, to the line
// ranges a change added or modified there, sorted and non-overlapping. A nil
// ChangedLines means "no change set": every line counts.
type ChangedLines map[string][]LineRange

// Touches reports whether a span intersects a changed line. A finding with no
// location, such as one about the model, touches nothing: it belongs to the
// whole system, not to a change.
//
// Ranges must be sorted, as ChangedLines documents.
func (c ChangedLines) Touches(s *Span) bool {
	if s == nil {
		return false
	}
	for _, r := range c[s.File] {
		if r.Start > s.End {
			return false
		}
		if r.End >= s.Start {
			return true
		}
	}
	return false
}
