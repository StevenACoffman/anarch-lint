// Package baseline implements the "baseline" CLI command.
package baseline

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/renameio/v2/maybe"
	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
)

// Config holds the configuration for the baseline command.
type Config struct {
	*root.Config
	// Out is the baseline file to write; empty means anarch-baseline.json in
	// --dir.
	Out string
	// Shrink only removes what was fixed from the existing baseline, and
	// never accepts anything new.
	Shrink  bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the baseline command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("baseline").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"baseline file to write (default: "+pipeline.DefaultBaseline+" in --dir)")
	cfg.Flags.BoolVar(&cfg.Shrink, 0, "shrink",
		"only remove fixed findings and keep the better value of each metric; accept nothing new")
	cfg.Command = &ff.Command{
		Name:      "baseline",
		Usage:     "anarch-lint [FLAGS] baseline [--out FILE] [--shrink] [PACKAGES]",
		ShortHelp: "record the current findings and metrics",
		LongHelp: `Baseline measures the packages and records their findings and system metrics,
the state check compares against. Commit the file next to the model: the gate
then fails only on what gets worse, so an existing system can adopt it
without first fixing every finding.

With --shrink, it rewrites the existing baseline with only what got better:
fixed findings leave it, and each metric keeps the better of its two values.
Nothing new is ever accepted that way, so a scheduled job can run it safely;
check says when the baseline can shrink.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	in, err := cfg.GateInput(ctx, args, nil)
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	out := cfg.Out
	if out == "" {
		out = filepath.Join(cfg.Dir, pipeline.DefaultBaseline)
	}
	b := arch.NewBaseline(&in)
	if cfg.Shrink {
		old, err := cfg.ReadBaseline(out)
		if err != nil {
			return fmt.Errorf("baseline: %w", err)
		}
		b = arch.Shrink(&old, &in)
	}
	if err := writeBaseline(out, &b); err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "baseline: wrote %d findings and %d metrics to %s\n",
		len(b.Findings), len(b.Metrics), out)
	return nil
}

// writeBaseline writes b to path as JSON. It encodes the whole baseline
// before it writes, and renameio renames a temporary file into place, so a
// failure leaves the old baseline whole, which --shrink has just read; on
// Windows renameio writes in place.
func writeBaseline(path string, b *arch.Baseline) error {
	const op = "baseline.writeBaseline"
	var buf bytes.Buffer
	if err := arch.WriteJSON(&buf, b); err != nil {
		return &arch.Error{Op: op, Err: err}
	}
	if err := maybe.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("write %s: %v", path, err)}
	}
	return nil
}
