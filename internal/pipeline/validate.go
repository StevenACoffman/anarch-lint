package pipeline

import (
	"context"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/git"
)

// Validate measures the code at the revision after names, checked out apart
// from the work tree, and tests its package metrics against the fixes among
// the commits after names (IMPROVEMENTS.md §10.5). The model is read as it
// is now, as BaselineAt reads it, so the packages are grouped alike at both
// ends.
//
// Requires: after.Commits > 0.
func (o *Options) Validate(
	ctx context.Context, after git.After, patterns []string,
) (arch.Validation, error) {
	const op = "pipeline.Options.Validate"
	history, err := git.Fixes(ctx, o.Dir, after)
	if err != nil {
		return arch.Validation{}, &arch.Error{Op: op, Err: err}
	}
	base, remove, err := checkout(ctx, o, after.Rev)
	if err != nil {
		return arch.Validation{}, &arch.Error{Op: op, Err: err}
	}
	in, err := base.Load(ctx, patterns, NoFacts)
	if rmErr := remove(); err == nil && rmErr != nil {
		err = rmErr
	}
	if err != nil {
		return arch.Validation{}, &arch.Error{Op: op, Err: err}
	}
	return arch.Validate(&arch.ValidationInput{
		Program: in.Program, Model: in.Model, History: history,
	}), nil
}
