// Package draft implements the "draft" CLI command.
package draft

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
)

// Config holds the configuration for the draft command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the draft command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("draft").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "draft",
		Usage:     "anarch-lint [FLAGS] draft [PACKAGES]",
		ShortHelp: "draft a model from the communities of the import graph",
		LongHelp: `Draft loads the packages (default ./...) and prints a starter model for a
module that has none: each community of the import graph, as analyze
reports them, is a subsystem listing its packages, named after its shortest
package path, and when the subsystems depend on each other without a cycle,
the subsystem order runs from the dependents down.

Communities say how the code is coupled today, not how it should be, and
shift with small edits. Save the output as anarch.json, then rename and
merge the subsystems, and add their interfaces, the layers and the reasons.
Any model already in --dir is ignored.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	l, err := cfg.Load(ctx, args, pipeline.NoFacts)
	if err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	m := arch.StarterModel(&l.Program)
	if _, err := cfg.Stdout.Write(m.Format()); err != nil {
		return fmt.Errorf("draft: %w", &arch.Error{Op: "draft", Err: err})
	}
	return nil
}
