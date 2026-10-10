// Package baseline implements the "baseline" CLI command.
package baseline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Config holds the configuration for the baseline command.
type Config struct {
	*root.Config
	// Out is the baseline file to write; empty means anarch-baseline.json in
	// --dir.
	Out     string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the baseline command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("baseline").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"baseline file to write (default: "+root.DefaultBaseline+" in --dir)")
	cfg.Command = &ff.Command{
		Name:      "baseline",
		Usage:     "anarch-lint [FLAGS] baseline [--out FILE] [PACKAGES]",
		ShortHelp: "record the current findings and metrics",
		LongHelp: `Baseline measures the packages and records their findings and system metrics,
the state check compares against. Commit the file next to the model: the gate
then fails only on what gets worse, so an existing system can adopt it
without first fixing every finding.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	report, err := cfg.Analyze(ctx, args)
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	out := cfg.Out
	if out == "" {
		out = filepath.Join(cfg.Dir, root.DefaultBaseline)
	}
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	b := arch.NewBaseline(&report)
	if err := arch.WriteJSON(f, b); err != nil {
		_ = f.Close()
		return fmt.Errorf("baseline: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "baseline: wrote %d findings and %d metrics to %s\n",
		len(b.Findings), len(b.Metrics), out)
	return nil
}
