package arch_test

import (
	"bytes"
	"cmp"
	"slices"
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// programGen draws a program of up to nine types in three packages, with
// references between them and the imports those references need, sorted as
// [arch.Program] requires.
func programGen() *rapid.Generator[arch.Program] {
	return rapid.Custom(func(t *rapid.T) arch.Program {
		pkgs := []string{"m/a", "m/b", "m/c"}
		var p arch.Program
		for _, pkg := range pkgs {
			p.Packages = append(p.Packages, arch.Package{Path: pkg, Name: pkg[2:], Module: "m"})
		}
		names := []string{"X", "Y", "Z"}
		for _, pkg := range pkgs {
			for _, n := range names {
				p.Types = append(p.Types, arch.Type{
					ID: pkg + "." + n, Package: pkg, Name: n, Kind: arch.KindStruct,
				})
			}
		}
		ids := make([]string, len(p.Types))
		for i := range p.Types {
			ids[i] = p.Types[i].ID
		}
		seen := map[[2]string]bool{}
		imports := map[[2]string]bool{}
		for range rapid.IntRange(0, 25).Draw(t, "refs") {
			from := rapid.SampledFrom(ids).Draw(t, "from")
			to := rapid.SampledFrom(ids).Draw(t, "to")
			if from == to || seen[[2]string{from, to}] {
				continue
			}
			seen[[2]string{from, to}] = true
			p.Refs = append(p.Refs, arch.Ref{
				From: from, To: to, Weight: rapid.IntRange(1, 4).Draw(t, "weight"),
				Kinds: []arch.RefKind{arch.RefField},
			})
			if fp, tp := from[:3], to[:3]; fp != tp {
				imports[[2]string{fp, tp}] = true
			}
		}
		slices.SortFunc(p.Refs, func(a, b arch.Ref) int {
			return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
		})
		for k := range imports {
			p.Imports = append(p.Imports, arch.Import{From: k[0], To: k[1]})
		}
		slices.SortFunc(p.Imports, func(a, b arch.Import) int {
			return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
		})
		return p
	})
}

// determinismModel puts the packages in two subsystems and two layers, so
// every measure has something to compute.
func determinismModel() arch.Model {
	return arch.Model{
		Version: 1,
		Subsystems: []arch.Subsystem{
			{Name: "ab", Packages: []string{"m/a", "m/b"}, Interface: []string{"m/a"}},
			{Name: "c", Packages: []string{"m/c"}},
		},
		Layers: []arch.Layer{
			{Name: "top", Packages: []string{"m/a"}},
			{Name: "bottom", Packages: []string{"m/b", "m/c"}},
		},
		Thresholds: arch.Thresholds{MaxTypeCycleGroup: 2},
	}
}

// The same input gives byte-identical output on every run: nothing iterates
// a map into the report.
func TestAnalyzeIsDeterministic(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		in := arch.Input{Program: programGen().Draw(t, "program"), Model: determinismModel()}
		var first, second bytes.Buffer
		if err := arch.WriteJSON(&first, arch.Analyze(&in)); err != nil {
			t.Fatal(err)
		}
		for range 3 {
			second.Reset()
			if err := arch.WriteJSON(&second, arch.Analyze(&in)); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first.Bytes(), second.Bytes()) {
				t.Fatalf("two runs differ:\n%s\n%s", first.String(), second.String())
			}
		}
	})
}

// The graph measures do not depend on the order references come in.
func TestGraphMeasuresIgnoreReferenceOrder(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		p := programGen().Draw(t, "program")
		reversed := p
		reversed.Refs = slices.Clone(p.Refs)
		slices.Reverse(reversed.Refs)
		a := arch.Analyze(&arch.Input{Program: p, Model: determinismModel()})
		b := arch.Analyze(&arch.Input{Program: reversed, Model: determinismModel()})
		equalsRapid(t, a.ComponentRank, b.ComponentRank)
		equalsRapid(t, a.Coupling, b.Coupling)
		equalsRapid(t, a.Ordering, b.Ordering)
		equalsRapid(t, a.Cohesion, b.Cohesion)
	})
}

// equalsRapid fails a property when exp and act differ.
func equalsRapid(t *rapid.T, exp, act any) {
	t.Helper()
	var e, a bytes.Buffer
	if err := arch.WriteJSON(&e, exp); err != nil {
		t.Fatal(err)
	}
	if err := arch.WriteJSON(&a, act); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(e.Bytes(), a.Bytes()) {
		t.Fatalf("\nexpected: %s\nactual:   %s", e.String(), a.String())
	}
}
