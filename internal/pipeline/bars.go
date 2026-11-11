package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/git"
)

// BarFiles names the bars to compare: the revision, and the baseline and
// waivers files, each empty for its default in Dir.
type BarFiles struct {
	Rev, BaselinePath, WaiversPath string
}

// barReader reads the bars at one revision of the repository holding dir,
// and in the work tree.
type barReader struct {
	dir, rev string
}

// LoosenedSince compares the bars at bars.Rev with the work tree's: the
// model, the baseline and the waivers. It returns how the work tree loosens
// them or, when they cannot be compared, why not: rev is unknown, as in a
// shallow clone, or a file lies outside Dir. A model or baseline that rev
// lacks has nothing to loosen; waivers it lacks count as none.
func (o *Options) LoosenedSince(ctx context.Context, bars BarFiles) ([]string, string, error) {
	rev, baselinePath, waiversPath := bars.Rev, bars.BaselinePath, bars.WaiversPath
	if baselinePath == "" {
		baselinePath = filepath.Join(o.Dir, DefaultBaseline)
	}
	if waiversPath == "" {
		waiversPath = filepath.Join(o.Dir, DefaultWaivers)
	}
	r := barReader{dir: o.Dir, rev: rev}
	var prev, next arch.Bars
	why, err := r.model(ctx, o.ModelFile(), &prev, &next)
	if why == "" && err == nil {
		why, err = r.baseline(ctx, baselinePath, &prev, &next)
	}
	if why == "" && err == nil {
		why, err = r.waivers(ctx, waiversPath, &prev, &next)
	}
	if why != "" || err != nil {
		return nil, why, err
	}
	return arch.Loosened(&prev, &next), "", nil
}

// model reads the model at the revision and in the work tree. A model the
// revision lacks has nothing to loosen; one the work tree deleted loosens
// every rule.
func (r *barReader) model(ctx context.Context, path string, prev, next *arch.Bars) (string, error) {
	const op = "pipeline.barReader.model"
	old, cur, why, err := r.files(ctx, path)
	if why != "" || err != nil || old == nil {
		return why, err
	}
	if prev.Model, err = DecodeModelFile(path, old); err != nil {
		return "", &arch.Error{Op: op, Err: err}
	}
	// The rules a rule set holds are bars too, so a set read at the revision
	// shows a rule removed from it.
	if prev.Model, err = includeRuleSets(&prev.Model, path, r.atRevision(ctx)); err != nil {
		return "the rule sets at " + r.rev + " could not be read: " + arch.ErrorMessage(err), nil
	}
	if cur == nil {
		return "", nil
	}
	if next.Model, err = DecodeModelFile(path, cur); err != nil {
		return "", &arch.Error{Op: op, Err: err}
	}
	if next.Model, err = includeRuleSets(&next.Model, path, ReadFile); err != nil {
		return "", &arch.Error{Op: op, Err: err}
	}
	return "", nil
}

// atRevision returns a reader of files as the revision has them, by their
// path in the work tree.
func (r *barReader) atRevision(ctx context.Context) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		rel, inside := relativeTo(r.dir, path)
		if !inside {
			return nil, &arch.Error{Code: arch.EINVALID, Message: path + " lies outside --dir"}
		}
		return git.Show(ctx, r.dir, git.FileAt{Rev: r.rev, Path: rel})
	}
}

// baseline reads the baseline at the revision and in the work tree, when
// both have one.
func (r *barReader) baseline(
	ctx context.Context,
	path string,
	prev, next *arch.Bars,
) (string, error) {
	const op = "pipeline.barReader.baseline"
	old, cur, why, err := r.files(ctx, path)
	if why != "" || err != nil || old == nil || cur == nil {
		return why, err
	}
	pb, err := arch.DecodeBaseline(bytes.NewReader(old))
	if err != nil {
		return "", &arch.Error{Op: op, Err: err}
	}
	nb, err := arch.DecodeBaseline(bytes.NewReader(cur))
	if err != nil {
		return "", &arch.Error{Op: op, Err: err}
	}
	prev.Baseline, next.Baseline = &pb, &nb
	return "", nil
}

