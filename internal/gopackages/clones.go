package gopackages

import (
	"cmp"
	"path/filepath"
	"slices"

	"github.com/golangci/dupl/lib"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// cloneThreshold is dupl's threshold: a clone is a run of consecutive
// statements or declarations, each of at least this many syntax-tree nodes,
// repeated elsewhere. dupl's own default, 15, reports a few repeated lines,
// and golangci-lint's 150 only the largest statements; at 60, on anarch-lint
// itself, every clone spans at least four lines.
const cloneThreshold = 60

// spanUnion joins spans into groups: a union-find over the spans seen.
type spanUnion struct {
	spans  []arch.Span
	index  map[arch.Span]int
	parent []int
}

// clonesOf runs dupl over the files the measures read and returns its clone
// groups, sorted by their first fragment. dupl pairs each copy with the next
// one of its group; the pairs are joined back into groups here.
func clonesOf(dir string, p *arch.Program) ([]arch.CloneGroup, error) {
	var files []string
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			files = append(files, filepath.Join(dir, filepath.FromSlash(f)))
		}
	}
	slices.Sort(files)
	issues, err := lib.Run(files, cloneThreshold)
	if err != nil {
		return nil, &arch.Error{Code: arch.EINTERNAL, Message: "find clones: " + err.Error()}
	}
	var u spanUnion
	for _, is := range issues {
		u.join(
			arch.Span{
				File: relPath(dir, is.From.Filename()), Start: is.From.LineStart(),
				End: is.From.LineEnd(),
			},
			arch.Span{
				File: relPath(dir, is.To.Filename()), Start: is.To.LineStart(),
				End: is.To.LineEnd(),
			},
		)
	}
	return u.groups(), nil
}

// join puts a and b in one group.
func (u *spanUnion) join(a, b arch.Span) {
	ra, rb := u.root(u.add(a)), u.root(u.add(b))
	if ra != rb {
		u.parent[max(ra, rb)] = min(ra, rb)
	}
}

// add returns the index of s, adding it when it is new.
func (u *spanUnion) add(s arch.Span) int {
	if u.index == nil {
		u.index = map[arch.Span]int{}
	}
	if i, ok := u.index[s]; ok {
		return i
	}
	u.index[s] = len(u.spans)
	u.spans = append(u.spans, s)
	u.parent = append(u.parent, len(u.parent))
	return len(u.spans) - 1
}

// root returns the representative of i's group.
func (u *spanUnion) root(i int) int {
	for u.parent[i] != i {
		i = u.parent[i]
	}
	return i
}

// groups returns every group of two or more spans, each sorted, sorted by
// its first span.
func (u *spanUnion) groups() []arch.CloneGroup {
	byRoot := map[int][]arch.Span{}
	for i, s := range u.spans {
		byRoot[u.root(i)] = append(byRoot[u.root(i)], s)
	}
	out := make([]arch.CloneGroup, 0, len(byRoot))
	for _, spans := range byRoot {
		if len(spans) < 2 {
			continue
		}
		slices.SortFunc(spans, compareSpans)
		out = append(out, arch.CloneGroup{Fragments: spans})
	}
	slices.SortFunc(out, func(a, b arch.CloneGroup) int {
		return compareSpans(a.Fragments[0], b.Fragments[0])
	})
	return out
}

// compareSpans orders spans by file, then start and end line.
func compareSpans(a, b arch.Span) int {
	return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Start, b.Start),
		cmp.Compare(a.End, b.End))
}
