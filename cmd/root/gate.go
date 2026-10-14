package root

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
	"github.com/StevenACoffman/anarch-lint/internal/codeowners"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
	"github.com/StevenACoffman/anarch-lint/internal/coverage"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
	"github.com/StevenACoffman/anarch-lint/internal/git"
)

// History names the evidence balance may read beyond the code: the commits
// after SinceRev, and a coverage profile; empty means none.
type History struct {
	SinceRev     string
	CoverProfile string
}

// GateInput measures everything the gate compares with a baseline: the MMI
// report always, and the fitness, connascence and Balanced Coupling reports
// when the model opts them in. changed is the change set a scoped check
// measures, which the rules on changes read; nil means none.
func (cfg *Config) GateInput(
	ctx context.Context, patterns []string, changed arch.ChangedLines,
) (arch.GateInput, error) {
	const op = "root.Config.GateInput"
	l, err := cfg.Load(ctx, patterns, GatedFacts)
	if err != nil {
		return arch.GateInput{}, &arch.Error{Op: op, Err: err}
	}
	l.Changed = changed
	report := arch.Analyze(&l)
	in := report.GateInput()
	if l.Model.Fitness.Enabled() {
		fit := fitness.Analyze(&l)
		fit.AddTo(&in)
	}
	if !l.Model.Connascence.Enabled() && !l.Model.Balance.Enabled() {
		return in, nil
	}
	conn := connascence.Analyze(&l)
	if l.Model.Connascence.Enabled() {
		conn.AddTo(&in)
	}
	if l.Model.Balance.Enabled() {
		bin, err := cfg.BalanceInput(ctx, &l, &conn, History{})
		if err != nil {
			return arch.GateInput{}, &arch.Error{Op: op, Err: err}
		}
		r := balance.Analyze(&bin)
		r.AddTo(&in)
	}
	return in, nil
}

// BalanceInput gathers what the Balanced Coupling measure reads beyond the
// program: the connascence instances, the owners from CODEOWNERS, and the
// history h names.
func (cfg *Config) BalanceInput(
	ctx context.Context, l *arch.Input, conn *connascence.Report, h History,
) (balance.Input, error) {
	const op = "root.Config.BalanceInput"
	in := balance.Input{
		Input:       l,
		Connascence: connascence.Instances(&l.Program, &l.Model),
		Rejected:    conn.RejectedFingerprints(),
	}
	var err error
	if in.Owners, in.CodeownersHash, err = cfg.readOwners(&l.Program); err != nil {
		return balance.Input{}, &arch.Error{Op: op, Err: err}
	}
	if h.SinceRev != "" {
		churn, err := git.Churn(ctx, cfg.Dir, h.SinceRev)
		if err != nil {
			return balance.Input{}, &arch.Error{Op: op, Err: err}
		}
		in.Churn = &churn
	}
	if h.CoverProfile != "" {
		src, err := ReadFile(h.CoverProfile)
		if err != nil {
			return balance.Input{}, &arch.Error{Op: op, Err: err}
		}
		cov, err := coverage.Attribute(src, &l.Program)
		if err != nil {
			return balance.Input{}, &arch.Error{Op: op, Err: err}
		}
		in.Coverage = &cov
	}
	return in, nil
}

// readOwners reads the CODEOWNERS file, if any, and resolves each package's
// owners, with the file's hash for the report's inputs. A file named by the
// flag must exist; a missing default file means no owners.
func (cfg *Config) readOwners(p *arch.Program) (map[string][]string, string, error) {
	const op = "root.Config.readOwners"
	paths, required := []string{cfg.CodeownersPath}, true
	if cfg.CodeownersPath == "" {
		required = false
		paths = []string{
			filepath.Join(cfg.Dir, ".github", "CODEOWNERS"),
			filepath.Join(cfg.Dir, "CODEOWNERS"),
			filepath.Join(cfg.Dir, "docs", "CODEOWNERS"),
		}
	}
	for _, path := range paths {
		src, err := ReadFile(path)
		if !required && arch.ErrorCode(err) == arch.ENOTFOUND {
			continue
		}
		if err != nil {
			return nil, "", &arch.Error{Op: op, Err: err}
		}
		rules, err := codeowners.Parse(bytes.NewReader(src))
		if err != nil {
			return nil, "", &arch.Error{Op: op + " " + path, Err: err}
		}
		sum := sha256.Sum256(src)
		return codeowners.Resolve(rules, p), hex.EncodeToString(sum[:]), nil
	}
	return nil, "", nil
}

// ReadBaseline reads the baseline file, which must exist; an empty path
// means anarch-baseline.json in Dir.
func (cfg *Config) ReadBaseline(path string) (arch.Baseline, error) {
	const op = "root.Config.ReadBaseline"
	if path == "" {
		path = filepath.Join(cfg.Dir, DefaultBaseline)
	}
	src, err := ReadFile(path)
	if arch.ErrorCode(err) == arch.ENOTFOUND {
		return arch.Baseline{}, &arch.Error{Op: op, Err: &arch.Error{
			Code:    arch.ENOTFOUND,
			Message: fmt.Sprintf("no baseline at %s; run baseline first", path),
		}}
	}
	if err != nil {
		return arch.Baseline{}, &arch.Error{Op: op, Err: err}
	}
	b, err := arch.DecodeBaseline(bytes.NewReader(src))
	if err != nil {
		return arch.Baseline{}, &arch.Error{Op: op + " " + path, Err: err}
	}
	return b, nil
}
