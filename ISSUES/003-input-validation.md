# 003 — Input validation: containment and overlap

## Context

`Generate` must reject a malformed `Allocated` list before it tries to allocate:
an entry that cannot belong to the pool, or two entries that overlap each other,
are caller mistakes worth surfacing rather than silently working around.

## Tasks

- [ ] `validate.go`: `validateAllocated(parent netip.Prefix, allocated []netip.Prefix) ([]netip.Prefix, error)`
  - Sort a copy ascending by start address, ties broken by shorter prefix first
    (`sort.Slice`).
  - Every entry fully within `parent` → else `ErrOutOfParent`.
  - No two entries overlap: after sorting, `cur.start <= prevEnd` → `ErrOverlappingInput`
    (tracking `prevEnd` is sufficient because non-overlapping sorted intervals
    have monotonic ends).
  - Return the sorted slice for the allocator to consume.

## Test deliverable

`validate_test.go`, table-driven, parent `10.0.0.0/16`:

- empty list → ok, empty result.
- disjoint out-of-order input → ok, returned sorted.
- adjacent blocks (`10.0.0.0/25`, `10.0.0.128/25`) → ok, not an overlap.
- identical entries → `ErrOverlappingInput`.
- nested entries → `ErrOverlappingInput`.
- partial overlap (`10.0.0.0/23`, `10.0.1.0/24`) → `ErrOverlappingInput`.
- entry in a different range (`192.168.1.0/24`) → `ErrOutOfParent`.
- entry larger than parent (`10.0.0.0/15`) → `ErrOutOfParent`.
- parent-sized entry (`10.0.0.0/16`) → ok.

## Done when

`go test ./...` green; the allocator can assume its input is sorted, in-parent,
and non-overlapping.
