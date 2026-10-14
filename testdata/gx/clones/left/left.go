// Package left totals orders; its loops are copied in package right.
package left

// Total sums the positive quantities of each line, capped at a limit, and
// counts the lines it skipped.
func Total(prices, quantities []int, limit int) (total, skipped int) {
	if len(prices) != len(quantities) {
		return 0, len(prices)
	}
	for i := range prices {
		q := quantities[i]
		if q <= 0 {
			skipped++
			continue
		}
		line := prices[i] * q
		if line > limit {
			line = limit
		}
		if total+line > limit*len(prices) {
			skipped++
			continue
		}
		total += line
	}
	for i := len(prices) - 1; i >= 0; i-- {
		switch {
		case quantities[i] > limit:
			total -= quantities[i] - limit
		case quantities[i] < 0 && prices[i] > 0:
			skipped--
		default:
			total += quantities[i] % 3
		}
	}
	for k := 0; k+1 < len(prices); k += 2 {
		if prices[k] == prices[k+1] {
			skipped++
		} else if prices[k] > prices[k+1] {
			total += prices[k] - prices[k+1]
		} else {
			total -= quantities[k+1] - quantities[k]
		}
	}
	if total < 0 || skipped < 0 {
		total, skipped = 0, 0
	}
	return total, skipped
}
