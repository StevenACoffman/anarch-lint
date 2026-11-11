// Package validate implements the "validate" CLI command.
package validate

import (
	"context"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/git"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// Config holds the configuration for the validate command.
type Config struct {
	*root.Config
	// Rev is the revision whose metrics are tested; Commits is how many
	// first-parent commits after it are read for fixes.
	Rev     string
	Commits int
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the validate command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("validate").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Rev, 0, "rev", "", "the git revision whose package metrics are tested")
	cfg.Flags.IntVar(&cfg.Commits, 0, "commits", 200,
		"how many first-parent commits after --rev to read for fixes")
	cfg.Command = &ff.Command{
		Name:      "validate",
		Usage:     "anarch-lint [FLAGS] validate --rev REV [--commits N] [PACKAGES]",
		ShortHelp: "test whether the package metrics at a revision predicted the fixes after it",
		LongHelp: `Validate checks out --rev in a temporary git worktree, measures each package
there (Martin's Ca, Ce, instability, abstractness and distance, and summed
cyclomatic and cognitive complexity), and reads the first --commits
first-parent commits after it. A commit whose subject says fix, bug,
regression or revert is a fix. For each metric it reports the Spearman
correlation of its ranking of the packages with their fixes, holding lines
of code constant, and a verdict: predictive, not-predictive or
insufficient-data (IMPROVEMENTS.md §10.5).

It is a report, never a gate: a metric that predicts nothing in one
history may in another. The model is read as it is now. It always exits 0
once the code loads.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if cfg.Rev == "" || cfg.Commits <= 0 {
		return fmt.Errorf("validate: %w", &arch.Error{
			Code: arch.EINVALID, Message: "--rev is required, and --commits must be positive",
		})
	}
	v, err := cfg.Validate(ctx, git.After{Rev: cfg.Rev, Commits: cfg.Commits}, args)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	if err := cfg.Write(
		&v,
		func(w io.Writer) error { return text.WriteValidation(w, &v) },
	); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return nil
}
