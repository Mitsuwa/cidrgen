package cidrgen

import (
	"fmt"
	"net/netip"
)

// parsePrefix parses s as an IPv4 CIDR and returns it in canonical (masked) form,
// so that a value such as "10.0.0.5/24" becomes "10.0.0.0/24".
func parsePrefix(s string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("%w: %q: %v", ErrInvalidPrefix, s, err)
	}
	if !p.Addr().Is4() {
		return netip.Prefix{}, fmt.Errorf("%w: %q: not an IPv4 CIDR", ErrInvalidPrefix, s)
	}
	return p.Masked(), nil
}

// addrToU32 converts an IPv4 address to its uint32 representation.
func addrToU32(a netip.Addr) uint32 {
	b := a.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// u32ToAddr converts a uint32 back to an IPv4 address.
func u32ToAddr(v uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
}

// prefixRange returns the inclusive [start, end] address range covered by p,
// which must be a canonical IPv4 prefix. The range is returned as uint64 so the
// end of 0.0.0.0/0 (and arithmetic just past the top of the space) does not
// overflow.
func prefixRange(p netip.Prefix) (start, end uint64) {
	start = uint64(addrToU32(p.Addr()))
	size := uint64(1) << (32 - p.Bits())
	return start, start + size - 1
}
