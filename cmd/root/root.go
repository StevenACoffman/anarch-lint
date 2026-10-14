// Package root defines the root configuration for the CLI.
package root

import (
	"errors"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
)

// ErrUsage matches any usage error through [errors.Is]: a missing or
// malformed argument, an unknown flag or subcommand, conflicting flags. The
// dispatcher prints the selected command's help only for errors that match
// ErrUsage (and for --help), so a runtime failure is reported without a
// screenful of help above it, and main exits 2 for them where other failures
// exit 1. For the details, recover [*UsageError] with [errors.AsType].
var ErrUsage = errors.New("invalid usage")

// ExitError is returned by commands that want a specific non-zero exit code
// without printing an additional error message. run() in main.go checks for
// ExitError with errors.As and calls os.Exit(int(e)) directly, bypassing the
// default "error: ..." printer.
type ExitError int

// UsageError reports that the command line, not the work it asked for, was
// wrong. Err says what was wrong, e.g. "add: command name required"; its text
// is the whole message. A command's exec returns one as
//
//	return &UsageError{Err: fmt.Errorf("add: --name: %w", err)}
//
// building Err with errors.New or fmt.Errorf, whose %w keeps a cause
// inspectable.
type UsageError struct {
	Err error
}

// Config holds shared I/O writers, the analysis flags every measuring command
// shares, and the root ff.Command. All subcommand configs embed *Config to
// inherit these.
type Config struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Flags   *ff.FlagSet
	Command *ff.Command

	// Dir is the directory to analyze, and the root of every reported path.
	Dir string
	// ModelPath is the target-architecture model; empty means anarch.json in
	// Dir if it exists, and an empty model if it does not.
	ModelPath string
	// JudgementsPath optionally supplies verdicts for the judgement slots.
	JudgementsPath string
	// CodeownersPath names the CODEOWNERS file for Balanced Coupling's
	// owner boundary; empty means the first of .github/CODEOWNERS,
	// CODEOWNERS and docs/CODEOWNERS in Dir, if any.
	CodeownersPath string
	// Tags are build tags.
	Tags []string
	// Format is "text" or "json".
	Format string
}

func (e ExitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

func (e *UsageError) Error() string {
	if e.Err == nil {
		return ErrUsage.Error()
	}
	return e.Err.Error()
}

// Unwrap returns the cause, so errors.Is and errors.As see through to it.
func (e *UsageError) Unwrap() error { return e.Err }

// Is reports whether target is [ErrUsage].
func (e *UsageError) Is(target error) bool { return target == ErrUsage }

// New returns a new root Config with the given I/O writers.
func New(stdin io.Reader, stdout, stderr io.Writer) *Config {
	var cfg Config
	cfg.Stdin = stdin
	cfg.Stdout = stdout
	cfg.Stderr = stderr
	// The analysis flags are shared: every subcommand calls
	// SetParent(parent.Flags), so analyze, baseline and check all accept them.
	cfg.Flags = ff.NewFlagSet("anarch-lint")
	cfg.Flags.StringVar(
		&cfg.Dir,
		'C',
		"dir",
		".",
		"directory to analyze; reported paths are relative to it",
	)
	cfg.Flags.StringVar(&cfg.ModelPath, 0, "model", "",
		"target-architecture model (default: anarch.json in --dir, if present)")
	cfg.Flags.StringVar(&cfg.JudgementsPath, 0, "judgements", "",
		"verdicts for the judgement slots, from a human or an LLM reviewer")
	cfg.Flags.StringVar(&cfg.CodeownersPath, 0, "codeowners", "",
		"CODEOWNERS file for the owner boundary, matched relative to --dir "+
			"(default: .github/CODEOWNERS, CODEOWNERS or docs/CODEOWNERS in --dir)")
	cfg.Flags.StringListVar(&cfg.Tags, 0, "tags", "build tag to compile with (repeatable)")
	cfg.Flags.StringEnumVar(&cfg.Format, 0, "format",
		"output format; sarif is for check", "text", "json", "sarif")
	cfg.Command = &ff.Command{
		Name:      "anarch-lint",
		Usage:     "anarch-lint [FLAGS] <SUBCOMMAND> ...",
		Flags:     cfg.Flags,
		ShortHelp: "measure and gate the architecture of Go code",
		LongHelp: `anarch-lint measures the architecture of a Go module deterministically, following
Software Architecture Metrics: the ordering (cycle) measures behind Lilienthal's
Modularity Maturity Index, von Zitzewitz's coupling metrics, size and complexity,
and architecture rules declared in a model file (anarch.json).

The same inputs always produce byte-identical output. Criteria that need a
human or LLM judgement are reported as judgement slots, never as scores, and
never gate.

  analyze   report every measure, finding and judgement slot
  baseline  record the current findings and metrics
  check     fail when the architecture degrades against the baseline`,
	}
	return &cfg
}
