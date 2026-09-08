# cidrgen — working agreement

`cidrgen` allocates non-overlapping IPv4 CIDR blocks from a parent pool. It is a
single Go package (`github.com/Mitsuwa/cidrgen`), no CLI, no subpackages.

## The rule for every change

1. **Tests stay green.** `go test ./... -race` must pass before a PR is opened and
   before it merges. CI (`.github/workflows/test.yml`) enforces this.
2. **New behavior ships with tests.** Any PR that changes what the package does
   adds or extends table-driven tests covering that change in the same PR. A PR
   that only refactors keeps coverage at least where it was.
3. **Allocation tests assert the invariant.** Every test that exercises
   `Generator.Generate` or `firstFit` on a success path calls `assertFits` (or
   an equivalent check): the returned prefix is inside the parent, aligned to
   its own size, and overlaps nothing in the input.

## Design constraints (do not drift from these)

- **Immutable after construction.** No package-level state, no mutation of
  inputs. The parent pool and classification map are fixed at `New`; the
  `Generator` is never mutated afterward and concurrent `Generate` calls are
  safe. Callers re-supply `Request.Allocated` every call.
- **`net/netip` for parsing/validation, `uint32` for address math.** IPv4 only.
- **Inputs are CIDR strings**, parsed inside the package and canonicalized with
  `netip.Prefix.Masked()` (host bits set are tolerated, not rejected). The
  return value is a `netip.Prefix`.
- **Size resolution:** `Request.Netmask` wins when non-zero; otherwise
  `Request.Classification` is looked up in the classification map passed to
  `New`. Neither set is `ErrNoSizeSpecified`.
- **Allocation strategy:** first-fit, lowest address, correctly aligned.
- **Errors** are the sentinels in `errors.go`, wrapped with `fmt.Errorf("%w", …)`
  and matched by callers with `errors.Is`. Do not return bare `errors.New`
  strings for conditions that already have a sentinel.

## Layout

| File | Responsibility |
|---|---|
| `cidrgen.go` | `Request`, `Generator`, `New`, `Generate`, `resolveBits` |
| `parse.go` | string → `netip.Prefix`, normalization, `uint32` helpers |
| `validate.go` | containment + overlap checks, sorting |
| `allocate.go` | first-fit scan |
| `classification.go` | `LoadClassifications` YAML helper |
| `errors.go` | sentinel errors |
| `docs/` | design, API, algorithm, classification, and development docs |
| `ISSUES/` | ordered, test-first PR breakdown |

When behavior or design constraints change, update the relevant file in `docs/`
in the same PR.

## Before committing

```
go mod tidy
go vet ./...
go test ./... -race -count=1
```
