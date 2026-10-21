package gopackages

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

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
