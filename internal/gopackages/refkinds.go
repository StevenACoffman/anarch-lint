package gopackages

import (
	"go/ast"
	"go/token"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// region is a span of a declaration whose references all have one kind.
type region struct {
	start, end token.Pos
	kind       arch.RefKind
}

// refRegions splits a type or method declaration into the regions that give
// a reference its kind (see [fieldRegions]); in a method, its receiver and
// signature. A reference outside every region has the declaration's default
// kind: definition for a type, body for a method.
func refRegions(node ast.Node) (regions []region, fallback arch.RefKind) {
	switch d := node.(type) {
	case *ast.TypeSpec:
		switch t := d.Type.(type) {
		case *ast.StructType:
			return fieldRegions(t.Fields, arch.RefField), arch.RefDefinition
		case *ast.InterfaceType:
			return fieldRegions(t.Methods, arch.RefSignature), arch.RefDefinition
		}
	case *ast.FuncDecl:
		return []region{{d.Recv.Pos(), d.Type.End(), arch.RefSignature}}, arch.RefBody
	}
	return nil, arch.RefDefinition
}

// fieldRegions gives each entry of a struct's fields or an interface's
// methods its region: named entries have the kind named, and embedded ones
// are embeddings.
func fieldRegions(list *ast.FieldList, named arch.RefKind) []region {
	out := make([]region, 0, len(list.List))
	for _, f := range list.List {
		kind := named
		if len(f.Names) == 0 {
			kind = arch.RefEmbed
		}
		out = append(out, region{f.Type.Pos(), f.Type.End(), kind})
	}
	return out
}

// kindAt returns the kind of a reference at pos.
func kindAt(regions []region, fallback arch.RefKind, pos token.Pos) arch.RefKind {
	for _, r := range regions {
		if r.start <= pos && pos < r.end {
			return r.kind
		}
	}
	return fallback
}

// addKind adds kind to a reference's kinds, keeping them hardest to cut
// first.
func addKind(r *arch.Ref, kind arch.RefKind) {
	if slices.Contains(r.Kinds, kind) {
		return
	}
	r.Kinds = append(r.Kinds, kind)
	slices.SortFunc(r.Kinds, func(a, b arch.RefKind) int {
		return a.Order() - b.Order()
	})
}
