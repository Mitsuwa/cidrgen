// Package cidrgen allocates non-overlapping IPv4 CIDR blocks from a parent pool.
//
// Given a parent CIDR and the list of CIDRs already carved out of it, [Generate]
// returns the lowest-address, correctly-aligned free block of a requested size.
// The size is given either directly as a prefix length (Request.Netmask) or
// indirectly through a classification name that maps to a prefix length
// (Request.Classification + Request.Classifications). The classification map can
// be loaded from a YAML document with [LoadClassifications].
//
// The package is stateless: callers re-supply the full Allocated list on every
// call and append each result to it before requesting the next block.
//
// Only IPv4 is supported.
package cidrgen
