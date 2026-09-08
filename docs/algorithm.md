# Allocation algorithm

The parent pool is parsed once in `New`. Each `Generate` call then runs four
stages: **parse → resolve size → validate → allocate**.

## 0. Parse the parent (`New` in `cidrgen.go`)

`New` runs the `parent` string through `parsePrefix` and stores the canonical
`netip.Prefix` on the `Generator`. A bad parent is `ErrInvalidPrefix` returned
from `New`, not `Generate`.

## 1. Parse the request (`parse.go`)

Every `Allocated` string is run through `parsePrefix`:

- `netip.ParsePrefix` — rejects bad syntax and `/33`+ automatically.
- non-IPv4 (including IPv4-mapped IPv6) is rejected.
- `netip.Prefix.Masked()` canonicalizes: `10.0.0.5/24` → `10.0.0.0/24`.

Any failure wraps `ErrInvalidPrefix`.

Helpers:

- `addrToU32` / `u32ToAddr` — IPv4 address ↔ `uint32`.
- `prefixRange(p) (start, end uint64)` — the inclusive address range a prefix
  covers. Returned as `uint64` so `0.0.0.0/0` (end `0xFFFFFFFF`) and arithmetic
  that steps one past the top of the space during the scan cannot overflow.

## 2. Resolve size (`(*Generator).resolveBits` in `cidrgen.go`)

`Netmask != 0` → use it. Else look up `Classification` in the Generator's map.
Else `ErrNoSizeSpecified`.
The result `bits` must satisfy `parentBits < bits <= 32`, otherwise
`ErrInvalidPrefix`. A negative `Netmask` falls through to this range check and so
also yields `ErrInvalidPrefix`. See [classifications.md](classifications.md).

## 3. Validate (`validate.go`)

`validateAllocated`:

1. Sort a copy of the entries ascending by start address, ties broken by shorter
   prefix first (so a container sorts before the blocks nested in it).
2. Each entry's `[start, end]` must lie within the parent's range — else
   `ErrOutOfParent`.
3. Walking the sorted list, if the current entry's `start <= prevEnd` the entries
   overlap — `ErrOverlappingInput`. Tracking only the previous end is sufficient:
   a set of non-overlapping intervals sorted by start has monotonically
   increasing ends, so any nesting or partial overlap shows up as this
   comparison failing.

The sorted slice is handed to the allocator.

## 4. Allocate (`firstFit` in `allocate.go`)

Inputs: the canonical `parent`, the validated **sorted** allocations, and the
target `bits`.

```
blockSize = 1 << (32 - bits)          // addresses per block
candidate = parent.start              // first address to try
```

`candidate` is always aligned to `blockSize`. It starts at the parent's network
address (already a multiple of the parent's larger block size, and
`bits > parentBits`), and every time it is advanced past an allocation it is
rounded **up** to the next multiple of `blockSize`.

Scan the sorted allocations in order. For each allocation `a`:

- If `candidate + blockSize - 1 < a.start`, the block `[candidate, candidate +
  blockSize - 1]` fits entirely before `a` — **return it**.
- Otherwise `candidate` collides with or precedes `a`. Set `candidate` to
  `a.end + 1` rounded up to `blockSize` (unless that would move it backwards, in
  which case leave it — a later, larger allocation already pushed it past this
  one).

After the last allocation, if `candidate + blockSize - 1 <= parent.end` the
trailing block fits — return it. Otherwise `ErrNoSpace`.

All arithmetic is `uint64`; the result is narrowed to `uint32` only when
constructing the returned `netip.Prefix`, which by then is guaranteed `<=
parent.end < 2^32`.

### Why alignment can be assumed, not enforced per-candidate

A CIDR of prefix length `n` must start on a multiple of `2^(32-n)`. Because the
scan only ever sets `candidate` to (a) the parent start or (b) `a.end + 1`
rounded up to `blockSize`, every value it takes is already a multiple of
`blockSize`. No separate alignment step is needed.

### Worked example

Parent `10.0.0.0/24`, allocated `[10.0.0.0/26, 10.0.0.192/26]`, request `/25`
(`blockSize = 128`), offsets relative to `10.0.0.0`:

| step | candidate | vs allocation | action |
|---|---|---|---|
| start | 0 | `10.0.0.0/26` = `[0,63]` | `0 + 127 < 0`? no → advance to `64`, round up to `128` |
| | 128 | `10.0.0.192/26` = `[192,255]` | `128 + 127 < 192`? no → advance to `256`, round up to `256` |
| end | 256 | — | `256 + 127 <= 255`? no → **`ErrNoSpace`** |

There are 128 free addresses (`64–191`) but no aligned `/25` fits them, which is
the correct answer.

### Complexity

`O(k log k)` for the sort, `O(k)` for the scan, where `k = len(Allocated)`.
Constant extra space beyond the sorted copy.
