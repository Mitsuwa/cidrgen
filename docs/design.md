# Design

## Problem

Given a parent IPv4 pool (e.g. `10.0.0.0/16`) and the CIDRs already carved out of
it, return the next free block of a requested size that does not overlap anything
already allocated and is correctly aligned to its own prefix length.

The size is expressed either directly as a prefix length, or indirectly through a
*classification* name (`datanode`, `computenode`, …) that maps to a prefix
length. The classification map is small and typically loaded from a YAML file
when the calling application starts.

Scope for v1 is **IPv4 only**. The types are chosen so IPv6 can be added later
without an API break, but no IPv6 code paths exist today.

## Decisions

### Bounded parent pool, passed explicitly

Finding a "non-overlapping" CIDR only has meaning inside a boundary. `cidrgen`
takes that boundary as an explicit argument to `New`, which builds a `Generator`
bound to that parent. `Request.Allocated` are the sub-blocks already taken from
it, supplied per call.

The rejected alternative was to search all of `0.0.0.0/0` minus reserved ranges
and treat the input list purely as "occupied" space — that hands back public
address space and is almost never what a caller wants. Inferring the parent from
the largest entry in the list was also rejected: it is ambiguous when two entries
tie, and it prevents a caller from ever requesting a block larger than an
existing one.

### `net/netip` for identity, `uint32` for arithmetic

`net/netip.Prefix` is a comparable value type with a built-in `Compare`, no
allocations, and it parses and canonicalizes CIDRs. It is used at the boundary
and for all parsing and validation.

Address arithmetic (stepping through candidate blocks, computing ranges) is done
on `uint32` — an IPv4 address is exactly 32 bits, so the math is trivial and
exact. Ranges are carried as `uint64` so that the end of `0.0.0.0/0`, and
"one past the top of the space" during the scan, do not overflow. `net.IPNet`
was rejected: byte-slice fields, not comparable, not `sort`-friendly, allocates.

### Immutable after construction

`New` takes the two inputs that are fixed for a caller — the parent pool and the
classification map — parses and copies them, and returns a `*Generator` that is
never mutated afterward. Everything that varies between calls lives in `Request`,
which the caller re-supplies in full every time (`Allocated` especially: the
package keeps no running list). `Generate` mutates nothing.

Because the `Generator` is immutable, concurrent `Generate` calls on one
`Generator` are safe, as long as each call passes its own `Request`.

An earlier revision made `Generate` a bare package-level function with the parent
in `Request`. Moving the parent to `New` parses and validates it once instead of
on every call, shrinks `Request` to the fields that actually change, and lets the
concurrency guarantee be stated rather than merely "not a problem". The core
stays trivially testable — behavior is still fully determined by `New`'s
arguments plus the `Request`.

### Explicit size wins over classification

`Request.Netmask`, when non-zero, always determines the size. Only when it is
zero is `Request.Classification` looked up in the classification map passed to
`New`. Supplying neither is an error (`ErrNoSizeSpecified`). See
[classifications.md](classifications.md).

### First-fit, lowest address

When several free aligned blocks of the requested size exist, the one with the
lowest address is returned. It is deterministic and keeps low addresses dense.
Best-fit / lowest-fragmentation strategies were considered out of scope for v1.

### Lenient input, canonical output

The parent (in `New`) and every `Allocated` entry are run through
`netip.Prefix.Masked()` before use, so `10.0.0.5/24` is accepted and treated as
`10.0.0.0/24`. What is *not* tolerated: entries outside the parent
(`ErrOutOfParent`) and entries that overlap each other (`ErrOverlappingInput`) —
those are caller mistakes worth surfacing rather than silently working around.

### Sentinel errors

Every failure mode has an exported sentinel (`errors.New`) that callers match
with `errors.Is`. Errors returned from `Generate` wrap a sentinel with
`fmt.Errorf("%w", …)` and added context. See [api.md](api.md#errors).

### One package, one dependency

Everything lives in package `cidrgen`. The only external dependency is
`gopkg.in/yaml.v3`, pulled in by `LoadClassifications`. A subpackage split for
the loader was judged overkill for a single helper.
