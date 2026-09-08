# 002 — Domain types, parsing, normalization

> **Superseded in part by [007](007-generator-type.md):** `Request` no longer
> carries `Parent` or `Classifications` — those move to `New`. `Request` is
> `Allocated`, `Netmask`, `Classification`.

## Context

The package boundary takes CIDR strings and works internally in `net/netip` plus
`uint32` address math. This issue establishes the request type, the sentinel
errors, and the parse/normalize layer everything else builds on.

## Tasks

- [ ] `cidrgen.go`: `Request` struct — `Parent string`, `Allocated []string`,
      `Netmask int`, `Classification string`, `Classifications map[string]int`.
- [ ] `errors.go`: `ErrNoSizeSpecified`, `ErrUnknownClassification`,
      `ErrOverlappingInput`, `ErrOutOfParent`, `ErrInvalidPrefix`, `ErrNoSpace`
      as exported `errors.New` values with doc comments.
- [ ] `parse.go`:
  - `parsePrefix(s string) (netip.Prefix, error)` — parse, reject non-IPv4,
    return `Masked()` canonical form; all failures wrap `ErrInvalidPrefix`.
  - `addrToU32` / `u32ToAddr` conversions.
  - `prefixRange(p) (start, end uint64)` — inclusive range, `uint64` so
    `0.0.0.0/0` and just-past-the-top arithmetic do not overflow.

## Test deliverable

`parse_test.go`, table-driven:

- canonical input unchanged; host bits set → masked (`10.0.0.5/24` → `10.0.0.0/24`).
- `/32` and `/0` edge prefixes.
- rejects: missing `/bits`, garbage, `/33`, IPv6, IPv4-mapped IPv6, empty string
  — each `errors.Is(err, ErrInvalidPrefix)`.
- `u32ToAddr(addrToU32(x)) == x` round trip including `0.0.0.0` and
  `255.255.255.255`.
- `prefixRange` for `/24`, `/32`, `/0`, and a range touching the top of the space.

## Done when

`go test ./...` green; `parsePrefix` is the only place CIDR strings enter the
package.
