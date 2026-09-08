# 004 — Size resolution: netmask and classification

## Context

The requested block size comes from one of two places, and the precedence and
error cases need to be exact: an explicit `Netmask` always wins; otherwise a
`Classification` name is looked up; a size that cannot sit inside the parent is
rejected up front.

## Tasks

- [ ] `cidrgen.go`: `resolveBits(req Request, parentBits int) (int, error)`
  - `req.Netmask != 0` → use it (a negative value falls through to the range
    check below and yields `ErrInvalidPrefix`).
  - else `req.Classification != ""` → look up in `req.Classifications`; missing
    key (or nil map) → `ErrUnknownClassification`.
  - else → `ErrNoSizeSpecified`.
  - Result must satisfy `parentBits < bits <= 32`, else `ErrInvalidPrefix`.

## Test deliverable

`cidrgen_test.go` `TestResolveBits`, table-driven:

- netmask only; classification only; both set (netmask wins).
- neither → `ErrNoSizeSpecified`.
- unknown classification → `ErrUnknownClassification`.
- classification with nil map → `ErrUnknownClassification`.
- `bits == parentBits`, `bits < parentBits`, `bits > 32`, negative netmask →
  `ErrInvalidPrefix`.

## Done when

`go test ./...` green; `resolveBits` is pure (no I/O, no parsing).
