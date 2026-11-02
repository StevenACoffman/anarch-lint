package judge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/renameio/v2/maybe"
	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/judging"
	"github.com/StevenACoffman/anarch-lint/internal/text"
	"github.com/StevenACoffman/skillet/calibration"
	"github.com/StevenACoffman/skillet/stats"
)

// admitConfig holds the configuration for judge admit.
type admitConfig struct {
	*Config
	cacheOnly bool
	signer    string
	flags     *ff.FlagSet
	command   *ff.Command
}

func newAdmit(parent *Config) {
	cfg := admitConfig{Config: parent}
	cfg.flags = ff.NewFlagSet("admit").SetParent(parent.Flags)
	cfg.flags.BoolVar(&cfg.cacheOnly, 0, "cache-only",
		"read replies only from the cache, and fail listing any that is missing, as CI does")
	cfg.flags.StringVar(&cfg.signer, 0, "signer", "",
		"who commits the judgements file (default: its current signer)")
	cfg.command = &ff.Command{
		Name:      "admit",
		Usage:     "anarch-lint --judgements FILE judge admit [--cache-only] [--signer NAME]",
		ShortHelp: "parse the replies, check the canaries, and record the tallies",
		Flags:     cfg.flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.command)
}

func (cfg *admitConfig) exec(_ context.Context, _ []string) error {
	r, signer, err := cfg.admit(cfg.cacheOnly)
	if err != nil {
		return fmt.Errorf("judge admit: %w", err)
	}
	if cfg.cacheOnly && len(r.Missing) > 0 {
		return fmt.Errorf("judge admit: %d replies are not cached, such as %s",
			len(r.Missing), r.Missing[0])
	}
	if cfg.signer != "" {
		signer = cfg.signer
	}
	if signer == "" {
		return &root.UsageError{Err: errors.New("judge admit: name the --signer")}
	}
	var out bytes.Buffer
	if err := arch.EncodeJudgements(&out, signer, r.Verdicts); err != nil {
		return fmt.Errorf("judge admit: %w", err)
	}
	if err := maybe.WriteFile(cfg.JudgementsPath, out.Bytes(), 0o644); err != nil {
		return fmt.Errorf("judge admit: %w", err)
	}
	if err := cfg.Write(&r, func(w io.Writer) error {
		return text.WriteJudging(w, &r, r.Agreements(statistics()))
	}); err != nil {
		return fmt.Errorf("judge admit: %w", err)
	}
	if r.Untrustworthy() {
		return ErrUntrustworthy
	}
	return nil
}

// statistics are skillet's Wilson interval and calibration, which the shell
// supplies so the judging core imports no third-party code (mmi-plan.md
// §9.4).
func statistics() judging.Statistics {
	return judging.Statistics{
		Interval: stats.Wilson,
		Calibrate: func(outcomes []judging.GoldOutcome) (float64, float64) {
			samples := make([]calibration.Sample, len(outcomes))
			for i, o := range outcomes {
				samples[i] = calibration.Sample{Confidence: o.Confidence, Correct: o.Correct}
			}
			c := calibration.Compute(samples)
			return c.ECE, c.Brier
		},
	}
}
