// Package tax computes taxes.
package tax

import "example.com/fx/shop/catalog"

// Rate is a tax rate for a product.
type Rate struct{ P catalog.Product }
