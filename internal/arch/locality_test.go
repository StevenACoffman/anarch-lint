package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

func TestLocalityAndDegree(t *testing.T) {
	t.Parallel()
	subsystemOf := map[string]string{"m/a": "A", "m/a2": "A", "m/b": "B"}
	moduleOf := map[string]string{"m/a": "m", "m/a2": "m", "m/b": "m", "n/c": "n"}
	at := func(file string, line int, pkg, fn, typ string) arch.Site {
		return arch.Site{File: file, Line: line, Package: pkg, Func: fn, Type: typ}
	}
	cases := map[string]struct {
		sites    []arch.Site
		locality arch.Locality
		degree   int
	}{
		"one function": {
			sites:    []arch.Site{at("a.go", 1, "m/a", "f", ""), at("a.go", 2, "m/a", "f", "")},
			locality: arch.LocalityFunction, degree: 2,
		},
		"a type whose methods span two files": {
			sites: []arch.Site{
				at("a.go", 1, "m/a", "T.M", "T"), at("b.go", 9, "m/a", "T.N", "T"),
				at("a.go", 5, "m/a", "", "T"),
			},
			locality: arch.LocalityType, degree: 3, // two methods and the declaration
		},
		"a file holding two types": {
			sites: []arch.Site{
				at("a.go", 1, "m/a", "T.M", "T"), at("a.go", 9, "m/a", "U.M", "U"),
				at("a.go", 20, "m/a", "f", ""),
			},
			locality: arch.LocalityFile, degree: 3,
		},
		"two files of a package": {
			sites:    []arch.Site{at("a.go", 1, "m/a", "f", ""), at("b.go", 1, "m/a", "g", "")},
			locality: arch.LocalityPackage, degree: 2,
		},
		"two packages of a subsystem": {
			sites:    []arch.Site{at("a.go", 1, "m/a", "f", ""), at("c.go", 1, "m/a2", "g", "")},
			locality: arch.LocalitySubsystem, degree: 2,
		},
		"two subsystems of a module": {
			sites: []arch.Site{
				at("a.go", 1, "m/a", "f", ""), at("c.go", 1, "m/a2", "g", ""),
				at("b.go", 1, "m/b", "h", ""),
			},
			locality: arch.LocalitySystem, degree: 2,
		},
		"two modules": {
			sites:    []arch.Site{at("a.go", 1, "m/a", "f", ""), at("c.go", 1, "n/c", "g", "")},
			locality: arch.LocalityExternal, degree: 2,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			l := arch.LocalityOf(tc.sites, subsystemOf, moduleOf)
			equals(t, tc.locality, l)
			equals(t, tc.degree, arch.DegreeOf(tc.sites, l, subsystemOf, moduleOf))
		})
	}
}

func TestLocalityText(t *testing.T) {
	t.Parallel()
	for l := arch.LocalityFunction; l <= arch.LocalityExternal; l++ {
		text, err := l.MarshalText()
		ok(t, err)
		var back arch.Locality
		ok(t, back.UnmarshalText(text))
		equals(t, l, back)
	}
	_, err := arch.Locality(9).MarshalText()
	equals(t, arch.EINVALID, arch.ErrorCode(err))
}
