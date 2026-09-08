# Classifications and size resolution

A request's block size comes from one of two inputs.

## Resolution order

1. **`Request.Netmask`** — if non-zero, this is the prefix length. It always
   wins, even when a classification is also given.
2. **`Request.Classification`** — otherwise, this name is looked up in the
   classification map passed to `New`. A missing key (or a `nil` map) is
   `ErrUnknownClassification`.
3. **Neither** — `ErrNoSizeSpecified`.

The resolved length must be strictly longer than the parent's prefix and no
greater than `32`, otherwise `ErrInvalidPrefix`.

`Classifications` below is the map passed to `New`.

| `Netmask` | `Classification` | `Classifications` | Result |
|---|---|---|---|
| `26` | `""` | — | `/26` |
| `0` | `"datanode"` | `{datanode: 28}` | `/28` |
| `26` | `"datanode"` | `{datanode: 28}` | `/26` (netmask wins) |
| `0` | `""` | — | `ErrNoSizeSpecified` |
| `0` | `"edge"` | `{datanode: 28}` | `ErrUnknownClassification` |
| `0` | `"datanode"` | `nil` | `ErrUnknownClassification` |
| `16` | — | — | `ErrInvalidPrefix` (parent is `/16` or shorter) |

## YAML file format

`LoadClassifications` reads a single YAML document:

```yaml
classifications:
  datanode: 28
  computenode: 27
  edgenode: 30
```

- Top-level key **must** be `classifications` (spelled exactly — an unknown
  top-level key is an error, so `classification:` is rejected rather than
  silently ignored).
- Each value is an integer prefix length in `0..32`.

```go
f, err := os.Open("classifications.yaml")
if err != nil { /* ... */ }
defer f.Close()

classes, err := cidrgen.LoadClassifications(f)
if err != nil { /* ... */ }

g, err := cidrgen.New("10.0.0.0/24", classes)
if err != nil { /* ... */ }

p, err := g.Generate(cidrgen.Request{
    Allocated:      []string{"10.0.0.0/28"},
    Classification: "datanode",
})
// p.String() == "10.0.0.16/28"
```

### Errors from `LoadClassifications`

| Input | Result |
|---|---|
| empty / comment-only document | error: `empty document` |
| `classifications:` present but empty or `null` | error: `missing "classifications" key` |
| a different top-level key (`other:`), or `classification:` misspelled | YAML decode error (strict fields) |
| non-integer value (`datanode: small`) | YAML decode error |
| value `> 32` or `< 0` | error: `prefix length N out of range 0..32` |
| malformed YAML | YAML decode error |

The map is validated only for range here. Whether a given length actually fits a
particular parent is checked later, by `Generate`.
