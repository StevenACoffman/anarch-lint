package gopackages

import (
	"go/scanner"
	"go/token"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// codeLines records which lines of a file contain code: a line counts when a
// token other than a comment starts on it (mmi.md §7.3). Blank lines and
// comment-only lines don't count, and neither do the continuation lines of a
// multi-line raw string.
type codeLines []bool // indexed by 1-based line

// countCodeLines scans src and marks its code lines.
func countCodeLines(src []byte) codeLines {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	// No error handler: the type checker has already accepted the file, and
	// counting lines needs only the tokens.
	var s scanner.Scanner
	s.Init(file, src, nil, 0)
	var lines codeLines
	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			return lines
		}
		if tok == token.COMMENT {
			continue
		}
		line := file.Line(pos)
		for len(lines) <= line {
			lines = append(lines, false)
		}
		lines[line] = true
	}
}

// total returns the number of code lines.
func (c codeLines) total() int { return c.between(0, len(c)) }

// within returns the number of code lines in a span of the same file.
func (c codeLines) within(s arch.Span) int { return c.between(s.Start, s.End) }

// between returns the number of code lines from start to end, inclusive.
func (c codeLines) between(start, end int) int {
	n := 0
	for line := max(start, 0); line <= end && line < len(c); line++ {
		if c[line] {
			n++
		}
	}
	return n
}
