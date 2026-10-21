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

// Every source file no measure reads is listed with the reason: generated
// code, and files the build constraints leave out.
func TestLoadListsExcludedFiles(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)
	equals(t, []arch.ExcludedFile{
		{Path: "gen/gen.go", Package: "example.com/fx/gen", Reason: arch.ExcludedGenerated},
		{
			Path:    "metrics/legacy.go",
			Package: "example.com/fx/metrics",
			Reason:  arch.ExcludedByBuild,
		},
	}, p.Excluded)
}

func TestLoadSkipsGeneratedFiles(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)
	for _, pkg := range p.Packages {
		if pkg.Path == "example.com/fx/gen" {
			equals(t, []string{"gen/doc.go"}, pkg.Files)
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
		Cognitive  int
	}
	// Cognitive complexity by SonarSource's rules: a decision costs 1 plus
	// its nesting, and each run of && or || costs 1.
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
			Cognitive:  7, // if 1, && 1, for 1, range 1, nested switch 2, || 1
		},
		"Nested": {
			Complexity: arch.Complexity{Plain: 5, Modified: 5, Extended: 5},
			Statements: 7,
			MaxNesting: 5,
			Cognitive:  10, // 1 + 2 + 3 + 4 down the nesting
		},
	}
	got := map[string]metrics{}
	for _, fn := range p.Funcs {
		if fn.Package == "example.com/fx/metrics" && fn.Receiver == "" {
			got[fn.Name] = metrics{
				Complexity: fn.Complexity,
				Statements: fn.Statements,
				MaxNesting: fn.MaxNesting,
				Cognitive:  fn.Cognitive,
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

// Each reference records where its uses are: embedded, a field, a type's
// definition, a signature or a method body.
func TestRefKinds(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(),
		gopackages.Config{Dir: "../../testdata/gx", Patterns: []string{"./kinds"}})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := map[string][]arch.RefKind{}
	for _, r := range p.Refs {
		got[r.From+" -> "+r.To] = r.Kinds
	}
	const k = "example.com/gx/kinds."
	equals(t, map[string][]arch.RefKind{
		k + "Holder -> " + k + "Base":  {arch.RefEmbed},
		k + "Holder -> " + k + "Item":  {arch.RefField},
		k + "Spec -> " + k + "Item":    {arch.RefDefinition},
		k + "Reader -> " + k + "Item":  {arch.RefSignature},
		k + "Service -> " + k + "Base": {arch.RefSignature},
		k + "Service -> " + k + "Item": {arch.RefSignature, arch.RefBody},
		k + "Service -> " + k + "Spec": {arch.RefBody},
	}, got)
}

// A call is named by what it calls, not how the file spells it: aliased and
// dot imports of time call time.Now, a local variable named time does not,
// and a method of a generic type is named without its type parameters.
func TestCallsAreResolvedByType(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(), gopackages.Config{
		Dir: "../../testdata/gx", Patterns: []string{"./calls"}, Facts: true,
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	callees := map[string]string{}
	for _, c := range p.Facts.Calls {
		callees[c.Site.Func] = c.Callee
	}
	const pkg = "example.com/gx/calls."
	equals(t, map[string]string{
		pkg + "Aliased":  "time.Now",
		pkg + "Dotted":   "time.Now",
		pkg + "Generic":  "(*example.com/gx/calls.list).push",
		pkg + "Shadowed": "(example.com/gx/calls.stopwatch).Now",
	}, callees)
}

// dupl finds the loop package right copies from package left, renamed, inside
// functions that differ around it: a partial clone.
func TestLoadFindsPartialClones(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(), gopackages.Config{
		Dir: "../../testdata/gx", Patterns: []string{"./clones/..."}, Facts: true,
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	t.Logf("%+v", p.Facts.Clones)
	equals(t, 1, len(p.Facts.Clones))
	files := []string{}
	for _, f := range p.Facts.Clones[0].Fragments {
		files = append(files, f.File)
	}
	equals(t, []string{"clones/left/left.go", "clones/right/right.go"}, files)
}

// A module below the analysis is listed by its go.mod when a pattern's "..."
// would reach it but for its module boundary: only the outermost, and none
// that the go command skips or that no pattern walks into.
func TestLoadListsNestedModules(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	nested := arch.ExcludedFile{
		Path:    "modules/nested/go.mod",
		Package: "example.com/gx/modules/nested",
		Reason:  arch.ExcludedOtherModule,
	}
	tests := []struct {
		name     string
		patterns []string
		want     []arch.ExcludedFile
	}{
		{"recursive pattern", []string{"./modules/..."}, []arch.ExcludedFile{nested}},
		{
			"two patterns reach it once",
			[]string{"./...", "./modules/..."},
			[]arch.ExcludedFile{nested},
		},
		{"package pattern", []string{"./modules"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := gopackages.Load(context.Background(),
				gopackages.Config{Dir: "../../testdata/gx", Patterns: tt.patterns})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			var got []arch.ExcludedFile
			for _, f := range p.Excluded {
				if f.Reason == arch.ExcludedOtherModule {
					got = append(got, f)
				}
			}
			equals(t, tt.want, got)
		})
	}
}

// The imports of a file the build constraints exclude are read from its
// source, for the package-to-package imports the build does not have.
func TestLoadReadsImportsOfExcludedFiles(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(),
		gopackages.Config{Dir: "../../testdata/gx", Patterns: []string{"./platform"}})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	equals(t, []arch.Import{{
		From: "example.com/gx/platform", To: "os", Inactive: true,
		Pos: arch.Position{File: "platform/platform_plan9.go", Line: 4},
	}}, p.InactiveImports)
}
