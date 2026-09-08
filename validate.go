package cidrgen

import (
	"fmt"
	"net/netip"
	"sort"
)

// validateAllocated verifies that every entry is fully contained within parent
// and that no two entries overlap. Entries must already be parsed and canonical.
// It returns the entries sorted ascending by start address (ties broken by
// shorter prefix first), ready for the first-fit scan.
func validateAllocated(parent netip.Prefix, allocated []netip.Prefix) ([]netip.Prefix, error) {
	pStart, pEnd := prefixRange(parent)

	sorted := make([]netip.Prefix, len(allocated))
	copy(sorted, allocated)
	sort.Slice(sorted, func(i, j int) bool {
		si, sj := addrToU32(sorted[i].Addr()), addrToU32(sorted[j].Addr())
		if si != sj {
			return si < sj
		}
		return sorted[i].Bits() < sorted[j].Bits()
	})

	var prevEnd uint64
	havePrev := false
	for _, p := range sorted {
		s, e := prefixRange(p)
		if s < pStart || e > pEnd {
			return nil, fmt.Errorf("%w: %s not within %s", ErrOutOfParent, p, parent)
		}
		if havePrev && s <= prevEnd {
			return nil, fmt.Errorf("%w: %s", ErrOverlappingInput, p)
		}
		prevEnd = e
		havePrev = true
	}
	return sorted, nil
}
