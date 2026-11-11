package gopackages_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/gopackages"
)

// factsFixtureDir is the connascence fixture module.
const factsFixtureDir = "../../testdata/cx"

func loadFacts(t *testing.T) arch.Program {
	t.Helper()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(), gopackages.Config{
		Dir: factsFixtureDir, Patterns: []string{"./..."}, Facts: true,
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return p
}

// where is a fact's file and line, the way the tests below name sites.
func where(s arch.Site) string { return s.File + ":" + strconv.Itoa(s.Line) }

func TestLoadFacts(t *testing.T) {
	t.Parallel()
	p := loadFacts(t)
	f := p.Facts
	cases := map[string]struct {
		got, want any
	}{
		"literal shadowing a typed constant": {
			got: func() []string {
				var out []string
				for _, c := range f.ConstExprs {
					if c.Value == `"admin"` && c.Literal {
						out = append(out, where(c.Site))
					}
				}
				return out
			}(),
			want: []string{
				"billing/invoice.go:26",
				"cmd/server/main.go:10",
				"web/admin.go:17",
				"web/admin.go:36",
			},
		},
		"regexp pattern argument": {
			got: func() []string {
				var out []string
				for _, c := range f.ConstExprs {
					if c.Callee == "regexp.MustCompile" {
						out = append(out, where(c.Site)+" "+c.Value)
					}
				}
				return out
			}(),
			want: []string{
				`billing/invoice.go:16 "^[A-Z]{3}-[0-9]+$"`,
				`web/admin.go:14 "^[A-Z]{3}-[0-9]+$"`,
			},
		},
		"constants and aliases": {
			got: func() map[string]bool {
				out := map[string]bool{}
				for _, c := range f.Constants {
					out[c.ID] = c.Alias
				}
				return out
			}(),
			want: map[string]bool{
				"example.com/cx/auth.RoleAdmin":      false,
				"example.com/cx/auth.RoleUser":       false,
				"example.com/cx/auth.RoleDefault":    true,
				"example.com/cx/auth.Low":            false,
				"example.com/cx/auth.Mid":            false,
				"example.com/cx/auth.High":           false,
				"example.com/cx/auth.DefaultPort":    false,
				"example.com/cx/billing.DefaultPort": false,
			},
		},
		"writes to an exported variable": {
			got: func() []string {
				var out []string
				for _, g := range f.GlobalUses {
					if g.Write {
						out = append(out, g.Var+" "+g.Site.Func)
					}
				}
				return out
			}(),
			want: []string{
				"example.com/cx/auth.Sessions example.com/cx/billing.Login",
				"example.com/cx/auth.Sessions example.com/cx/web.Reset",
			},
		},
		"unkeyed literal": {
			got: f.UnkeyedLits,
			want: []arch.UnkeyedLit{
				{
					Type:    "example.com/cx/auth.Credentials",
					Package: "example.com/cx/auth",
					Site: arch.Site{
						File:    "billing/invoice.go",
						Line:    64,
						Package: "example.com/cx/billing",
						Func:    "example.com/cx/billing.Login",
					},
				},
			},
		},
		"switch without default": {
			got:  len(f.Switches) == 1 && !f.Switches[0].HasDefault,
			want: true,
		},
		"index into a call result": {
			got: []string{
				f.ConstIndexes[0].Origin + "[" + f.ConstIndexes[0].Index + "]",
				f.ConstIndexes[1].Index,
			},
			want: []string{"strings.Split[0]", "1"},
		},
		"one nil, nil return": {got: where(f.NilNils[0].Site), want: "billing/invoice.go:56"},
		"guard on a field": {
			got:  f.Guards[0].Field + " " + f.Guards[0].Method,
			want: "sent example.com/cx/state.Email.SetSubject",
		},
		"lock calls": {
			got: func() []string {
				var out []string
				for _, l := range f.LockCalls {
					out = append(out, l.Site.Func+" "+l.Method)
				}
				return out
			}(),
			want: []string{
				"example.com/cx/auth.Store.Put Lock", "example.com/cx/auth.Store.Put Unlock",
				"example.com/cx/auth.Store.Get Lock",
			},
		},
		"deferred unlock": {got: f.LockCalls[1].Deferred, want: true},
		"algorithm calls": {
			got:  []string{f.AlgoCalls[0].Algorithm, where(f.AlgoCalls[1].Site)},
			want: []string{"crypto/sha256", "web/admin.go:45"},
		},
		"linkname target package": {
			got:  f.Linknames[0].Package + " " + f.Linknames[0].Local,
			want: "example.com/cx/auth Steal",
		},
		"unsafe conversion of a foreign type": {
			got:  f.UnsafeConvs[0].Type,
			want: "example.com/cx/auth.Secret",
		},
		"reflect by constant name": {got: f.ReflectByNames[0].Name, want: "token"},
		"pointer equality": {
			got:  f.PointerEqs[0].Type,
			want: "example.com/cx/state.Email",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			equals(t, tc.want, tc.got)
		})
	}
}

