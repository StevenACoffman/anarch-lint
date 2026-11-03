package gopackages

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// addSourceImports records the imports the type checker does not see,
// read from source: those of the files the build excludes, and those of
// the test files.
func (x *extractor) addSourceImports(
	p *packages.Package,
	readFile func(string) ([]byte, error),
) error {
	if err := x.addInactiveImports(p, readFile); err != nil {
		return err
	}
	return x.addTestImports(p, readFile)
}

// addInactiveImports records the imports of the files the build constraints
// exclude from p, which the load lists by name only: each is read and its
// imports parsed from source, keeping the first position of each
// package-to-package import, so the import rules check a file of another
// platform too. Test files are not read, as they are not loaded.
func (x *extractor) addInactiveImports(
	p *packages.Package,
	readFile func(string) ([]byte, error),
) error {
	for _, name := range p.IgnoredFiles {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := readFile(name)
		if err != nil {
			return &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("read %s: %v", name, err)}
		}
		_, imports := sourceImports(name, src)
		file := relPath(x.dir, name)
		for _, spec := range imports {
			keepFirst(x.inactive, arch.Import{
				From: p.PkgPath, To: spec.to,
				Pos:      arch.Position{File: file, Line: spec.line},
				Inactive: true,
			})
		}
	}
	return nil
}

// addTestImports records the imports of p's test files, internal and
// external, read from their source as the inactive imports are, keeping
// the first position of each import: the import rules that opt in check
// them (IMPROVEMENTS.md §9.7). An external test package, p's name with
// "_test", counts as p.
func (x *extractor) addTestImports(
	p *packages.Package,
	readFile func(string) ([]byte, error),
) error {
	if len(p.GoFiles) == 0 {
		return nil
	}
	names, err := filepath.Glob(filepath.Join(filepath.Dir(p.GoFiles[0]), "*_test.go"))
	if err != nil {
		return &arch.Error{Code: arch.EINVALID, Message: err.Error()}
	}
	for _, name := range names {
		src, err := readFile(name)
		if err != nil {
			return &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("read %s: %v", name, err)}
		}
		_, imports := sourceImports(name, src)
		file := relPath(x.dir, name)
		for _, spec := range imports {
			if spec.to == p.PkgPath {
				continue // an external test importing the package it tests
			}
			keepFirst(x.tests, arch.Import{
				From: p.PkgPath, To: spec.to,
				Pos:  arch.Position{File: file, Line: spec.line},
				Test: true,
			})
		}
	}
	return nil
}
