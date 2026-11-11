package proposals

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// reportConfig holds the configuration for proposals report.
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
		Usage:     "anarch-lint [FLAGS] proposals report FILE [PACKAGES]",
		ShortHelp: "say what each proposed rule would do on the code",
		LongHelp: `Report loads the packages (default ./...) and reports each proposal of FILE:
declared, when the model already has a rule of its name, or its outcome on
the code as it is, with its violations. It writes nothing and exits 0 once
the packages load.`,
		Flags: cfg.flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.command)
}

func (cfg *reportConfig) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return &root.UsageError{Err: errors.New("proposals report: a proposals file is required")}
	}
	props, err := readProposals(args[0])
	if err != nil {
		return fmt.Errorf("proposals report: %w", err)
	}
	l, err := cfg.Load(ctx, args[1:], pipeline.AllFacts)
	if err != nil {
		return fmt.Errorf("proposals report: %w", err)
	}
	r, err := fitness.AnalyzeProposals(&l, &props)
	if err != nil {
		return fmt.Errorf("proposals report: %w", err)
	}
	if err := cfg.Write(
		&r,
		func(w io.Writer) error { return text.WriteProposals(w, &r) },
	); err != nil {
		return fmt.Errorf("proposals report: %w", err)
	}
	return nil
}
