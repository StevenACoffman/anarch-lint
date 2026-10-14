package connascence

import (
	"fmt"
	"strconv"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// files finds the package of a file and the function at a line.
type files struct {
	pkg   map[string]string
	funcs map[string][]*arch.Func
}

// partialCloneDetector reports runs of code that dupl found repeated in two
// or more packages: connascence of algorithm, like a whole-body clone, but
// found within functions too (connascence.md §2.5, detector 4). A group
// every copy of which is the whole of functions of one body shape is left
// to cloneDetector, which reports it already.
func partialCloneDetector(s *subject) []arch.Connascence {
	var out []arch.Connascence
	files := fileIndex(s.p)
	for _, g := range s.p.Facts.Clones {
		sites := make([]arch.Site, len(g.Fragments))
		hashes := map[string]bool{}
		for i, frag := range g.Fragments {
			fn := files.funcAt(frag)
			sites[i] = arch.Site{File: frag.File, Line: frag.Start, Package: files.pkg[frag.File]}
			if fn != nil {
				sites[i].Func, sites[i].Type = fn.ID, fn.Receiver
			}
			hashes[wholeBodyHash(fn, frag)] = true
		}
		if distinct(sites, sitePackage) < 2 || (len(hashes) == 1 && !hashes[""]) {
			continue
		}
		first := g.Fragments[0]
		out = append(out, s.instance(&instanceSpec{
			kind:       arch.StrengthAlgorithm,
			detector:   "algorithm.partial-clone",
			subject:    first.File + ":" + strconv.Itoa(first.Start),
			basis:      arch.BasisCandidate,
			confidence: arch.ConfidenceLow,
			sites:      sites,
			message: fmt.Sprintf("%d copies of lines %d–%d of %s in %d packages",
				len(g.Fragments), first.Start, first.End, first.File, distinct(sites, sitePackage)),
			fix: "extract the repeated code into one function both sides call, or confirm " +
				"that the copies may evolve apart",
		}))
	}
	return out
}

// wholeBodyHash is the body hash of fn when the fragment spans all of fn,
// and "" otherwise.
func wholeBodyHash(fn *arch.Func, frag arch.Span) string {
	if fn == nil || frag.Start > fn.Span.Start || frag.End < fn.Span.End {
		return ""
	}
	return fn.BodyHash
}

// fileIndex indexes p's files by path.
func fileIndex(p *arch.Program) files {
	f := files{pkg: map[string]string{}, funcs: map[string][]*arch.Func{}}
	for _, pkg := range p.Packages {
		for _, file := range pkg.Files {
			f.pkg[file] = pkg.Path
		}
	}
	for i := range p.Funcs {
		fn := &p.Funcs[i]
		f.funcs[fn.Span.File] = append(f.funcs[fn.Span.File], fn)
	}
	return f
}

// funcAt returns the function whose span holds the fragment's first line, or
// nil.
func (f files) funcAt(frag arch.Span) *arch.Func {
	for _, fn := range f.funcs[frag.File] {
		if fn.Span.Start <= frag.Start && frag.Start <= fn.Span.End {
			return fn
		}
	}
	return nil
}
