package plantuml_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/plantuml"
)

const shop = `@startuml
skinparam componentStyle rectangle
[Web] <<example.com/shop/web/...>> as web
component "Orders" <<example.com/shop/orders/...>> <<example.com/shop/api>> as orders
package "Catalog" <<example.com/shop/catalog/...>>
note right of web: the storefront
web --> orders
[Web] -[#blue]-> "Catalog"
"Catalog" <.. orders
@enduml
`

func equals(t *testing.T, exp, act any) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf("\nexpected: %#v\nactual:   %#v", exp, act)
	}
}

// TestParse: components with their stereotypes and aliases, and arrows in
// either direction and style, become subsystems and isolations that a model
// decodes.
func TestParse(t *testing.T) {
	t.Parallel()
	d, err := plantuml.Parse(strings.NewReader(shop))
	if err != nil {
		t.Fatal(err)
	}
	m := d.Model("shop.puml")
	equals(t, []arch.Subsystem{
		{Name: "Web", Packages: []string{"example.com/shop/web/..."}},
		{Name: "Orders", Packages: []string{"example.com/shop/orders/...", "example.com/shop/api"}},
		{Name: "Catalog", Packages: []string{"example.com/shop/catalog/..."}},
	}, m.Subsystems)
	equals(t, arch.Isolation{
		From: "example.com/shop/web/...",
		MayImport: []string{
			"example.com/shop/web/...", "example.com/shop/catalog/...",
			"example.com/shop/orders/...", "example.com/shop/api",
		},
		Reason: "shop.puml draws what Web may depend on",
	}, m.Isolations[0])
	equals(t, 4, len(m.Isolations))
	equals(t, true, m.Fitness.RequireAssigned)
	var buf bytes.Buffer
	if err := arch.WriteJSON(&buf, &m); err != nil {
		t.Fatal(err)
	}
	if _, err := arch.DecodeModel(&buf); err != nil {
		t.Fatalf("the model does not decode: %v", err)
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		src, message string
	}{
		"a component with no package": {
			src:     "[Web]\n",
			message: `diagram line 1: component "Web" names no package in a <<stereotype>>`,
		},
		"an arrow to nothing": {
			src:     "[Web] <<w/...>>\n[Web] --> [Nowhere]\n",
			message: `diagram: an arrow joins "Web" and "Nowhere", which are not both components`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := plantuml.Parse(strings.NewReader(tc.src))
			equals(t, tc.message, arch.ErrorMessage(err))
		})
	}
}
