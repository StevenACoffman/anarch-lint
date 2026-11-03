package balance

import (
	"fmt"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// crossingCheck finds the untested crossings of a change: what it reads of
// the input, and the subsystem of each package.
type crossingCheck struct {
	in          *Input
	p           *arch.Program
	c           arch.Containers
	subsystemOf func(pkg string) string
}

// untestedCrossings returns a candidate for each function the change set
// adds or changes that calls into another subsystem and that no test runs,
// by the coverage profile: a new integration point nothing checks
// (IMPROVEMENTS.md §6.4). It needs both a change set and a profile; a
// function the profile does not instrument says nothing either way.
func (s *crossingCheck) untestedCrossings() []Candidate {
	if s.in.Changed == nil || s.in.Coverage == nil {
		return nil
	}
	covered := map[string]arch.FuncCoverage{}
	for _, fc := range s.in.Coverage.Funcs {
		covered[fc.Func] = fc
	}
	crossing := s.crossingCalls()
	var out []Candidate
	for i := range s.p.Funcs {
		fn := &s.p.Funcs[i]
		call, fc := crossing[fn.ID], covered[fn.ID]
		if call == nil || fc.Statements == 0 || fc.Covered > 0 || !s.in.Changed.Touches(&fn.Span) {
			continue
		}
		out = append(out, fingerprinted(&Candidate{
			Rule:    arch.RuleUntestedCrossing,
			Subject: fn.ID,
			Object:  s.subsystemOf(arch.FuncPackage(call.Callee)),
			Message: fmt.Sprintf("%s, which the change adds or modifies, calls into subsystem %s, "+
				"and no test runs it", fn.ID, s.subsystemOf(arch.FuncPackage(call.Callee))),
			Fix: "test the call across the boundary before relying on it",
			Evidence: []string{fmt.Sprintf("calls %s at %s:%d; coverage 0 of %d statements",
				call.Callee, call.Site.File, call.Site.Line, fc.Statements)},
			Site: &call.Site,
		}))
	}
	return out
}

// crossingCalls maps each function to its first call into another
// subsystem under analysis.
func (s *crossingCheck) crossingCalls() map[string]*arch.Call {
	out := map[string]*arch.Call{}
	for i := range s.p.Facts.Calls {
		c := &s.p.Facts.Calls[i]
		callee := arch.FuncPackage(c.Callee)
		if _, inScope := s.c.Module(callee); !inScope || out[c.Site.Func] != nil {
			continue
		}
		if s.subsystemOf(callee) != s.subsystemOf(c.Site.Package) {
			out[c.Site.Func] = c
		}
	}
	return out
}
