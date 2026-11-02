// Package diagram implements the "diagram" CLI command.
package diagram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
	"github.com/StevenACoffman/anarch-lint/internal/plantuml"
)

// Config holds the configuration for the diagram command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the diagram command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("diagram").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "diagram",
		Usage:     "anarch-lint diagram FILE.puml",
		ShortHelp: "draft a model from a PlantUML component diagram",
		LongHelp: `Diagram reads a PlantUML component diagram and prints the model it stands for,
as ArchUnit's adhereToPlantUmlDiagram reads one: each component whose
stereotypes name package patterns is a subsystem,

  [Orders] <<example.com/shop/orders/...>> as orders
  orders --> [Catalog]

each arrow lets one component import another, an isolation per component
forbids every import the diagram does not draw, and a package in no component
is a finding. Save the output as anarch.json, then add reasons, layers and
anything else the diagram cannot say.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) != 1 {
		return &root.UsageError{Err: errors.New("diagram: want one diagram file")}
	}
	src, err := pipeline.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("diagram: %w", err)
	}
	d, err := plantuml.Parse(bytes.NewReader(src))
	if err != nil {
		return fmt.Errorf("diagram: %s: %w", args[0], err)
	}
	m := d.Model(filepath.Base(args[0]))
	if err := arch.WriteJSON(cfg.Stdout, &m); err != nil {
		return fmt.Errorf("diagram: %w", err)
	}
	return nil
}
