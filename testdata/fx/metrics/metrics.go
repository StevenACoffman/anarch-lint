// Package metrics holds functions with known size and complexity.
package metrics

// Counter has two unrelated halves, so its LCOM4 is 2.
type Counter struct {
	n, m int
	name string
}

// Inc touches n only.
func (c *Counter) Inc() { c.n++ }

// Both touches n and m.
func (c *Counter) Both() int { return c.n + c.m }

// Name touches name only.
func (c *Counter) Name() string { return c.name }

// Straight has no branches.
func Straight() int {
	x := 1
	return x
}

// Branchy has an if with &&, a for, a range and a three-clause switch.
func Branchy(xs []int, a, b bool) int {
	total := 0
	if a && b {
		total++
	}
	for i := 0; i < 3; i++ {
		total += i
	}
	for _, x := range xs {
		switch {
		case x > 10:
			total += 10
		case x > 5 || x < -5:
			total += 5
		default:
			total += x
		}
	}
	return total
}

// Nested nests four levels deep.
func Nested(xs [][]int) int {
	n := 0
	for _, row := range xs {
		for _, x := range row {
			if x > 0 {
				if x%2 == 0 {
					n++
				}
			}
		}
	}
	return n
}
