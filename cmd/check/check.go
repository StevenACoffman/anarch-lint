// Package check implements the "check" CLI command.
package check

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/git"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// defaultWaivers is the waivers file looked for in --dir when --waivers is
// unset.
const defaultWaivers = "anarch-waivers.json"

// Config holds the configuration for the check command.
type Config struct {
	*root.Config
	// BaselinePath is the baseline to compare with; empty means
	// anarch-baseline.json in --dir.
	BaselinePath string
	// NewFromRev, when set, limits the finding gate to lines changed since
	// that revision.
	NewFromRev string
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
		"baseline to compare with (default: "+root.DefaultBaseline+" in --dir)")
	cfg.Flags.StringVar(&cfg.NewFromRev, 0, "new-from-rev", "",
		"count only findings on lines changed since this git revision")
	cfg.Flags.StringVar(
		&cfg.WaiversPath,
		0,
		"waivers",
		"",
		"waivers that accept findings until they expire (default: "+defaultWaivers+" in --dir, if present)",
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
the granular gate for one change; the system metrics gate either way.

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
	report, err := cfg.GateInput(ctx, args)
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	baseline, err := cfg.ReadBaseline(cfg.BaselinePath)
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	var changed arch.ChangedLines
	if cfg.NewFromRev != "" {
		if changed, err = git.ChangedLines(ctx, cfg.Dir, cfg.NewFromRev); err != nil {
			return fmt.Errorf("check: %w", err)
		}
	}
	opts, err := cfg.gateOptions(changed)
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	result := arch.Gate(&report, &baseline, opts)
	if cfg.Format == "json" {
		err = arch.WriteJSON(cfg.Stdout, result)
	} else {
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

// gateOptions reads the waivers, if any, and fixes today's date: the one
// place the clock is read, so the domain stays deterministic.
func (cfg *Config) gateOptions(changed arch.ChangedLines) (*arch.GateOptions, error) {
	const op = "check.gateOptions"
	path, required := cfg.WaiversPath, true
	if path == "" {
		path, required = filepath.Join(cfg.Dir, defaultWaivers), false
	}
	opts := &arch.GateOptions{Changed: changed}
	src, err := root.ReadFile(path)
	switch {
	case !required && arch.ErrorCode(err) == arch.ENOTFOUND:
	case err != nil:
		return nil, &arch.Error{Op: op, Err: err}
	default:
		if opts.Waivers, err = arch.DecodeWaivers(bytes.NewReader(src)); err != nil {
			return nil, &arch.Error{Op: op + " " + path, Err: err}
		}
	}
	today := cfg.Today
	if today == "" {
		today = time.Now().Format(time.DateOnly)
	}
	if opts.Today, err = arch.ParseDate(today); err != nil {
		return nil, &arch.Error{Op: op + " --today", Err: err}
	}
	return opts, nil
}
