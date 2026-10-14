// Package kinds holds one type reference of each kind, for the loader's
// reference-kind test.
package kinds

// Base is embedded.
type Base struct{}

// Item is used in every other way.
type Item struct{}

// Spec refers to Item in its definition.
type Spec []Item

// Holder embeds Base and has a field of type Item.
type Holder struct {
	Base

	Item Item
}

// Reader refers to Item in a method signature.
type Reader interface {
	Read() Item
}

// Service refers to Base and Item in a method signature, and to Item and Spec
// in the method's body.
type Service struct{}

// Make returns an item.
func (s *Service) Make(b Base) Item {
	var spec Spec
	_, _ = b, spec
	return Item{}
}
