package cidrgen

import (
	"fmt"
	"net/netip"
)

// firstFit returns the lowest-address, correctly-aligned prefix of length bits
// that fits within parent without overlapping any entry in sorted, which must be
// the output of validateAllocated (ascending by start, within parent, non-
// overlapping). bits must already be validated as strictly longer than
// parent.Bits() and no greater than 32.
func firstFit(parent netip.Prefix, sorted []netip.Prefix, bits int) (netip.Prefix, error) {
	pStart, pEnd := prefixRange(parent)
	blockSize := uint64(1) << (32 - bits)

	// candidate stays aligned to blockSize: pStart is aligned to the parent's
	// (larger) block, and every re-alignment below rounds up to a multiple of
	// blockSize.
	candidate := pStart

	for _, a := range sorted {
		aStart, aEnd := prefixRange(a)
		if candidate+blockSize-1 < aStart {
			return netip.PrefixFrom(u32ToAddr(uint32(candidate)), bits), nil
		}
		next := aEnd + 1
		if next <= candidate {
			continue
		}
		if r := next % blockSize; r != 0 {
			next += blockSize - r
		}
		candidate = next
	}

	if candidate+blockSize-1 <= pEnd {
		return netip.PrefixFrom(u32ToAddr(uint32(candidate)), bits), nil
	}
	return netip.Prefix{}, fmt.Errorf("%w: /%d within %s", ErrNoSpace, bits, parent)
}
