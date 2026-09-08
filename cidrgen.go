package cidrgen

import (
	"fmt"
	"net/netip"
)

// Request is the full input to [Generate]. The package holds no state, so the
// caller supplies Allocated in full on every call.
type Request struct {
	// Parent is the pool to allocate within, e.g. "10.0.0.0/16". Required.
	Parent string

	// Allocated lists the CIDRs already carved from Parent. Each must be inside
	// Parent, and no two may overlap. CIDR strings with host bits set are
	// accepted and canonicalized.
	Allocated []string

	// Netmask is the prefix length for the new CIDR. When non-zero it takes
	// precedence over Classification.
	Netmask int

	// Classification names an entry in Classifications; used when Netmask is 0.
	Classification string

	// Classifications maps a classification name to a prefix length.
	Classifications map[string]int
}

// Generate returns the lowest-address, correctly-aligned CIDR of the requested
// size that fits within req.Parent without overlapping any entry in
// req.Allocated.
//
// The requested size comes from req.Netmask when it is non-zero, otherwise from
// looking up req.Classification in req.Classifications. It is an error to supply
// neither.
//
// Errors are one of the sentinels declared in this package, wrapped with
// context; match them with errors.Is.
func Generate(req Request) (netip.Prefix, error) {
	parent, err := parsePrefix(req.Parent)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("parent: %w", err)
	}

	bits, err := resolveBits(req, parent.Bits())
	if err != nil {
		return netip.Prefix{}, err
	}

	allocated := make([]netip.Prefix, 0, len(req.Allocated))
	for _, s := range req.Allocated {
		p, err := parsePrefix(s)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("allocated: %w", err)
		}
		allocated = append(allocated, p)
	}

	sorted, err := validateAllocated(parent, allocated)
	if err != nil {
		return netip.Prefix{}, err
	}

	return firstFit(parent, sorted, bits)
}

// resolveBits determines the requested prefix length. Netmask wins when set;
// otherwise Classification is looked up. The result must be strictly longer than
// parentBits and within 1..32.
func resolveBits(req Request, parentBits int) (int, error) {
	var bits int
	switch {
	case req.Netmask != 0:
		bits = req.Netmask
	case req.Classification != "":
		b, ok := req.Classifications[req.Classification]
		if !ok {
			return 0, fmt.Errorf("%w: %q", ErrUnknownClassification, req.Classification)
		}
		bits = b
	default:
		return 0, ErrNoSizeSpecified
	}
	if bits <= parentBits || bits > 32 {
		return 0, fmt.Errorf("%w: /%d is not inside parent /%d", ErrInvalidPrefix, bits, parentBits)
	}
	return bits, nil
}
