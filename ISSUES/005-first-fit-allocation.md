# 005 — First-fit allocation

> **Superseded in part by [007](007-generator-type.md):** `Generate` is now a
> method on `*Generator`; the parent is parsed in `New`, not per call. `firstFit`
> and the `parse → resolve → validate → allocate` pipeline are unchanged.

## Context

The core: given the validated, sorted allocations and a target prefix length,
return the lowest-address aligned block that fits. This issue also wires
`parse → resolve → validate → allocate` together in `Generate`.

## Tasks

- [ ] `allocate.go`: `firstFit(parent netip.Prefix, sorted []netip.Prefix, bits int) (netip.Prefix, error)`
  - `blockSize = 1 << (32 - bits)`; walk `candidate` from the parent's start.
  - `candidate` stays aligned: the parent start is aligned to its larger block,
    and each re-alignment rounds `allocatedEnd + 1` up to a multiple of
    `blockSize`.
  - Return the first `candidate` whose block ends before the next allocation.
  - After the last allocation, return `candidate` if its block still fits inside
    the parent, else `ErrNoSpace`.
  - Do the arithmetic in `uint64` and only narrow to `uint32` when constructing
    the result, so the top of the address space does not overflow.
- [ ] `cidrgen.go`: `Generate` calls `parsePrefix` (parent + each allocated),
      `resolveBits`, `validateAllocated`, then `firstFit`; wraps errors with
      context.

## Test deliverable

`allocate_test.go`:

- `assertFits` helper: result inside parent, aligned to its own size, overlaps
  no input — called on every success case.
- empty pool → lowest block; fills first gap; skips a run; re-aligns past a
  smaller allocation; smaller request slots into a sub-block gap; exact fit in
  the last slot.
- full pool → `ErrNoSpace`; free space exists but no aligned gap large enough →
  `ErrNoSpace`.
- top of the address space (`255.255.255.0/24`) and whole space (`0.0.0.0/0`).
- `TestFirstFitSequential`: carve a `/16` into 256 `/24`s one at a time, feeding
  each result back; invariant holds every iteration, no block twice, 257th call
  → `ErrNoSpace`.
- `cidrgen_test.go` `TestGenerate`: end-to-end incl. host-bits-in-input,
  unparseable parent/allocated, overlap, out-of-parent, exhaustion.

## Done when

`go test ./... -race` green; coverage ≥ 95%.
