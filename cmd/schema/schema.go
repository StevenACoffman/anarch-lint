// Package schema implements the "schema" CLI command.
package schema

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	modelschema "github.com/StevenACoffman/anarch-lint/internal/schema"
)

// Config holds the configuration for the schema command.
type Config struct {
	*root.Config
	// Proposals prints the schema of a proposals file instead of the
	// model's.
	Proposals bool
	Flags     *ff.FlagSet
	Command   *ff.Command
}

// New creates and registers the schema command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("schema").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Proposals, 0, "proposals",
		"print the schema of a proposals file (see proposals) instead")
	cfg.Command = &ff.Command{
		Name:      "schema",
		Usage:     "anarch-lint schema [--proposals]",
		ShortHelp: "print the JSON Schema of anarch.json",
		LongHelp: `Schema prints the JSON Schema (draft 2020-12) of the model file, generated
from the types that define it, so it never drifts from what anarch-lint
reads. Point an editor at it for completion, or name it in the model:

  {"$schema": "` + modelschema.ID + `", "version": 1, ...}

With --proposals it prints the schema of a proposals file instead, which a
tool proposing rules writes and validates against.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	s := modelschema.Generate()
	if cfg.Proposals {
		s = modelschema.GenerateProposals()
	}
	if err := arch.WriteJSON(cfg.Stdout, s); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	return nil
}
