package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestDeclare: a package naming a subsystem the model lacks makes it; one
// exposing itself joins the interface of the subsystem it names or the
// model gives it; one the model already places is not added twice; the
// model keeps its hash.
func TestDeclare(t *testing.T) {
	t.Parallel()
	m := arch.Model{Version: 1, Subsystems: []arch.Subsystem{
		{Name: "billing", Packages: []string{"m/billing/..."}},
	}}
	declared := m.Declare([]arch.Package{
		{Path: "m/billing/api", Exposes: true},
		{Path: "m/billing/core", Subsystem: "billing"},
		{Path: "m/orders", Subsystem: "orders", Exposes: true},
	})
	equals(t, []arch.Subsystem{
		{
			Name:      "billing",
			Packages:  []string{"m/billing/..."},
			Interface: []string{"m/billing/api"},
		},
		{Name: "orders", Packages: []string{"m/orders"}, Interface: []string{"m/orders"}},
	}, declared.Subsystems)
	equals(t, m.Hash(), declared.Hash())
	equals(t, 1, len(m.Subsystems))
}
