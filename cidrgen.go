package cidrgen

import (
	"fmt"
	"maps"
	"net/netip"
)

// Request is the per-call input to [Generator.Generate]. The parent pool and the
// classification map are fixed for the life of a [Generator] and are supplied to
// [New]; only the fields here change between calls. The package holds no state,
// so the caller supplies Allocated in full on every call.
type Request struct {
	// Allocated lists the CIDRs already carved from the Generator's parent. Each
	// must be inside the parent, and no two may overlap. CIDR strings with host
	// bits set are accepted and canonicalized.
	Allocated []string

	// Netmask is the prefix length for the new CIDR. When non-zero it takes
	// precedence over Classification.
	Netmask int

	// Classification names an entry in the Generator's classification map; used
	// when Netmask is 0.
	Classification string
}

// Generator allocates non-overlapping CIDRs from a fixed parent pool. It is
// created with [New], is immutable afterward, and is safe for concurrent use by
// multiple goroutines as long as each call supplies its own [Request].
type Generator struct {
	parent          netip.Prefix
	classifications map[string]int
}

// New builds a Generator for the given parent pool (e.g. "10.0.0.0/16") and
// classification map. The parent is parsed and canonicalized now, so a CIDR
// string with host bits set is accepted and an unparseable or non-IPv4 parent is
// an ErrInvalidPrefix returned here rather than from Generate.
//
// classifications maps a classification name to a prefix length; it may be nil
// when callers only ever request an explicit Netmask. New copies the map, so
// later mutation by the caller does not affect the Generator. Values are not
// range-checked here: a length that cannot sit inside the parent is reported by
// Generate as ErrInvalidPrefix, and an unknown name as ErrUnknownClassification.
func New(parent string, classifications map[string]int) (*Generator, error) {
	p, err := parsePrefix(parent)
	if err != nil {
		return nil, fmt.Errorf("parent: %w", err)
	}

	return &Generator{parent: p, classifications: maps.Clone(classifications)}, nil
}

// Generate returns the lowest-address, correctly-aligned CIDR of the requested
// size that fits within the Generator's parent without overlapping any entry in
// req.Allocated.
//
// The requested size comes from req.Netmask when it is non-zero, otherwise from
// looking up req.Classification in the Generator's classification map. It is an
// error to supply neither.
//
// Errors are one of the sentinels declared in this package, wrapped with
// context; match them with errors.Is.
func (g *Generator) Generate(req Request) (netip.Prefix, error) {
	bits, err := g.resolveBits(req)
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

	sorted, err := validateAllocated(g.parent, allocated)
	if err != nil {
		return netip.Prefix{}, err
	}

	return firstFit(g.parent, sorted, bits)
}

// resolveBits determines the requested prefix length. Netmask wins when set;
// otherwise Classification is looked up in the Generator's map. The result must
// be strictly longer than the parent prefix and within 1..32. It performs no I/O
// and no parsing.
func (g *Generator) resolveBits(req Request) (int, error) {
	parentBits := g.parent.Bits()

	var bits int
	switch {
	case req.Netmask != 0:
		bits = req.Netmask
	case req.Classification != "":
		b, ok := g.classifications[req.Classification]
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
