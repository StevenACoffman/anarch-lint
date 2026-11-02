// Package fitness implements the "fitness" CLI command.
package fitness

import (
	"context"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// Config holds the configuration for the fitness command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the fitness command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("fitness").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "fitness",
		Usage:     "anarch-lint [FLAGS] fitness [PACKAGES]",
		ShortHelp: "report ArchUnit-style fitness: rules, metrics and cycles",
		LongHelp: `Fitness loads the packages (default ./...) and reports, deterministically, what
ArchUnit tests in Java:

  - every fitness rule of the model's "fitness" section, with its reason,
    gate, matches, violations and status: layer access, strict layers, an
    onion architecture, slices, dependents, calls and interface-only use;
  - a rule whose selectors match nothing, which fails unless it is optional,
    and the forbidden imports, isolations and roles that match nothing;
  - Lakos's metrics with NCCD, Dowalil's visibility metrics, and ArchUnit's
    component dependency metrics;
  - the elementary cycles of the type and subsystem graphs, in a fixed order.

A rule's gate makes its violations errors (fail, the default) or warnings
(warn), or turns it off. It always exits 0 once the packages load; use check
to gate.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	l, err := cfg.Load(ctx, args, pipeline.AllFacts)
	if err != nil {
		return fmt.Errorf("fitness: %w", err)
	}
	r := fitness.Analyze(&l)
	if err := cfg.Write(
		&r,
		func(w io.Writer) error { return text.WriteFitness(w, &r) },
	); err != nil {
		return fmt.Errorf("fitness: %w", err)
	}
	return nil
}
