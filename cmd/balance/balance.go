// Package balance implements the "balance" CLI command.
package balance

import (
	"context"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// Config holds the configuration for the balance command.
type Config struct {
	*root.Config
	// SinceRev, when set, bounds the change history read as evidence of
	// volatility: the commits after it, up to HEAD.
	SinceRev string
	// CoverProfile, when set, is a go test -coverprofile file whose coverage
	// ranks the packages many import that tests rarely run.
	CoverProfile string
	Flags        *ff.FlagSet
	Command      *ff.Command
}

// New creates and registers the balance command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("balance").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.SinceRev, 0, "since-rev", "",
		"read the commits after this git revision as evidence of volatility")
	cfg.Flags.StringVar(&cfg.CoverProfile, 0, "coverprofile", "",
		"a go test -coverprofile file: rank packages many import that tests rarely run")
	cfg.Command = &ff.Command{
		Name:      "balance",
		Usage:     "anarch-lint [FLAGS] balance [--since-rev REV] [--coverprofile FILE] [PACKAGES]",
		ShortHelp: "report Balanced Coupling: strength, distance and volatility of every subsystem pair",
		LongHelp: `Balance loads the packages (default ./...) and reports, deterministically, Vlad
Khononov's Balanced Coupling for every ordered pair of subsystems
(connascence.md §9):

  - integration strength: intrusive, functional, model or contract coupling,
    with the strongest connascence at that level;
  - distance: the module, owner (CODEOWNERS or the model) and deploy
    boundaries, and asynchronous integration a reviewer confirmed;
  - volatility: each subsystem's subdomain, a judgement slot, with its churn
    since --since-rev as evidence;
  - the quadrant, archfit's balance score, and package exposure;
  - with --coverprofile, the packages many import whose tests run little
    of them.

Tight coupling on facts is a finding, an error when the model's "balance"
section sets "gate": true. Candidates gate only once an approved verdict
confirms them. CODEOWNERS (--codeowners) paths are matched relative to
--dir. It always
exits 0 once the packages load; use check to gate.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	l, err := cfg.Load(ctx, args, root.AllFacts)
	if err != nil {
		return fmt.Errorf("balance: %w", err)
	}
	conn := connascence.Analyze(&l)
	in, err := cfg.BalanceInput(ctx, &l, &conn,
		root.History{SinceRev: cfg.SinceRev, CoverProfile: cfg.CoverProfile})
	if err != nil {
		return fmt.Errorf("balance: %w", err)
	}
	r := balance.Analyze(&in)
	if err := cfg.Write(
		&r,
		func(w io.Writer) error { return text.WriteBalance(w, &r) },
	); err != nil {
		return fmt.Errorf("balance: %w", err)
	}
	return nil
}
