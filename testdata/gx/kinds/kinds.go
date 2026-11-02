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

// Counter has a field other types read and write, and a method they call.
type Counter struct {
	N int
}

// Add adds one.
func (c *Counter) Add() { c.N++ }

// Celsius is converted to and used as a type argument.
type Celsius float64

// Box is generic.
type Box[T any] struct {
	V T
}

// Worker uses the types above in its method body only, each in its own way.
type Worker struct{}

// Run constructs a Counter, writes and reads its field, calls its method,
// instantiates Box with Celsius, and converts to Celsius.
func (w Worker) Run() float64 {
	c := &Counter{N: 1}
	c.N = c.N + 1
	c.Add()
	b := Box[Celsius]{}
	_ = b
	return float64(Celsius(2))
}

// Message has a wire format: its tags name its fields in two encodings.
type Message struct {
	ID     string `json:"id" protobuf:"bytes,1,opt,name=message_id"`
	Body   string `json:"body,omitempty"`
	Secret string `json:"-"`
	Plain  string
}
