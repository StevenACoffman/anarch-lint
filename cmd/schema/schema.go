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
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the schema command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("schema").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "schema",
		Usage:     "anarch-lint schema",
		ShortHelp: "print the JSON Schema of anarch.json",
		LongHelp: `Schema prints the JSON Schema (draft 2020-12) of the model file, generated
from the types that define it, so it never drifts from what anarch-lint
reads. Point an editor at it for completion, or name it in the model:

  {"$schema": "` + modelschema.ID + `", "version": 1, ...}`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	if err := arch.WriteJSON(cfg.Stdout, modelschema.Generate()); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	return nil
}
