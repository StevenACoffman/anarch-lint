package arch

import "math/bits"

// bitset is a fixed-size set of small non-negative integers.
type bitset []uint64

func newBitset(n int) bitset { return make(bitset, (n+63)/64) }

func (b bitset) set(i int) { b[i/64] |= 1 << (uint(i) % 64) }

func (b bitset) or(other bitset) {
	for i := range b {
		b[i] |= other[i]
	}
}

func (b bitset) count() int {
	n := 0
	for _, w := range b {
		n += bits.OnesCount64(w)
	}
	return n
}
