// Package format implements the "fmt" CLI command.
package format

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

// Config holds the configuration for the fmt command.
type Config struct {
	*root.Config
	// Check compares the model file with its formatting instead of writing
	// it.
	Check   bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the fmt command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("fmt").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Check, 0, "check", "exit 3 if the model file is not formatted")
	cfg.Command = &ff.Command{
		Name:      "fmt",
		Usage:     "anarch-lint [FLAGS] fmt [--check]",
		ShortHelp: "rewrite the model file in its canonical form",
		LongHelp: `Fmt rewrites the JSON model file (default anarch.json in --dir) in the form
the model hash reads: indented by two spaces, with every set of names and
patterns sorted and without duplicates. The layers, the subsystem order, the
roles, the threshold overrides, import paths and every list of rules keep
their order, so a rule keeps its index in names such as "isolations[2]".
The model hashes the same before and after, so the baseline still applies.

With --check it writes nothing and exits 3, like a failed gate, when the
file differs from its formatting. A YAML model is refused: formatting it
would lose its comments.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) > 0 {
		return &root.UsageError{Err: fmt.Errorf("fmt: takes no arguments, got %q", args)}
	}
	path := cfg.ModelFile()
	if ext := filepath.Ext(path); ext != ".json" {
		return fmt.Errorf("fmt: %w", &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf(
			"%s: only a JSON model is formatted; formatting YAML would lose its comments", path)})
	}
	src, err := pipeline.ReadFile(path)
	if err != nil {
		return fmt.Errorf("fmt: %w", err)
	}
	m, err := pipeline.DecodeModelFile(path, src)
	if err != nil {
		return fmt.Errorf("fmt: %w", err)
	}
	out := m.Format()
	switch {
	case bytes.Equal(src, out):
		_, _ = fmt.Fprintf(cfg.Stdout, "fmt: %s is formatted\n", path)
		return nil
	case cfg.Check:
		_, _ = fmt.Fprintf(cfg.Stderr, "fmt: %s is not formatted; run anarch-lint fmt\n", path)
		return root.ErrGateFailed
	}
	// renameio writes a temporary file and renames it into place, so a
	// failed write leaves the old file whole; on Windows it writes in place.
	if err := maybe.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("fmt: %w",
			&arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("write %s: %v", path, err)})
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "fmt: wrote %s\n", path)
	return nil
}
