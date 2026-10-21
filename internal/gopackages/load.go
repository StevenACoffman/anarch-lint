// Package gopackages loads Go packages with golang.org/x/tools/go/packages and
// translates them into the facts of an [arch.Program]: types, functions with
// their size and complexity, type-to-type references and imports.
//
// It is the only package that knows about go/packages, go/ast and go/types.
// Every position it reports is relative to the directory it loaded from, so a
// program describes the code, not the machine that read it.
package gopackages

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// loadMode is everything extraction reads: syntax and type information for
// the packages under analysis, and their import graph.
const loadMode = packages.NeedName | packages.NeedFiles | packages.NeedImports |
	packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule

// maxReportedErrors bounds the package errors one failure lists.
const maxReportedErrors = 10

// Config says what to load.
type Config struct {
	// Dir is the directory patterns are resolved in, and the root every
	// reported path is relative to.
	Dir string
	// Patterns are go list patterns, such as "./...".
	Patterns []string
	// Tags are build tags; they decide which files are compiled, and so are
	// an input of every measure.
	Tags []string
	// Facts asks for the connascence and Balanced Coupling facts of
	// [arch.Facts], which only those measures read.
	Facts bool
}

// Load loads the packages cfg names and returns their facts, sorted as
// [arch.Program] requires. Test files are left out. A package that fails to
// load or type-check is an EINVALID error listing the first problems, because
// measures over a partial program would look complete without being so.
func Load(ctx context.Context, cfg Config) (arch.Program, error) {
	const op = "gopackages.Load"
	dir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return arch.Program{}, &arch.Error{Op: op, Err: &arch.Error{
			Code: arch.EINVALID, Message: fmt.Sprintf("directory %q: %v", cfg.Dir, err),
		}}
	}
	pcfg := &packages.Config{Context: ctx, Mode: loadMode, Dir: dir}
	if len(cfg.Tags) > 0 {
		pcfg.BuildFlags = []string{"-tags=" + strings.Join(cfg.Tags, ",")}
	}
	pkgs, err := packages.Load(pcfg, cfg.Patterns...)
	if err != nil {
		return arch.Program{}, &arch.Error{Op: op, Err: &arch.Error{
			Code: arch.EINVALID, Message: "load packages: " + err.Error(),
		}}
	}
	if msgs := loadErrors(pkgs); len(msgs) > 0 {
		return arch.Program{}, &arch.Error{Op: op, Err: &arch.Error{
			Code:    arch.EINVALID,
			Message: "packages do not load cleanly:\n  " + strings.Join(msgs, "\n  "),
		}}
	}
	if len(pkgs) == 0 {
		return arch.Program{}, &arch.Error{Op: op, Err: &arch.Error{
			Code: arch.ENOTFOUND, Message: fmt.Sprintf("no packages match %q", cfg.Patterns),
		}}
	}
	prog, err := extract(dir, pkgs, cfg.Facts, os.ReadFile)
	if err != nil {
		return arch.Program{}, &arch.Error{Op: op, Err: err}
	}
	nested, err := nestedModules(dir, cfg.Patterns, pkgs)
	if err != nil {
		return arch.Program{}, &arch.Error{Op: op, Err: err}
	}
	prog.Excluded = append(prog.Excluded, nested...)
	slices.SortFunc(prog.Excluded, compareExcluded)
	if cfg.Facts {
		if prog.Facts.Clones, err = clonesOf(dir, &prog); err != nil {
			return arch.Program{}, &arch.Error{Op: op, Err: err}
		}
	}
	return prog, nil
}

// loadErrors returns every error of the loaded packages and their
// dependencies, sorted and de-duplicated, at most maxReportedErrors of them
// plus a count of the rest.
func loadErrors(pkgs []*packages.Package) []string {
	var msgs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			msgs = append(msgs, e.Error())
		}
	})
	slices.Sort(msgs)
	msgs = slices.Compact(msgs)
	if len(msgs) > maxReportedErrors {
		rest := len(msgs) - maxReportedErrors
		msgs = append(msgs[:maxReportedErrors], fmt.Sprintf("... and %d more", rest))
	}
	return msgs
}

// extract turns loaded packages into a sorted program.
func extract(
	dir string,
	pkgs []*packages.Package,
	facts bool,
	readFile func(string) ([]byte, error),
) (arch.Program, error) {
	slices.SortFunc(
		pkgs,
		func(a, b *packages.Package) int { return cmp.Compare(a.PkgPath, b.PkgPath) },
	)
	inScope := map[string]bool{}
	for _, p := range pkgs {
		inScope[p.PkgPath] = true
	}
	x := newExtractor(dir, inScope, facts)
	for _, p := range pkgs {
		if err := x.addPackage(p, readFile); err != nil {
			return arch.Program{}, err
		}
	}
	return x.program(), nil
}

// relPath returns file relative to dir with forward slashes, or file itself
// when it lies outside dir.
func relPath(dir, file string) string {
	rel, err := filepath.Rel(dir, file)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(file)
	}
	return filepath.ToSlash(rel)
}
