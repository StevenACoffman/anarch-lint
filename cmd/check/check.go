// Package check implements the "check" CLI command.
package check

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/git"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
	"github.com/StevenACoffman/anarch-lint/internal/sarif"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// Config holds the configuration for the check command.
type Config struct {
	*root.Config
	// BaselinePath is the baseline to compare with; empty means
	// anarch-baseline.json in --dir.
	BaselinePath string
	// NewFromRev, when set, limits the finding gate to lines changed since
	// that revision.
	NewFromRev string
	// BaselineRev, when set, measures the code at that revision and
	// compares with it instead of the baseline file.
	BaselineRev string
	// WaiversPath names the waivers file; empty means anarch-waivers.json in
	// --dir, if it exists.
	WaiversPath string
	// Today is the date waivers expire against, YYYY-MM-DD; empty means the
	// current date.
	Today   string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the check command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("check").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.BaselinePath, 0, "baseline", "",
		"baseline to compare with (default: "+pipeline.DefaultBaseline+" in --dir)")
	cfg.Flags.StringVar(&cfg.NewFromRev, 0, "new-from-rev", "",
		"count only findings on lines changed since this git revision")
	cfg.Flags.StringVar(&cfg.BaselineRev, 0, "baseline-rev", "",
		"measure the code at this git revision, under the current model, as the baseline")
	cfg.Flags.StringVar(
		&cfg.WaiversPath,
		0,
		"waivers",
		"",
		"waivers that accept findings until they expire (default: "+pipeline.DefaultWaivers+" in --dir, if present)",
	)
	cfg.Flags.StringVar(&cfg.Today, 0, "today", "",
		"the date waivers expire against, YYYY-MM-DD (default: today)")
	cfg.Command = &ff.Command{
		Name:      "check",
		Usage:     "anarch-lint [FLAGS] check [--baseline FILE] [--new-from-rev REV] [PACKAGES]",
		ShortHelp: "fail when the architecture degrades against the baseline",
		LongHelp: `Check measures the packages and compares them with the baseline. It fails, with
exit status 3, when an error-severity finding is new, when a system metric is
worse than its baseline value (cycles, SDI, relative cyclicity, the MMI's low
bound, and the connascence and balance metrics below), or when the baseline
was recorded under a different model.

With --new-from-rev, only findings on lines changed since the revision count,
the granular gate for one change; the system metrics gate either way. Such a
change also fails when it changes Go code and loosens the bars it is measured
against: a raised threshold or limit, a lowered floor, a rule removed or its
gate weakened, findings or worse metrics accepted into the baseline, or a
waiver added or extended. Loosen them in a change of their own.

With --baseline-rev, check measures the code at that revision, checked out in
a temporary git worktree, under the current model, and compares with it
instead of the baseline file: what the change adds to the revision, under
today's rules.

When the model has a "connascence" section, the connascence findings and the
degree of each strong instance across subsystems gate too; with "balance":
{"gate": true}, so do the Balanced Coupling findings and the strong references
of each tightly coupled pair.

A waiver accepts one new finding, by fingerprint, until its expiry date, with
a reason and an approver. An expired waiver protects nothing and is reported.
The model's coverage and compliance floors (gates) fail the check whatever the
baseline says. A judgement slot never gates by itself; an approved verdict can
confirm a candidate into a finding, or suppress one.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	var changed arch.ChangedLines
	var err error
	if cfg.NewFromRev != "" {
		if changed, err = git.ChangedLines(ctx, cfg.Dir, cfg.NewFromRev); err != nil {
			return fmt.Errorf("check: %w", err)
		}
	}
	report, err := cfg.GateInput(ctx, args, changed)
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	baseline, err := cfg.baseline(ctx, args)
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	opts, err := cfg.gateOptions(changed)
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	if changesCode(changed) {
		if opts.Loosened, opts.BarsUnchecked, err = cfg.LoosenedSince(ctx, pipeline.BarFiles{
			Rev: cfg.NewFromRev, BaselinePath: cfg.BaselinePath, WaiversPath: cfg.WaiversPath,
		}); err != nil {
			return fmt.Errorf("check: %w", err)
		}
	}
	result := arch.Gate(&report, &baseline, opts)
	switch cfg.Format {
	case "json":
		err = arch.WriteJSON(cfg.Stdout, result)
	case "sarif":
		err = sarif.WriteGate(cfg.Stdout, &result, cfg.SARIFOptions())
	default:
		err = text.WriteGate(cfg.Stdout, &result)
	}
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	if !result.Passed {
		return root.ErrGateFailed
	}
	return nil
}

// baseline reads the baseline file, or measures the code at --baseline-rev.
func (cfg *Config) baseline(ctx context.Context, patterns []string) (arch.Baseline, error) {
	switch {
	case cfg.BaselineRev != "" && cfg.BaselinePath != "":
		return arch.Baseline{}, &root.UsageError{
			Err: errors.New("--baseline and --baseline-rev name two baselines; give one"),
		}
	case cfg.BaselineRev != "":
		b, err := cfg.BaselineAt(ctx, cfg.BaselineRev, patterns)
		if err != nil {
			return arch.Baseline{}, &arch.Error{Op: "check.baseline", Err: err}
		}
		return b, nil
	}
	b, err := cfg.ReadBaseline(cfg.BaselinePath)
	if err != nil {
		return arch.Baseline{}, &arch.Error{Op: "check.baseline", Err: err}
	}
	return b, nil
}

// gateOptions reads the waivers, if any, and fixes today's date.
func (cfg *Config) gateOptions(changed arch.ChangedLines) (*arch.GateOptions, error) {
	waivers, today, err := cfg.ReadWaivers(cfg.WaiversPath, cfg.Today)
	if err != nil {
		return nil, &arch.Error{Op: "check.gateOptions", Err: err}
	}
	return &arch.GateOptions{Changed: changed, Waivers: waivers, Today: today}, nil
}

// changesCode reports a change set that changes Go code the gate measures:
// a .go file that is not a test. A change to the bars alone may loosen them.
func changesCode(changed arch.ChangedLines) bool {
	for file := range changed {
		if strings.HasSuffix(file, ".go") && !strings.HasSuffix(file, "_test.go") {
			return true
		}
	}
	return false
}