func TestLoadFactsUsesAndPackages(t *testing.T) {
	t.Parallel()
	p := loadFacts(t)
	uses := map[string]int{}
	for _, u := range p.Facts.Uses {
		uses[u.Object+" in "+u.Site.Func] = u.Count
	}
	equals(t, 1, uses["example.com/cx/auth.RoleAdmin in example.com/cx/billing.Describe"])
	equals(t, 1, uses["example.com/cx/billing.Invoice.Role in example.com/cx/billing.Approve"])
	_, local := uses["example.com/cx/auth.Check in example.com/cx/auth.Check"]
	equals(t, false, local)

	names := map[string]string{}
	for _, pkg := range p.Packages {
		names[pkg.Path] = pkg.Name + " " + pkg.Module
	}
	equals(t, "main example.com/cx", names["example.com/cx/cmd/server"])
	equals(t, "auth example.com/cx", names["example.com/cx/auth"])
}

// The two renamed copies of one body hash alike, and nothing else does.
func TestLoadFactsBodyHash(t *testing.T) {
	t.Parallel()
	p := loadFacts(t)
	shape, exact := map[string]string{}, map[string]string{}
	for _, fn := range p.Funcs {
		if fn.BodyHash != "" {
			shape[fn.ID], exact[fn.ID] = fn.BodyHash, fn.ExactHash
		}
	}
	const total = "example.com/cx/billing.Total"
	equals(t, 4, len(shape))
	for _, id := range []string{
		"example.com/cx/web.Tally", "example.com/cx/audit.Count",
		"example.com/cx/state.Total",
	} {
		equals(t, shape[total], shape[id])
	}
	// state.Total is a copy with a comment and a blank line; the others only
	// share the shape.
	equals(t, exact[total], exact["example.com/cx/state.Total"])
	if exact[total] == exact["example.com/cx/web.Tally"] {
		t.Error("a renamed copy hashes as a copy")
	}
}

// Without Facts, the loader does no more than analyze needs.
func TestLoadWithoutFacts(t *testing.T) {
	t.Parallel()
	p := loadFixture(t)
	equals(t, arch.Facts{}, p.Facts)
	for _, fn := range p.Funcs {
		equals(t, "", fn.BodyHash)
	}
}

func TestLoadFactsCalls(t *testing.T) {
	t.Parallel()
	p := loadFacts(t)
	calls := map[string]int{}
	for _, c := range p.Facts.Calls {
		calls[c.Callee+" from "+c.Site.Func] = c.Count
	}
	equals(t, 1, calls["time.Sleep from example.com/cx/state.Wait"])
	equals(t, 1, calls["example.com/cx/auth.Version from example.com/cx/audit.Record"])
	equals(t, 1, calls["(*regexp.Regexp).MatchString from example.com/cx/billing.Valid"])
}

