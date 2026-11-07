package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/balance"
	"github.com/StevenACoffman/anarch-lint/internal/codeowners"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
	"github.com/StevenACoffman/anarch-lint/internal/fitness"
	"github.com/StevenACoffman/anarch-lint/internal/git"
)

// History names the change history balance may read beyond the code: the
// commits after SinceRev; empty means none.
type History struct {
	SinceRev string
}

// GateInput measures everything the gate compares with a baseline: the MMI
// report always, and the fitness, connascence and Balanced Coupling reports
// when the model opts them in. changed is the change set a scoped check
// measures, which the rules on changes read; nil means none.
func (o *Options) GateInput(
	ctx context.Context, patterns []string, changed arch.ChangedLines,
) (arch.GateInput, error) {
	const op = "pipeline.Options.GateInput"
	l, err := o.Load(ctx, patterns, GatedFacts)
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
		in.Rank(&l.Program)
		return in, nil
	}
	conn := connascence.Analyze(&l)
	if l.Model.Connascence.Enabled() {
		conn.AddTo(&in)
	}
	if l.Model.Balance.Enabled() {
		bin, err := o.BalanceInput(ctx, &l, &conn, History{})
		if err != nil {
			return arch.GateInput{}, &arch.Error{Op: op, Err: err}
		}
		r := balance.Analyze(&bin)
		r.AddTo(&in)
	}
	in.Rank(&l.Program)
	return in, nil
}

// JudgementSlots measures everything that asks a reviewer: the MMI's
// judgement criteria, the connascence candidates over the limits and the
// Balanced Coupling questions, whether or not the model gates on them.
func (o *Options) JudgementSlots(
	ctx context.Context, patterns []string,
) ([]arch.JudgementSlot, error) {
	const op = "pipeline.Options.JudgementSlots"
	l, err := o.Load(ctx, patterns, AllFacts)
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	report := arch.Analyze(&l)
	conn := connascence.Analyze(&l)
	bin, err := o.BalanceInput(ctx, &l, &conn, History{})
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	bal := balance.Analyze(&bin)
	return slices.Concat(report.Judgements, conn.Judgements, bal.Judgements), nil
}

// BalanceInput gathers what the Balanced Coupling measure reads beyond the
// program: the connascence instances, the owners from CODEOWNERS, and the
// history h names. A coverage profile comes with l (ReadCoverage).
func (o *Options) BalanceInput(
	ctx context.Context, l *arch.Input, conn *connascence.Report, h History,
) (balance.Input, error) {
	const op = "pipeline.Options.BalanceInput"
	in := balance.Input{
		Input:       l,
		Connascence: connascence.Instances(&l.Program, &l.Model),
		Rejected:    conn.RejectedFingerprints(),
	}
	var err error
	if in.Owners, in.CodeownersHash, err = o.readOwners(&l.Program); err != nil {
		return balance.Input{}, &arch.Error{Op: op, Err: err}
	}
	if h.SinceRev != "" {
		if in.Churn, in.Input, err = readRange(ctx, o.Dir, l, h.SinceRev); err != nil {
			return balance.Input{}, &arch.Error{Op: op, Err: err}
		}
	}
	return in, nil
}

// readRange reads the churn of the repository at dir since rev and, unless
// l has a change set already, a copy of l whose change set is the range,
// which the untested crossings are drawn from; l itself is left as it was.
func readRange(
	ctx context.Context, dir string, l *arch.Input, rev string,
) (*arch.Churn, *arch.Input, error) {
	const op = "pipeline.readRange"
	churn, err := git.Churn(ctx, dir, rev)
	if err != nil {
		return nil, nil, &arch.Error{Op: op, Err: err}
	}
	if l.Changed != nil {
		return &churn, l, nil
	}
	changed, err := git.ChangedLines(ctx, dir, rev)
	if err != nil {
		return nil, nil, &arch.Error{Op: op, Err: err}
	}
	withChange := *l
	withChange.Changed = changed
	return &churn, &withChange, nil
}

// readOwners reads the CODEOWNERS file, if any, and resolves each package's
// owners, with the file's hash for the report's inputs. A file named by the
// flag must exist; a missing default file means no owners.
func (o *Options) readOwners(p *arch.Program) (map[string][]string, string, error) {
	const op = "pipeline.Options.readOwners"
	paths, required := []string{o.CodeownersPath}, true
	if o.CodeownersPath == "" {
		required = false
		paths = []string{
			filepath.Join(o.Dir, ".github", "CODEOWNERS"),
			filepath.Join(o.Dir, "CODEOWNERS"),
			filepath.Join(o.Dir, "docs", "CODEOWNERS"),
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
func (o *Options) ReadBaseline(path string) (arch.Baseline, error) {
	const op = "pipeline.Options.ReadBaseline"
	if path == "" {
		path = filepath.Join(o.Dir, DefaultBaseline)
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

// ReadWaivers reads the waivers file at path, or anarch-waivers.json in Dir
// when path is empty and that file exists, and fixes today's date: today, or
// the current date when today is empty. It is the one place the clock is
// read, so the domain stays deterministic.
func (o *Options) ReadWaivers(path, today string) ([]arch.Waiver, arch.Date, error) {
	const op = "pipeline.Options.ReadWaivers"
	required := true
	if path == "" {
		path, required = filepath.Join(o.Dir, DefaultWaivers), false
	}
	var waivers []arch.Waiver
	src, err := ReadFile(path)
	switch {
	case !required && arch.ErrorCode(err) == arch.ENOTFOUND:
	case err != nil:
		return nil, "", &arch.Error{Op: op, Err: err}
	default:
		if waivers, err = arch.DecodeWaivers(bytes.NewReader(src)); err != nil {
			return nil, "", &arch.Error{Op: op + " " + path, Err: err}
		}
	}
	if today == "" {
		today = time.Now().Format(time.DateOnly)
	}
	date, err := arch.ParseDate(today)
	if err != nil {
		return nil, "", &arch.Error{Op: op + " --today", Err: err}
	}
	return waivers, date, nil
}
