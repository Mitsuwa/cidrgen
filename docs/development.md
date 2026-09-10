# Development

## Layout

| File | Responsibility |
|---|---|
| `doc.go` | package doc comment |
| `cidrgen.go` | `Request`, `Generator`, `New`, `Generate`, `resolveBits` |
| `parse.go` | CIDR string → `netip.Prefix`, normalization, `uint32` helpers |
| `validate.go` | containment + overlap checks, sorting |
| `allocate.go` | first-fit scan |
| `classification.go` | `LoadClassifications` YAML helper |
| `errors.go` | sentinel errors |
| `*_test.go` | one test file per source file, table-driven |
| `testdata/classifications.yaml` | fixture for the loader test |
| `ISSUES/` | ordered, test-first PR breakdown |

## Local checks

Run before every commit (also what CI runs):

```sh
go mod tidy
go vet ./...
go test ./... -race -count=1
```

`go test ./... -cover` should stay at or above its current level (~98%).

## The rule for every change

From [CLAUDE.md](../CLAUDE.md):

1. **Tests stay green.** `go test ./... -race` must pass before a PR opens and
   before it merges. `.github/workflows/test.yml` enforces this on every PR.
2. **New behavior ships with tests** in the same PR. A refactor-only PR keeps
   coverage where it was.
3. **Allocation success-path tests assert the invariant** via `assertFits`: the
   returned prefix is inside the parent, aligned to its own size, and overlaps
   nothing in the input.

## Design constraints (do not drift)

- Immutable after construction: no package-level state, no input mutation; the
  `Generator` is fixed after `New` and safe for concurrent `Generate` calls.
- `net/netip` for parsing/validation, `uint32` for address math, IPv4 only.
- Inputs are CIDR strings, canonicalized with `Masked()`; output is a
  `netip.Prefix`.
- First-fit, lowest address.
- Conditions that have a sentinel in `errors.go` return that sentinel (wrapped),
  never a bare `errors.New`.

## Testing patterns

- Table-driven, `t.Run` per case, `errors.Is` for error assertions.
- "Mock CIDRs" means fixture `[]string` / `[]netip.Prefix` slices — there are no
  interface mocks.
- `mustPrefixes(t, …)` builds `[]netip.Prefix` from strings in tests.
- `TestFirstFitSequential` carves a `/16` into 256 `/24`s one at a time, feeding
  each result back, and checks the invariant on every iteration — the closest
  thing to a property test without adding a dependency.
- `TestGenerateMixedSizeSequence` drives one `Generator` through an ordered mix
  of `/16`–`/32` requests (and classifications), feeding each result back into
  `Allocated`, and asserts the exact block returned at every step plus the
  `assertFits` invariant.

## CI

`.github/workflows/test.yml`, on every pull request:

1. `go mod tidy` then `git diff --exit-code -- go.mod go.sum` — modules stay tidy.
2. `go vet ./...`
3. `go test ./... -race -count=1 -covermode=atomic`

Go version comes from `go.mod` (`go-version-file`).

`.github/workflows/release.yml` is the authoritative run for `main`: on every
push it re-runs `go vet` and `go test ./... -race`, then tags `v<VERSION>` and
cuts a GitHub Release if that tag does not already exist. See
[releasing.md](releasing.md).

## Roadmap

`ISSUES/001`–`007` describe the intended incremental path (scaffold → types →
validation → size resolution → allocation → YAML loader → `Generator` type),
each with an explicit test deliverable. `ISSUES/README.md` has the dependency
order.
