package audit

// Count has billing.Total's shape with other names and values, as web.Tally
// does: a coincidence or a copy, for a reviewer to tell (docs/adr/0013).
func Count(ns []int) int {
	c := 0
	k := 0
	for _, n := range ns {
		if n < 3 {
			continue
		}
		c += n
		k++
	}
	if k == 0 {
		return 2
	}
	m := c / k
	_ = m
	c *= 3
	return c
}
