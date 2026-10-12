package balance

import (
	"fmt"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// domainIndex holds the domain models of the model's fitness.domains rules
// by the subsystem each describes. They are evidence for judgement slots,
// never answers: a model that imports another may still integrate through a
// published contract (mmi-plan.md §10.0).
type domainIndex struct {
	bySubsystem map[string][]*arch.DomainModel
}

// newDomainIndex indexes the domain models of the rules that name a
// subsystem.
//
// Requires: domains holds one model per fitness.domains rule, in order.
func newDomainIndex(m *arch.Model, domains []arch.DomainModel) *domainIndex {
	d := &domainIndex{bySubsystem: map[string][]*arch.DomainModel{}}
	for i := range m.Fitness.Domains {
		if sub := m.Fitness.Domains[i].Subsystem; sub != "" {
			d.bySubsystem[sub] = append(d.bySubsystem[sub], &domains[i])
		}
	}
	return d
}

// contextEvidence names the domain models that describe a subsystem, which a
// reviewer weighs in judging it a bounded context.
func (d *domainIndex) contextEvidence(sub string) []string {
	var out []string
	for _, model := range d.bySubsystem[sub] {
		out = append(out, "described by domain model "+model.Path)
	}
	return out
}

// integrationEvidence describes how the domain models of a downstream and
// an upstream subsystem refer to each other: an import, and whether the
// imported model exists to be shared.
func (d *domainIndex) integrationEvidence(from, to string) []string {
	var out []string
	for _, a := range d.bySubsystem[from] {
		for _, b := range d.bySubsystem[to] {
			scope, ok := a.ImportScope(b.Path)
			if !ok {
				continue
			}
			out = append(out, fmt.Sprintf("%s's domain model %s imports %s's %s as %s",
				from, a.Path, to, b.Path, scope))
			if b.Shared {
				out = append(out, fmt.Sprintf("%s's domain model %s is shared", to, b.Path))
			}
		}
	}
	return out
}
