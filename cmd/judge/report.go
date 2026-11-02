package judge

import (
	"context"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// reportConfig holds the configuration for judge report.
type reportConfig struct {
	*Config
	flags   *ff.FlagSet
	command *ff.Command
}

func newReport(parent *Config) {
	cfg := reportConfig{Config: parent}
	cfg.flags = ff.NewFlagSet("report").SetParent(parent.Flags)
	cfg.command = &ff.Command{
		Name:      "report",
		Usage:     "anarch-lint --judgements FILE judge report",
		ShortHelp: "show the canaries, gold-set agreement, tallies and rejections",
		Flags:     cfg.flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.command)
}

func (cfg *reportConfig) exec(_ context.Context, _ []string) error {
	r, _, err := cfg.admit(false)
	if err != nil {
		return fmt.Errorf("judge report: %w", err)
	}
	agreements := r.Agreements(statistics())
	if err := cfg.Write(agreements, func(w io.Writer) error {
		return text.WriteJudging(w, &r, agreements)
	}); err != nil {
		return fmt.Errorf("judge report: %w", err)
	}
	return nil
}
