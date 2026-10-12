// Package codeowners reads GitHub's CODEOWNERS format and answers who owns
// each package, for the owner boundary of Balanced Coupling's distance
// (connascence.md §9.3). It parses and matches only; the shell reads the
// file.
package codeowners

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Rule is one line of a CODEOWNERS file: a pattern and the owners of the
// paths it matches. A rule with no owners leaves its paths unowned.
type Rule struct {
	Pattern string
	Owners  []string
	Line    int
	match   *regexp.Regexp
}

// Rules are a file's rules in order. The last rule that matches a path wins,
// as on GitHub.
type Rules []Rule

// Parse reads a CODEOWNERS file. Blank lines and comments are skipped; a
// pattern with a character the format does not allow is EINVALID, naming the
// line.
func Parse(r io.Reader) (Rules, error) {
	var rules Rules
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		re, err := compile(fields[0])
		if err != nil {
			return nil, &arch.Error{
				Code:    arch.EINVALID,
				Message: fmt.Sprintf("CODEOWNERS line %d: %s", n, arch.ErrorMessage(err)),
			}
		}
		owners := slices.Sorted(slices.Values(fields[1:]))
		rules = append(rules, Rule{Pattern: fields[0], Owners: owners, Line: n, match: re})
	}
	if err := sc.Err(); err != nil {
		return nil, &arch.Error{Code: arch.EINVALID, Message: "read CODEOWNERS: " + err.Error()}
	}
	return rules, nil
}

// Owners returns the owners of a path relative to the repository root: those
// of the last rule that matches the path or a directory holding it. A path no
// rule matches has none.
func (rs Rules) Owners(path string) []string {
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i].matches(path) {
			return rs[i].Owners
		}
	}
	return nil
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

// matches reports whether the rule matches path, or a directory above it.
func (r *Rule) matches(path string) bool {
	for p := path; p != "" && p != "."; {
		if r.match.MatchString(p) {
			return true
		}
		i := strings.LastIndex(p, "/")
		if i < 0 {
			return false
		}
		p = p[:i]
	}
	return false
}

// compile turns a gitignore-style pattern into a regular expression over
// paths. A pattern with a slash at its start or in its middle is anchored to
// the root; one without matches at any depth. A trailing slash matches only
// what lies beneath the directory. "*" and "?" stay within one segment; "**"
// crosses segments.
func compile(pattern string) (*regexp.Regexp, error) {
	if strings.ContainsAny(pattern, "\\[]!") {
		return nil, &arch.Error{
			Code: arch.EINVALID,
			Message: fmt.Sprintf(
				"pattern %q uses a character CODEOWNERS does not support",
				pattern,
			),
		}
	}
	dirOnly := strings.HasSuffix(pattern, "/")
	p := strings.TrimSuffix(pattern, "/")
	anchored := strings.HasPrefix(p, "/") || strings.Contains(p, "/")
	p = strings.TrimPrefix(p, "/")
	var b strings.Builder
	b.WriteString("^")
	if !anchored {
		b.WriteString("(?:.*/)?")
	}
	b.WriteString(globToRegexp(p))
	if dirOnly {
		b.WriteString("/.+")
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, &arch.Error{
			Code:    arch.EINVALID,
			Message: fmt.Sprintf("pattern %q: %v", pattern, err),
		}
	}
	return re, nil
}

// globToRegexp translates a glob's "**", "*" and "?" into a regular
// expression and quotes everything else.
func globToRegexp(glob string) string {
	var b strings.Builder
	for i := 0; i < len(glob); i++ {
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(".*")
			i++
		case glob[i] == '*':
			b.WriteString("[^/]*")
		case glob[i] == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(glob[i : i+1]))
		}
	}
	return b.String()
}
