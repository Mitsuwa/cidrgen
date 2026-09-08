// Package cidrgen allocates non-overlapping IPv4 CIDR blocks from a parent pool.
//
// A [Generator] is created with [New] from a parent CIDR and a classification
// map. Given the list of CIDRs already carved out of the parent,
// [Generator.Generate] returns the lowest-address, correctly-aligned free block
// of a requested size. The size is given either directly as a prefix length
// (Request.Netmask) or indirectly through a classification name that maps to a
// prefix length (Request.Classification, resolved against the map passed to
// [New]). The classification map can be loaded from a YAML document with
// [LoadClassifications].
//
// A Generator is immutable after New and safe for concurrent use. The package
// holds no state: callers re-supply the full Allocated list on every call and
// append each result to it before requesting the next block.
//
// Only IPv4 is supported.
package cidrgen
