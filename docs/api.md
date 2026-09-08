# API reference

```go
import "github.com/Mitsuwa/cidrgen"
```

Package `cidrgen`. IPv4 only.

## `New`

```go
func New(parent string, classifications map[string]int) (*Generator, error)
```

Builds a [`Generator`](#generator) bound to a parent pool and a classification
map.

- **`parent`** — any IPv4 CIDR string, e.g. `"10.0.0.0/16"`. Host bits set are
  tolerated (`10.0.0.9/16` is treated as `10.0.0.0/16`). Parsed and
  canonicalized now; an unparseable or non-IPv4 string is `ErrInvalidPrefix`.
- **`classifications`** — maps a classification name to a prefix length,
  typically from [`LoadClassifications`](#loadclassifications). May be `nil` when
  callers only ever request an explicit `Netmask`. `New` copies the map, so
  mutating it afterward does not affect the `Generator`. Values are not
  range-checked here — a length that cannot sit inside `parent` surfaces from
  `Generate` as `ErrInvalidPrefix`.

```go
g, err := cidrgen.New("10.0.0.0/16", nil)
```

## `Generator`

```go
type Generator struct {
    // unexported
}
```

Immutable after `New`. Safe for concurrent use by multiple goroutines, as long
as each call passes its own `Request`.

### `Generate`

```go
func (g *Generator) Generate(req Request) (netip.Prefix, error)
```

Returns the lowest-address, correctly-aligned CIDR of the requested size that
fits within `g`'s parent without overlapping any entry in `req.Allocated`.

Resolution order for the size: `req.Netmask` if non-zero, else the classification
map entry for `req.Classification`, else `ErrNoSizeSpecified`.

The result is a canonical `netip.Prefix`; call `.String()` for text.

```go
g, _ := cidrgen.New("10.0.0.0/16", nil)
p, err := g.Generate(cidrgen.Request{
    Allocated: []string{"10.0.0.0/24", "10.0.2.0/24"},
    Netmask:   24,
})
// p.String() == "10.0.1.0/24"
```

## `Request`

```go
type Request struct {
    Allocated      []string // CIDRs already carved from the parent
    Netmask        int      // prefix length for the new CIDR; wins when non-zero
    Classification string   // classification name; used when Netmask == 0
}
```

- **`Allocated`** — each entry must be a valid IPv4 CIDR, fully inside the
  `Generator`'s parent, and non-overlapping with the others. Host bits set are
  tolerated. Order does not matter. `nil` / empty means the whole parent is free.
- **`Netmask`** — the requested prefix length, e.g. `24`. Must be strictly longer
  than the parent's prefix and no greater than `32`. `0` means "not set — use
  `Classification`".
- **`Classification`** — a key into the map passed to `New`. Consulted only when
  `Netmask == 0`. An unknown key (or a `nil` map) is `ErrUnknownClassification`.

### Allocating several blocks

`Generate` returns one block. Thread each result back into `Allocated`:

```go
g, _ := cidrgen.New("10.0.0.0/16", nil)
allocated := []string{"10.0.0.0/24"}
for i := 0; i < 3; i++ {
    p, err := g.Generate(cidrgen.Request{Allocated: allocated, Netmask: 24})
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

Parses a YAML document into a map suitable for the `classifications` argument of
[`New`](#new):

```yaml
classifications:
  datanode: 28
  computenode: 27
```

```go
f, err := os.Open("classifications.yaml")
// ...
classes, err := cidrgen.LoadClassifications(f)
g, err := cidrgen.New("10.0.0.0/16", classes)
```

Errors on: an empty document, a `classifications` key that is empty or `null`, a
stray or misspelled top-level key (`classification:` is rejected, not ignored), a
non-integer value, or a prefix length outside `0..32`. See
[classifications.md](classifications.md).

## Errors

All errors returned by this package match one of these sentinels with
`errors.Is`. `New` and `Generate` wrap them with context via
`fmt.Errorf("%w", …)`; `LoadClassifications` returns its own descriptive errors.

| Sentinel | Condition |
|---|---|
| `ErrNoSizeSpecified` | `Request` supplies neither `Netmask` nor `Classification` |
| `ErrUnknownClassification` | `Classification` is not a key in the map passed to `New` (a `nil` map counts) |
| `ErrOverlappingInput` | two `Allocated` entries overlap each other |
| `ErrOutOfParent` | an `Allocated` entry is not fully contained in the parent |
| `ErrInvalidPrefix` | unparseable / non-IPv4 CIDR string (parent or allocated), or a requested prefix not strictly inside the parent and within `1..32` |
| `ErrNoSpace` | no free aligned block of the requested size fits within the parent |

```go
_, err := g.Generate(req)
switch {
case errors.Is(err, cidrgen.ErrNoSpace):
    // pool is full for this size
case errors.Is(err, cidrgen.ErrOverlappingInput):
    // caller's allocated list is inconsistent
}
```
