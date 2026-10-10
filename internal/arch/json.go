package arch

import (
	"encoding/json"
	"io"
)

// WriteJSON writes v as indented JSON followed by a newline. It is the one
// place anarch-lint encodes JSON, so every file and report shares a format.
// encoding/json sorts map keys, and every slice in a report is sorted, so the
// same value always yields the same bytes.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return &Error{Op: "arch.WriteJSON", Err: err}
	}
	return nil
}
