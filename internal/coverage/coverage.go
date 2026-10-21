// Package coverage reads a test coverage profile, as go test -coverprofile
// writes one, with golang.org/x/tools/cover, and attributes its statements to
// the program's functions (IMPROVEMENTS.md 6.4). The shell reads the file.
package coverage

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"slices"

	"golang.org/x/tools/cover"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Attribute parses a profile and gives each block's statements to the
// function whose span holds the block's first line; a block outside every
// function, such as a package-level initializer, counts toward none. A
// malformed profile is EINVALID.
func Attribute(src []byte, p *arch.Program) (arch.Coverage, error) {
	profiles, err := cover.ParseProfilesFromReader(bytes.NewReader(src))
	if err != nil {
		return arch.Coverage{}, &arch.Error{
			Code: arch.EINVALID, Message: "coverage profile: " + err.Error(),
		}
	}
	funcs := funcsByFile(p)
	byFunc := map[string]*arch.FuncCoverage{}
	for _, prof := range profiles {
		for _, b := range prof.Blocks {
			fn := funcAt(funcs[prof.FileName], b.StartLine)
			if fn == nil {
				continue
			}
			fc := byFunc[fn.ID]
			if fc == nil {
				fc = &arch.FuncCoverage{Func: fn.ID}
				byFunc[fn.ID] = fc
			}
			fc.Statements += b.NumStmt
			if b.Count > 0 {
				fc.Covered += b.NumStmt
			}
		}
	}
	sum := sha256.Sum256(src)
	out := arch.Coverage{Hash: hex.EncodeToString(sum[:]), Funcs: make([]arch.FuncCoverage, 0)}
	for _, fc := range byFunc {
		out.Funcs = append(out.Funcs, *fc)
	}
	slices.SortFunc(
		out.Funcs,
		func(a, b arch.FuncCoverage) int { return cmp.Compare(a.Func, b.Func) },
	)
	return out, nil
}

// funcsByFile indexes the functions by the name a profile gives their file:
// the package's import path, a slash, and the file's base name.
func funcsByFile(p *arch.Program) map[string][]*arch.Func {
	pkgOf := map[string]string{}
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			pkgOf[f] = pkg.Path
		}
	}
	out := map[string][]*arch.Func{}
	for i := range p.Funcs {
		fn := &p.Funcs[i]
		name := pkgOf[fn.Span.File] + "/" + path.Base(fn.Span.File)
		out[name] = append(out[name], fn)
	}
	return out
}

// funcAt returns the function whose span holds line, or nil.
func funcAt(funcs []*arch.Func, line int) *arch.Func {
	for _, fn := range funcs {
		if fn.Span.Start <= line && line <= fn.Span.End {
			return fn
		}
	}
	return nil
}
