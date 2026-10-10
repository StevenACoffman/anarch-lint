// Package catalog lists products.
package catalog

import "example.com/fx/billing/invoice"

// Product is something to sell.
type Product struct{ cat *Category }

// Category groups items.
type Category struct{ items []Item }

// Item is an entry of a category.
type Item struct{}

// Product returns the product of the item.
func (i Item) Product() Product { return Product{} }

// Bill bills the item.
func (i Item) Bill() invoice.Invoice { return invoice.Invoice{} }
