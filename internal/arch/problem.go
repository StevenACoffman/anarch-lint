package arch

import "fmt"

// The problems a model can have.
const (
	ProblemMalformed  ProblemCode = "malformed"           // not JSON, or an unknown field
	ProblemVersion    ProblemCode = "unsupported-version" // a version other than 1
	ProblemDuplicate  ProblemCode = "duplicate-name"      // a name declared twice
	ProblemReference  ProblemCode = "unknown-reference"   // a name or capture that is not declared
	ProblemRange      ProblemCode = "out-of-range"        // a number outside its bounds
	ProblemValue      ProblemCode = "unknown-value"       // a value outside a closed set
	ProblemIncomplete ProblemCode = "incomplete-rule"     // a rule without a selector it needs
	ProblemPattern    ProblemCode = "bad-pattern"         // a pattern or glob that cannot work
)

// ProblemCode names what is wrong with a model, stably, so that an editor or
// CI can link a problem to its explanation in README's "Model Problems".
type ProblemCode string

// invalidFunc reports a model problem as an EINVALID error whose message is
// "model: CODE: text".
type invalidFunc func(code ProblemCode, format string, args ...any) error

// ruleProblem is what makes a rule unusable, with its code; the zero value
// means the rule is usable.
type ruleProblem struct {
	code ProblemCode
	text string
}

// InvalidModel reports a model problem as an EINVALID error, "model: CODE:
// text": the invalidFunc of every model check, and of a reader that finds a
// problem before decoding, such as YAML that does not parse.
func InvalidModel(code ProblemCode, format string, args ...any) error {
	return &Error{
		Code:    EINVALID,
		Message: "model: " + string(code) + ": " + fmt.Sprintf(format, args...),
	}
}

// incomplete is the problem of a rule without a selector it needs.
func incomplete(format string, args ...any) ruleProblem {
	return ruleProblem{ProblemIncomplete, fmt.Sprintf(format, args...)}
}
