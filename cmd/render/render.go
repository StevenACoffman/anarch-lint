// Package render implements the "render" CLI command.
package render

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
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// defaultOut is the file render writes, beside the model.
const defaultOut = "anarch.md"

// Config holds the configuration for the render command.
type Config struct {
	*root.Config
	// Out is the Markdown file to write or check; empty means anarch.md
	// beside the model file.
	Out string
	// Check compares the file with a fresh rendering instead of writing it.
	Check   bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the render command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("render").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"Markdown file to write or check (default: "+defaultOut+" beside the model)")
	cfg.Flags.BoolVar(&cfg.Check, 0, "check", "exit 3 if the file differs from a fresh rendering")
	cfg.Command = &ff.Command{
		Name:      "render",
		Usage:     "anarch-lint [FLAGS] render [--out FILE] [--check] [PACKAGES]",
		ShortHelp: "render the model as Markdown, or check the rendering for drift",
		LongHelp: `Render loads the packages (default ./...) and writes the model as Markdown for
people and coding agents to read: the subsystems and their interfaces and
owners, the layers, a Mermaid flowchart of the subsystem dependencies as
measured, the forbidden imports and isolations, the fitness rules with their
reasons, and the domain models.

The rendering is byte-identical for the same model and code, and every piece
of model text is escaped, so a reason cannot inject HTML. With --check it
writes nothing and exits 3, like a failed gate, when the file is missing or
differs: the model or the code changed and the rendering did not.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	l, err := cfg.Load(ctx, args, pipeline.NoFacts)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	var b bytes.Buffer
	if err := text.WriteArchitecture(&b, &l); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	out := cfg.Out
	if out == "" {
		out = filepath.Join(filepath.Dir(cfg.ModelFile()), defaultOut)
	}
	if cfg.Check {
		return cfg.check(out, b.Bytes())
	}
	// renameio writes a temporary file and renames it into place, so a
	// failed write leaves the old file whole; on Windows it writes in place.
	if err := maybe.WriteFile(out, b.Bytes(), 0o644); err != nil {
		return fmt.Errorf("render: %w",
			&arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("write %s: %v", out, err)})
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "render: wrote %s\n", out)
	return nil
}

// check reports whether the file at out holds the rendering want.
func (cfg *Config) check(out string, want []byte) error {
	got, err := pipeline.ReadFile(out)
	if err != nil && arch.ErrorCode(err) != arch.ENOTFOUND {
		return fmt.Errorf("render: %w", err)
	}
	if bytes.Equal(got, want) {
		_, _ = fmt.Fprintf(cfg.Stdout, "render: %s is current\n", out)
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stderr, "render: %s is out of date; run anarch-lint render\n", out)
	return root.ErrGateFailed
}