// A use of a field counts the references that write it: a keyed literal and
// an assignment write N, and the right-hand side reads it.
func TestLoadFactsFieldWrites(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(), gopackages.Config{
		Dir: "../../testdata/gx", Patterns: []string{"./kinds"}, Facts: true,
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	const counter = "example.com/gx/kinds.Counter"
	var got []arch.Use
	for _, u := range p.Facts.Uses {
		if u.Object == counter+".N" {
			got = append(got, u)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one use of %s.N, got %v", counter, got)
	}
	equals(t, "example.com/gx/kinds.Worker.Run", got[0].Site.Func)
	equals(t, counter, got[0].Owner)
	equals(t, 3, got[0].Count)
	equals(t, 2, got[0].Writes)
}

// External tests are not loaded, but the names they qualify are read from
// their syntax: memory's test names memory.Store.
func TestLoadFactsTestUses(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(), gopackages.Config{
		Dir: "../../testdata/gx", Patterns: []string{"./wiring/..."}, Facts: true,
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	equals(t, []string{"example.com/gx/wiring/memory.Store"}, p.Facts.TestUses)
}

// An error guard is an if at the top of a body, with no init statement, that
// returns a non-nil error or panics; an early return of a value, an if with
// an init statement and a guard inside a loop are not.
func TestLoadFactsErrorGuards(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(), gopackages.Config{
		Dir: "../../testdata/gx", Patterns: []string{"./guards"}, Facts: true,
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var got []string
	for _, g := range p.Facts.ErrorGuards {
		got = append(got, g.Site.Func+" "+where(g.Site))
	}
	equals(t, []string{
		"example.com/gx/guards.NewMoney guards/guards.go:14",
		"example.com/gx/guards.Money.MustAdd guards/guards.go:22",
	}, got)
}

// The dynamic calls resolve through the call graph to the callees that
// may run: a writer's Write to (*os.File).Write, which the caller passes in,
// and an interface's method to its one implementation. A call through a
// function value nothing supplies resolves to nothing.
func TestLoadDynamicCalls(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(), gopackages.Config{
		Dir: "../../testdata/gx", Patterns: []string{"./dynamic"}, Facts: true, DynamicCalls: true,
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var got []string
	for _, c := range p.Facts.DynamicCalls {
		got = append(got, c.Site.Func+" "+c.Callee+" "+where(c.Site))
	}
	equals(t, []string{
		"example.com/gx/dynamic.Report (*os.File).Write dynamic/dynamic.go:20",
		"example.com/gx/dynamic.Main (example.com/gx/dynamic.sink).Put dynamic/dynamic.go:27",
	}, got)
	var unresolved []string
	for _, s := range p.Facts.UnresolvedCalls {
		unresolved = append(unresolved, s.Func+" "+where(s))
	}
	equals(t, []string{"example.com/gx/dynamic.Each dynamic/dynamic.go:31"}, unresolved)
}

// A test file's calls of package-level functions are read from its source,
// one per caller and callee with a count; a name the file declares, such as
// a local os, hides the import of that name everywhere in the file.
func TestLoadTestCalls(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("loads and type-checks a module; skipped with -short")
	}
	p, err := gopackages.Load(context.Background(), gopackages.Config{
		Dir: "../../testdata/gx", Patterns: []string{"./guards"},
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var got []string
	for _, c := range p.TestCalls {
		got = append(got, fmt.Sprintf("%s calls %s %d at %s", c.Site.Func, c.Callee, c.Count,
			where(c.Site)))
	}
	equals(t, []string{
		"example.com/gx/guards.TestNewMoney calls time.Sleep 2 at guards/guards_test.go:13",
		"example.com/gx/guards.TestNewMoney calls example.com/gx/guards.NewMoney 1 at guards/guards_test.go:16",
	}, got)
}
