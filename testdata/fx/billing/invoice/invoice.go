// Package invoice is billing's interface package.
package invoice

// Invoice is a bill with lines.
type Invoice struct{ Lines []Line }

// Line is one line of an invoice.
type Line struct{ Parent *Invoice }

// Total wraps an invoice.
type Total struct{ Of Invoice }
