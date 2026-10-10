// Package analyze implements the "analyze" CLI command.
package analyze

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
)

// Config holds the configuration for the analyze command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the analyze command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("analyze").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "analyze",
		Usage:     "anarch-lint [FLAGS] analyze [PACKAGES]",
		ShortHelp: "report every measure, finding and judgement slot",
		LongHelp: `Analyze loads the packages (default ./...) and reports, deterministically:

  - the Modularity Maturity Index as an interval, with all 22 criteria;
  - the thirteen ordering measures of Lilienthal's dissertation;
  - von Zitzewitz's coupling metrics for types, packages and subsystems;
  - every architecture finding, each with a fix;
  - the judgement slots a human expert or an LLM reviewer can fill.

It always exits 0 once the packages load; use check to gate.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	report, err := cfg.Analyze(ctx, args)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	if err := cfg.WriteReport(&report); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	return nil
}
