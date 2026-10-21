package calls

// list is generic, so FullName writes its methods with its type parameter.
type list[T any] struct{ xs []T }

// push appends x.
func (l *list[T]) push(x T) { l.xs = append(l.xs, x) }

// Generic calls a method of a generic type: a rule names it by the type
// alone, (*example.com/gx/calls.list).push.
func Generic() {
	var l list[int]
	l.push(1)
}
