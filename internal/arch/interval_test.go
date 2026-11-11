package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// A dispatch adds an edge where the static graph has none, so the coupling
// with dispatch bounds the static value from above: here the shell calls
// the core's port statically, and the core reaches the shell's adapter only
// at run time, which closes a cycle the imports cannot show.
func TestCouplingWithDispatch(t *testing.T) {
	t.Parallel()
	p := arch.Program{
		Packages: []arch.Package{{Path: "m/core"}, {Path: "m/shell"}},
		Types: []arch.Type{
			{ID: "m/core.Service", Package: "m/core", Name: "Service"},
			{ID: "m/shell.Adapter", Package: "m/shell", Name: "Adapter"},
		},
		Funcs: []arch.Func{
			{ID: "m/core.Service.Run", Package: "m/core", Name: "Run", Receiver: "m/core.Service"},
		},
		Refs:    []arch.Ref{{From: "m/shell.Adapter", To: "m/core.Service", Weight: 1}},
		Imports: []arch.Import{{From: "m/shell", To: "m/core"}},
		Dispatches: []arch.Dispatch{{
			Caller: "m/core.Service.Run", CallerPackage: "m/core",
			Callee: "m/shell.Adapter.Save", Concrete: "m/shell.Adapter",
		}},
	}
	r := arch.Analyze(&arch.Input{Program: p, Model: arch.Model{Version: 1}})
	if r.CouplingWithDispatch == nil {
		t.Fatal("no coupling with dispatch for a program that carries dispatches")
	}
	static, dynamic := r.Coupling.Packages, r.CouplingWithDispatch.Packages
	if static.MaxCycleGroup != 0 || dynamic.MaxCycleGroup != 2 {
		t.Errorf("largest package cycle group %d static, %d with dispatch; expected 0 and 2",
			static.MaxCycleGroup, dynamic.MaxCycleGroup)
	}
	if dynamic.CCD < static.CCD || r.CouplingWithDispatch.Types.CCD < r.Coupling.Types.CCD {
		t.Errorf("CCD fell with edges added: %+v, %+v", r.Coupling, *r.CouplingWithDispatch)
	}
	p.Dispatches = nil
	if r := arch.Analyze(
		&arch.Input{Program: p, Model: arch.Model{Version: 1}},
	); r.CouplingWithDispatch != nil {
		t.Error("coupling with dispatch for a program without dispatches")
	}
}
