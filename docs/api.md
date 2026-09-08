# API reference

```go
import "github.com/Mitsuwa/cidrgen2"
```

Package `cidrgen`. IPv4 only.

## `Request`

```go
type Request struct {
    Parent          string         // pool to allocate within, e.g. "10.0.0.0/16" — required
    Allocated       []string       // CIDRs already carved from Parent
    Netmask         int             // prefix length for the new CIDR; wins when non-zero
    Classification  string          // used when Netmask == 0
    Classifications map[string]int  // classification name -> prefix length
}
```

- **`Parent`** — any IPv4 CIDR string. Host bits set are tolerated
  (`10.0.0.9/16` is treated as `10.0.0.0/16`).
- **`Allocated`** — each entry must be a valid IPv4 CIDR, fully inside `Parent`,
  and non-overlapping with the others. Host bits set are tolerated. Order does
  not matter. `nil` / empty means the whole parent is free.
- **`Netmask`** — the requested prefix length, e.g. `24`. Must be strictly longer
  than the parent's prefix and no greater than `32`. `0` means "not set — use
  `Classification`".
- **`Classification`** — a key into `Classifications`. Consulted only when
  `Netmask == 0`.
- **`Classifications`** — the lookup table, typically from
  [`LoadClassifications`](#loadclassifications). Only needed when using
  `Classification`.

## `Generate`

```go
func Generate(req Request) (netip.Prefix, error)
```

Returns the lowest-address, correctly-aligned CIDR of the requested size that
fits within `req.Parent` without overlapping any entry in `req.Allocated`.

Resolution order for the size: `req.Netmask` if non-zero, else
`req.Classifications[req.Classification]`, else `ErrNoSizeSpecified`.

The result is a canonical `netip.Prefix`; call `.String()` for text.

```go
p, err := cidrgen.Generate(cidrgen.Request{
    Parent:    "10.0.0.0/16",
    Allocated: []string{"10.0.0.0/24", "10.0.2.0/24"},
    Netmask:   24,
})
// p.String() == "10.0.1.0/24"
```

### Allocating several blocks

`Generate` returns one block. Thread each result back into `Allocated`:

```go
allocated := []string{"10.0.0.0/24"}
for i := 0; i < 3; i++ {
    p, err := cidrgen.Generate(cidrgen.Request{
        Parent: "10.0.0.0/16", Allocated: allocated, Netmask: 24,
    })
    if err != nil {
        break
    }
    allocated = append(allocated, p.String())
}
// allocated now also holds 10.0.1.0/24, 10.0.3.0/24, 10.0.4.0/24
```

## `LoadClassifications`

```go
func LoadClassifications(r io.Reader) (map[string]int, error)
```

Parses a YAML document into a map suitable for `Request.Classifications`:

```yaml
classifications:
  datanode: 28
  computenode: 27
```

```go
f, err := os.Open("classifications.yaml")
// ...
classes, err := cidrgen.LoadClassifications(f)
```

Errors on: an empty document, a `classifications` key that is empty or `null`, a
stray or misspelled top-level key (`classification:` is rejected, not ignored), a
non-integer value, or a prefix length outside `0..32`. See
[classifications.md](classifications.md).

## Errors

All errors returned by this package match one of these sentinels with
`errors.Is`. `Generate` wraps them with context via `fmt.Errorf("%w", …)`;
`LoadClassifications` returns its own descriptive errors.

| Sentinel | Condition |
|---|---|
| `ErrNoSizeSpecified` | `Request` supplies neither `Netmask` nor `Classification` |
| `ErrUnknownClassification` | `Classification` is not a key in `Classifications` (a `nil` map counts) |
| `ErrOverlappingInput` | two `Allocated` entries overlap each other |
| `ErrOutOfParent` | an `Allocated` entry is not fully contained in `Parent` |
| `ErrInvalidPrefix` | unparseable / non-IPv4 CIDR string, or a requested prefix not strictly inside the parent and within `1..32` |
| `ErrNoSpace` | no free aligned block of the requested size fits within `Parent` |

```go
_, err := cidrgen.Generate(req)
switch {
case errors.Is(err, cidrgen.ErrNoSpace):
    // pool is full for this size
case errors.Is(err, cidrgen.ErrOverlappingInput):
    // caller's allocated list is inconsistent
}
```
