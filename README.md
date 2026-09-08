# cidrgen

Allocate non-overlapping IPv4 CIDR blocks from a parent pool.

Given a parent CIDR and the blocks already carved out of it, `Generate` returns
the lowest-address, correctly-aligned free block of a requested size. The size is
given directly as a prefix length, or indirectly through a *classification* name
that maps to one.

```go
import "github.com/Mitsuwa/cidrgen2"

// Explicit size.
p, err := cidrgen.Generate(cidrgen.Request{
    Parent:    "10.0.0.0/16",
    Allocated: []string{"10.0.0.0/24", "10.0.2.0/24"},
    Netmask:   24,
})
// p == 10.0.1.0/24

// Size by classification.
classes := map[string]int{"datanode": 28, "computenode": 27}
p, err = cidrgen.Generate(cidrgen.Request{
    Parent:          "10.0.0.0/24",
    Allocated:       []string{"10.0.0.0/28"},
    Classification:  "datanode",
    Classifications: classes,
})
// p == 10.0.0.16/28
```

`Netmask` wins over `Classification` when both are set. The classification map
can be loaded from YAML:

```yaml
# classifications.yaml
classifications:
  datanode: 28
  computenode: 27
```

```go
f, _ := os.Open("classifications.yaml")
classes, err := cidrgen.LoadClassifications(f)
```

## Behavior

- **Stateless.** Re-supply `Allocated` on every call; append each result before
  requesting the next block.
- **IPv4 only.**
- CIDR strings with host bits set (`10.0.0.5/24`) are accepted and canonicalized.
- First-fit: the returned block is the lowest-address aligned gap that fits.

## Errors

All errors match one of the package sentinels with `errors.Is`:
`ErrNoSizeSpecified`, `ErrUnknownClassification`, `ErrOverlappingInput`,
`ErrOutOfParent`, `ErrInvalidPrefix`, `ErrNoSpace`.

## Documentation

| | |
|---|---|
| [docs/design.md](docs/design.md) | The problem and the design decisions |
| [docs/api.md](docs/api.md) | Full API reference |
| [docs/algorithm.md](docs/algorithm.md) | How first-fit allocation works |
| [docs/classifications.md](docs/classifications.md) | Classification YAML and size resolution |
| [docs/development.md](docs/development.md) | Local workflow and CI |

## Development

See [CLAUDE.md](CLAUDE.md) for the working agreement, [docs/development.md](docs/development.md)
for the workflow, and [ISSUES/](ISSUES/) for the planned work.

```
go vet ./...
go test ./... -race
```
