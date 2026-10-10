package gopackages

import (
	"go/ast"
	"go/types"
	"slices"
	"strings"
)

// roleDirective is the comment that names a type's pattern role explicitly.
const roleDirective = "//arch:role "

// embedsOf returns the IDs of the in-scope named types tn embeds, as struct
// fields or as embedded interfaces, sorted.
func embedsOf(tn *types.TypeName, inScope map[string]bool) []string {
	var ids []string
	for _, t := range embeddedTypes(tn) {
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := t.(*types.Named); ok && n.Obj().Pkg() != nil && inScope[n.Obj().Pkg().Path()] {
			ids = append(ids, typeID(n.Origin().Obj()))
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// embeddedTypes returns the types a struct embeds as fields, or an interface
// embeds; any other type embeds nothing.
func embeddedTypes(tn *types.TypeName) []types.Type {
	var embedded []types.Type
	switch u := tn.Type().Underlying().(type) {
	case *types.Struct:
		for i := range u.NumFields() {
			if f := u.Field(i); f.Embedded() {
				embedded = append(embedded, f.Type())
			}
		}
	case *types.Interface:
		for i := range u.NumEmbeddeds() {
			embedded = append(embedded, u.EmbeddedType(i))
		}
	}
	return embedded
}

// directiveOf returns the role an "//arch:role NAME" comment gives a type
// spec, or "". The comment may sit on the spec or, for a declaration with one
// spec, on the declaration.
func directiveOf(ts *ast.TypeSpec, d *ast.GenDecl) string {
	docs := []*ast.CommentGroup{ts.Doc}
	if len(d.Specs) == 1 {
		docs = append(docs, d.Doc)
	}
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		for _, c := range doc.List {
			if name, ok := strings.CutPrefix(c.Text, roleDirective); ok {
				return strings.TrimSpace(name)
			}
		}
	}
	return ""
}

// implementations records, for every concrete in-scope type, the in-scope
// interfaces it or a pointer to it implements. Generic types are left out.
func (x *extractor) implementations() {
	ifaces := x.roleInterfaces()
	for _, id := range sortedKeys(x.named) {
		tn := x.named[id]
		if types.IsInterface(tn.Type()) || generic(tn) {
			continue
		}
		t := &x.prog.Types[x.types[id]]
		for _, iface := range ifaces {
			it, _ := iface.Type().Underlying().(*types.Interface)
			if types.Implements(tn.Type(), it) ||
				types.Implements(types.NewPointer(tn.Type()), it) {
				t.Implements = append(t.Implements, typeID(iface))
			}
		}
	}
}

// roleInterfaces returns the in-scope interfaces that can mark a role, sorted:
// those with methods, since every type implements an empty interface, and
// that are not generic.
func (x *extractor) roleInterfaces() []*types.TypeName {
	var ifaces []*types.TypeName
	for _, id := range sortedKeys(x.named) {
		tn := x.named[id]
		if it, ok := tn.Type().Underlying().(*types.Interface); ok && it.NumMethods() > 0 &&
			!generic(tn) {
			ifaces = append(ifaces, tn)
		}
	}
	return ifaces
}

// generic reports whether tn declares type parameters.
func generic(tn *types.TypeName) bool {
	n, ok := tn.Type().(*types.Named)
	return ok && n.TypeParams().Len() > 0
}

// sortedKeys returns the keys of m in sorted order.
func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	})
}
