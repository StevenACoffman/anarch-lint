// Package explain implements the "explain" CLI command.
package explain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// Config holds the configuration for the explain command.
type Config struct {
	*root.Config
	// WaiversPath names the waivers file; empty means anarch-waivers.json in
	// --dir, if it exists.
	WaiversPath string
	// Today is the date waivers expire against, YYYY-MM-DD; empty means the
	// current date.
	Today   string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the explain command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("explain").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.WaiversPath, 0, "waivers", "",
		"waivers that accept findings (default: "+pipeline.DefaultWaivers+" in --dir, if present)")
	cfg.Flags.StringVar(&cfg.Today, 0, "today", "",
		"the date waivers expire against, YYYY-MM-DD (default: today)")
	cfg.Command = &ff.Command{
		Name:      "explain",
		Usage:     "anarch-lint [FLAGS] explain IMPORTER IMPORTED [PACKAGES]",
		ShortHelp: "say which rules examine one import, and why it passes or fails",
		LongHelp: `Explain loads the packages (default ./...) and lists every declared rule that
examines the import of IMPORTED by IMPORTER, with its decision:

  forbids  the rule's finding fails the gate unless the baseline accepts it
  waived   the rule forbids it, and an unexpired waiver accepts the finding
  allows   the rule examined the import and allows it, saying why

The structural rules (layers, subsystem order, interfaces, forbidden imports,
isolations) say why they allow an import; a fitness rule is listed when it
forbids the import or allows it by name, as a layer in mayOnlyAccess. The
rules run exactly as check runs them.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return &root.UsageError{Err: errors.New("explain: want IMPORTER and IMPORTED")}
	}
	from, to, patterns := args[0], args[1], args[2:]
	l, err := cfg.Load(ctx, patterns, pipeline.NoFacts)
	if err != nil {
		return fmt.Errorf("explain: %w", err)
	}
	waivers, today, err := cfg.ReadWaivers(cfg.WaiversPath, cfg.Today)
	if err != nil {
		return fmt.Errorf("explain: %w", err)
	}
	e := arch.ExplainImport(&l, from, to)
	if l.Model.Fitness.Enabled() {
		e.Rules = slices.Concat(e.Rules, fitness.ExplainImport(&l, from, to))
		arch.SortExplanations(e.Rules)
	}
	e.Waive(waivers, today)
	if err := cfg.Write(&e, func(w io.Writer) error {
		return text.WriteExplanation(w, &e)
	}); err != nil {
		return fmt.Errorf("explain: %w", err)
	}
	return nil
}