// waivers reads the waivers at the revision and in the work tree; a missing
// file is none.
func (r *barReader) waivers(
	ctx context.Context,
	path string,
	prev, next *arch.Bars,
) (string, error) {
	const op = "pipeline.barReader.waivers"
	old, cur, why, err := r.files(ctx, path)
	if why != "" || err != nil {
		return why, err
	}
	for _, side := range []struct {
		src     []byte
		waivers *[]arch.Waiver
	}{{old, &prev.Waivers}, {cur, &next.Waivers}} {
		if side.src == nil {
			continue
		}
		if *side.waivers, err = arch.DecodeWaivers(bytes.NewReader(side.src)); err != nil {
			return "", &arch.Error{Op: op, Err: err}
		}
	}
	return "", nil
}

// files reads one bar's file at the revision and in the work tree; either is
// nil when it does not exist. why says why the file cannot be compared.
func (r *barReader) files(
	ctx context.Context,
	path string,
) (old, cur []byte, why string, err error) {
	rel, inside := relativeTo(r.dir, path)
	if !inside {
		return nil, nil, path + " lies outside --dir, so the bars were not compared", nil
	}
	old, showErr := git.Show(ctx, r.dir, git.FileAt{Rev: r.rev, Path: rel})
	switch arch.ErrorCode(showErr) {
	case "", arch.ENOTFOUND:
	default:
		return nil, nil, "the bars at " + r.rev + " could not be read: " +
			arch.ErrorMessage(showErr), nil
	}
	cur, err = ReadFile(path)
	if arch.ErrorCode(err) == arch.ENOTFOUND {
		return old, nil, "", nil
	}
	if err != nil {
		return nil, nil, "", &arch.Error{Op: "pipeline.barReader.files", Err: err}
	}
	return old, cur, "", nil
}

// relativeTo returns path relative to dir, and false when it lies outside.
func relativeTo(dir, path string) (string, bool) {
	rel, err := filepath.Rel(dir, path)
	return rel, err == nil && !strings.HasPrefix(rel, "..")
}

// BaselineAt measures the code at rev, checked out apart from the work tree,
// under the current model, and returns it as a baseline: the gate then asks
// what the change adds to that revision, whatever the committed baseline
// says (TODO.md, "a base revision checked out in a git worktree"). The
// model, judgements and CODEOWNERS are read as they are now; the code, and
// the domain models the rules name, as they were at rev.
func (o *Options) BaselineAt(
	ctx context.Context, rev string, patterns []string,
) (arch.Baseline, error) {
	const op = "pipeline.Options.BaselineAt"
	dir, remove, err := git.Worktree(ctx, o.Dir, rev)
	if err != nil {
		return arch.Baseline{}, &arch.Error{Op: op, Err: err}
	}
	base, err := o.at(dir)
	if err != nil {
		_ = remove()
		return arch.Baseline{}, &arch.Error{Op: op, Err: err}
	}
	in, err := base.GateInput(ctx, patterns, nil)
	if rmErr := remove(); err == nil && rmErr != nil {
		err = rmErr
	}
	if err != nil {
		return arch.Baseline{}, &arch.Error{Op: op, Err: err}
	}
	return arch.NewBaseline(&in), nil
}

// at returns the options with Dir moved to dir, and every input file that
// lies outside the code (the model, the judgements and CODEOWNERS) still
// read from where it is now. A missing default model stays missing.
func (o *Options) at(dir string) (Options, error) {
	base := *o
	base.Dir = dir
	if _, err := os.Stat(o.ModelFile()); err == nil && base.ModelPath == "" {
		base.ModelPath = o.ModelFile()
	}
	for _, path := range []*string{&base.ModelPath, &base.JudgementsPath, &base.CodeownersPath} {
		if *path == "" {
			continue
		}
		abs, err := filepath.Abs(*path)
		if err != nil {
			return Options{}, &arch.Error{Code: arch.EINVALID, Message: err.Error()}
		}
		*path = abs
	}
	return base, nil
}
