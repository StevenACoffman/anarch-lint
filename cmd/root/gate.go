package root

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
	"github.com/StevenACoffman/anarch-lint/internal/codeowners"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
	"github.com/StevenACoffman/anarch-lint/internal/git"
)

// GateInput measures everything the gate compares with a baseline: the MMI
// report always, and the connascence and Balanced Coupling reports when the
// model opts them in.
func (cfg *Config) GateInput(ctx context.Context, patterns []string) (arch.GateInput, error) {
	const op = "root.Config.GateInput"
	l, err := cfg.Load(ctx, patterns, GatedFacts)
	if err != nil {
		return arch.GateInput{}, &arch.Error{Op: op, Err: err}
	}
	report := arch.Analyze(&l)
	in := report.GateInput()
	if !l.Model.Connascence.Enabled() && !l.Model.Balance.Enabled() {
		return in, nil
	}
	conn := connascence.Analyze(&l)
	if l.Model.Connascence.Enabled() {
		conn.AddTo(&in)
	}
	if l.Model.Balance.Enabled() {
		bin, err := cfg.BalanceInput(ctx, &l, &conn, "")
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
// churn since sinceRev when it is set.
func (cfg *Config) BalanceInput(
	ctx context.Context, l *arch.Input, conn *connascence.Report, sinceRev string,
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
	if sinceRev != "" {
		churn, err := git.Churn(ctx, cfg.Dir, sinceRev)
		if err != nil {
			return balance.Input{}, &arch.Error{Op: op, Err: err}
		}
		in.Churn = &churn
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
