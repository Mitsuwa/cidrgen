package cidrgen

import "errors"

// Sentinel errors returned by [New], [Generator.Generate], and
// [LoadClassifications]. Callers can match them with errors.Is.
var (
	// ErrNoSizeSpecified is returned when a Request supplies neither Netmask nor
	// Classification.
	ErrNoSizeSpecified = errors.New("cidrgen: neither Netmask nor Classification specified")

	// ErrUnknownClassification is returned when Request.Classification is not a
	// key in the classification map passed to New.
	ErrUnknownClassification = errors.New("cidrgen: classification not found in Classifications map")

	// ErrOverlappingInput is returned when two entries in Request.Allocated
	// overlap each other.
	ErrOverlappingInput = errors.New("cidrgen: allocated CIDRs overlap each other")

	// ErrOutOfParent is returned when an entry in Request.Allocated is not fully
	// contained within the Generator's parent.
	ErrOutOfParent = errors.New("cidrgen: allocated CIDR is not contained within Parent")

	// ErrInvalidPrefix is returned for an unparseable CIDR string, a non-IPv4
	// CIDR, or a requested prefix length that is not strictly longer than the
	// parent prefix and within 1..32.
	ErrInvalidPrefix = errors.New("cidrgen: invalid CIDR or prefix length")

	// ErrNoSpace is returned when no free aligned block of the requested size
	// fits within the Generator's parent.
	ErrNoSpace = errors.New("cidrgen: no free aligned block of the requested size fits within Parent")
)
