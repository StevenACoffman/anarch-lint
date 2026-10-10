package gopackages_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fzipp/gocyclo"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/gopackages"
)

// fixtureDir is the fixture module, relative to this package's directory.
const fixtureDir = "../../testdata/fx"

func loadFixture(t *testing.T) arch.Program {
	t.Helper()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(
		context.Background(),
		gopackages.Config{Dir: fixtureDir, Patterns: []string{"./..."}},
	)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return p
}

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

func TestLoadFixtureGraphFacts(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)

	var pkgs []string
	for _, pkg := range p.Packages {
		pkgs = append(pkgs, pkg.Path)
	}
	equals(t, []string{
		"example.com/fx/app", "example.com/fx/billing/invoice", "example.com/fx/billing/tax",
		"example.com/fx/gen", "example.com/fx/metrics", "example.com/fx/roles",
		"example.com/fx/service", "example.com/fx/shop/catalog",
	}, pkgs)

	refs := map[string]int{}
	for _, r := range p.Refs {
		refs[r.From+" -> "+r.To] = r.Weight
	}
	const inv, cat = "example.com/fx/billing/invoice.", "example.com/fx/shop/catalog."
	equals(t, map[string]int{
		inv + "Invoice -> " + inv + "Line":                             1,
		inv + "Line -> " + inv + "Invoice":                             1,
		inv + "Total -> " + inv + "Invoice":                            1,
		cat + "Category -> " + cat + "Item":                            1,
		cat + "Item -> " + cat + "Product":                             2, // result type and composite literal
		cat + "Item -> " + inv + "Invoice":                             2,
		cat + "Product -> " + cat + "Category":                         1,
		"example.com/fx/billing/tax.Rate -> " + cat + "Product":        1,
		"example.com/fx/roles.Customer -> example.com/fx/roles.Entity": 1, // embedding
	}, refs)

	for _, imp := range p.Imports {
		if imp.From == "example.com/fx/billing/tax" {
			equals(t, arch.Position{File: "billing/tax/tax.go", Line: 4}, imp.Pos)
		}
	}
}

func TestLoadSkipsGeneratedFiles(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)
	for _, pkg := range p.Packages {
		if pkg.Path == "example.com/fx/gen" {
			equals(t, []string{"gen/doc.go"}, pkg.Files)
			equals(t, 1, pkg.Generated)
		}
	}
	for _, typ := range p.Types {
		if typ.Package == "example.com/fx/gen" {
			t.Fatalf("generated type %s was extracted", typ.ID)
		}
	}
}

func TestLoadFunctionMetrics(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)
	type metrics struct {
		Complexity arch.Complexity
		Statements int
		MaxNesting int
	}
	cases := map[string]metrics{
		"Straight": {
			Complexity: arch.Complexity{Plain: 1, Modified: 1, Extended: 1},
			Statements: 2,
			MaxNesting: 1,
		},
		"Branchy": {
			Complexity: arch.Complexity{Plain: 6, Modified: 5, Extended: 8},
			Statements: 13,
			MaxNesting: 3,
		},
		"Nested": {
			Complexity: arch.Complexity{Plain: 5, Modified: 5, Extended: 5},
			Statements: 7,
			MaxNesting: 5,
		},
	}
	got := map[string]metrics{}
	for _, fn := range p.Funcs {
		if fn.Package == "example.com/fx/metrics" && fn.Receiver == "" {
			got[fn.Name] = metrics{
				Complexity: fn.Complexity,
				Statements: fn.Statements,
				MaxNesting: fn.MaxNesting,
			}
		}
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			equals(t, want, got[name])
		})
	}
}

// The Extended variant must equal gocyclo's value for every function of the
// fixture: gocyclo is the oracle mmi-ref-plan.md §3 names for it.
func TestExtendedComplexityMatchesGocyclo(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)
	oracle := map[string]int{}
	fset := token.NewFileSet()
	for _, pkg := range p.Packages {
		for _, file := range pkg.Files {
			f, err := parser.ParseFile(fset, filepath.Join(fixtureDir, file), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", file, err)
			}
			for _, decl := range f.Decls {
				if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil {
					oracle[pkg.Path+"."+fd.Name.Name] = gocyclo.Complexity(fd)
				}
			}
		}
	}
	for _, fn := range p.Funcs {
		if want, ok := oracle[fn.ID]; ok && fn.Complexity.Extended != want {
			t.Errorf("%s: Extended = %d, gocyclo = %d", fn.ID, fn.Complexity.Extended, want)
		}
	}
}

func TestLoadSizesAndCohesionFacts(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)
	for _, typ := range p.Types {
		if typ.ID == "example.com/fx/metrics.Counter" {
			equals(t, []string{"n", "m", "name"}, typ.Fields)
			// 4 lines of declaration plus three one-line methods.
			equals(t, 7, typ.Lines)
		}
	}
	uses := map[string][]string{}
	for _, fn := range p.Funcs {
		if fn.Receiver == "example.com/fx/metrics.Counter" {
			uses[fn.Name] = fn.Uses
		}
	}
	equals(t, map[string][]string{"Inc": {"n"}, "Both": {"m", "n"}, "Name": {"name"}}, uses)
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs go list; skipped with -short")
	}
	cases := map[string]struct {
		pattern string
		code    string
	}{
		"directory that does not exist": {pattern: "./nosuchdir/...", code: arch.EINVALID},
		"pattern that matches nothing": {
			pattern: "example.com/fx/nosuch/...",
			code:    arch.ENOTFOUND,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := gopackages.Load(context.Background(), gopackages.Config{
				Dir: fixtureDir, Patterns: []string{tc.pattern},
			})
			equals(t, tc.code, arch.ErrorCode(err))
		})
	}
}

func TestLoadRoleMarkers(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)
	type markers struct {
		Embeds, Implements []string
		Directive          string
	}
	got := map[string]markers{}
	for _, typ := range p.Types {
		if typ.Package == "example.com/fx/roles" {
			got[typ.Name] = markers{
				Embeds:     typ.Embeds,
				Implements: typ.Implements,
				Directive:  typ.Directive,
			}
		}
	}
	const roles = "example.com/fx/roles."
	equals(t, map[string]markers{
		"Entity":     {},
		"Repository": {},
		"Customer":   {Embeds: []string{roles + "Entity"}},
		// Find has a pointer receiver: the pointer implements Repository.
		"SQLStore": {Implements: []string{roles + "Repository", "example.com/fx/service.Finder"}},
		"Handler":  {Directive: "handler"},
		"Untagged": {},
	}, got)
}

func TestLoadInjections(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)
	equals(t, []arch.Injection{{
		Caller:        "example.com/fx/app",
		Callee:        "example.com/fx/service.Lookup",
		CalleePackage: "example.com/fx/service",
		Concrete:      "example.com/fx/roles.SQLStore",
		Pos:           arch.Position{File: "app/app.go", Line: 11},
	}}, p.Injections)
}
