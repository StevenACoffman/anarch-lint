// Package codeowners reads GitHub's CODEOWNERS format and answers who owns
// each package, for the owner boundary of Balanced Coupling's distance
// (connascence.md §9.3). Parsing and matching are github.com/hmarr/codeowners;
// this package turns its rules into owners per package. The shell reads the
// file.
package codeowners

import (
	"fmt"
	"io"
	"slices"

	"github.com/hmarr/codeowners"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Rules are a file's rules in order. The last rule that matches a path wins,
// as on GitHub; a rule with no owners leaves its paths unowned.
type Rules struct {
	set codeowners.Ruleset
}

// Parse reads a CODEOWNERS file. Blank lines and comments are skipped; a
// line the format does not allow is EINVALID, naming the line.
func Parse(r io.Reader) (Rules, error) {
	set, err := codeowners.ParseFile(r)
	if err != nil {
		return Rules{}, &arch.Error{
			Code:    arch.EINVALID,
			Message: fmt.Sprintf("CODEOWNERS %s", err),
		}
	}
	return Rules{set: set}, nil
}

// Owners returns the sorted owners of a path relative to the repository
// root: those of the last rule that matches the path or a directory holding
// it. A path no rule matches has none.
func (rs Rules) Owners(path string) []string {
	rule, err := rs.set.Match(path)
	if err != nil || rule == nil || len(rule.Owners) == 0 {
		// Match never fails: ParseFile compiled every pattern.
		return nil
	}
	owners := make([]string, len(rule.Owners))
	for i, o := range rule.Owners {
		owners[i] = o.String()
	}
	slices.Sort(owners)
	return owners
}

// Resolve returns the owners of each package of p: the sorted union of the
// owners of its files. A package none of whose files has an owner is left
// out.
func Resolve(rs Rules, p *arch.Program) map[string][]string {
	out := map[string][]string{}
	for _, pkg := range p.Packages {
		var owners []string
		for _, f := range pkg.Files {
			owners = append(owners, rs.Owners(f)...)
		}
		if len(owners) > 0 {
			slices.Sort(owners)
			out[pkg.Path] = slices.Compact(owners)
		}
	}
	return out
}
