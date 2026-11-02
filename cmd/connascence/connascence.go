// Package connascence implements the "connascence" CLI command.
package connascence

import (
	"context"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// Config holds the configuration for the connascence command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the connascence command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("connascence").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "connascence",
		Usage:     "anarch-lint [FLAGS] connascence [PACKAGES]",
		ShortHelp: "report connascence: its instances, candidates, findings and judgement slots",
		LongHelp: `Connascence loads the packages (default ./...) and reports, deterministically,
how they depend on each other (connascence.md):

  - every instance of the nine kinds, name to identity, and of contranascence,
    with its strength, degree and locality;
  - the facts, which can gate, apart from the candidates, which may be
    coincidence and gate only once a reviewer confirms them;
  - the findings: facts over the model's maxStrength and maxDegree limits;
  - a judgement slot for each candidate a limit would gate.

Findings are errors when the model has a "connascence" section, and warnings
otherwise. It always exits 0 once the packages load; use check to gate.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	l, err := cfg.Load(ctx, args, pipeline.AllFacts)
	if err != nil {
		return fmt.Errorf("connascence: %w", err)
	}
	r := connascence.Analyze(&l)
	if err := cfg.Write(
		&r,
		func(w io.Writer) error { return text.WriteConnascence(w, &r) },
	); err != nil {
		return fmt.Errorf("connascence: %w", err)
	}
	return nil
}
