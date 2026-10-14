// Package right reports on stock; its loops are a copy of package left's.
package right

import "fmt"

// Report describes the stock value, capped at a ceiling, after a header.
func Report(costs, counts []int, ceiling int) string {
	header := fmt.Sprintf("%d items", len(costs))
	value, ignored := 0, 0
	for j := range costs {
		n := counts[j]
		if n <= 0 {
			ignored++
			continue
		}
		worth := costs[j] * n
		if worth > ceiling {
			worth = ceiling
		}
		if value+worth > ceiling*len(costs) {
			ignored++
			continue
		}
		value += worth
	}
	for j := len(costs) - 1; j >= 0; j-- {
		switch {
		case counts[j] > ceiling:
			value -= counts[j] - ceiling
		case counts[j] < 0 && costs[j] > 0:
			ignored--
		default:
			value += counts[j] % 3
		}
	}
	for k := 0; k+1 < len(costs); k += 2 {
		if costs[k] == costs[k+1] {
			ignored++
		} else if costs[k] > costs[k+1] {
			value += costs[k] - costs[k+1]
		} else {
			value -= counts[k+1] - counts[k]
		}
	}
	if value < 0 || ignored < 0 {
		value, ignored = 0, 0
	}
	return fmt.Sprintf("%s: %d, %d ignored", header, value, ignored)
}
