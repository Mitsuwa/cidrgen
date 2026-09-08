# 007 — `Generator` type: parent and classifications at construction

## Context

Today `Generate` is a package-level function and every call re-supplies
`Request.Parent` (re-parsed each time) and `Request.Classifications`. Both are
fixed for the life of a caller: the parent pool is one CIDR, and the
classification map is loaded once at startup. Only `Allocated` genuinely changes
between calls.

This issue moves the two fixed inputs to a constructor. `Generate` becomes a
method on a `*Generator` that holds the parsed parent and the classification
map; `Request` shrinks to the per-call inputs.

This **reverses** the "no `Allocator` object" wording in `docs/design.md`. The
stateless *contract callers rely on* does not change: the `Generator` is
immutable after `New`, callers still re-supply the full `Allocated` list on
every call, and `Generate` performs no mutation. What we gain is parsing and
validating the parent exactly once, a smaller `Request`, and — because the
`Generator` is immutable — a stated guarantee that concurrent `Generate` calls
are safe.

Depends on **005** (first-fit / `Generate` wiring) and **006**
(`LoadClassifications`, whose output now feeds `New` instead of `Request`).

## Design decisions (settled)

- Type is `Generator`; constructor is
  `New(parent string, classifications map[string]int) (*Generator, error)`.
  Pointer receiver on `Generate`.
- `New` parses and canonicalizes `parent` with `parsePrefix` (host bits
  tolerated). A bad parent is `ErrInvalidPrefix` wrapped with a `parent:`
  prefix — the same error that `Generate` returns today, just raised earlier.
- `New` takes a **shallow copy** of `classifications` so the immutability
  guarantee holds even if the caller mutates the map afterward. `nil` is legal
  (Netmask-only callers never consult it).
- **No eager validation** of the classification values in `New`. Range checking
  stays where it is: `LoadClassifications` guards the YAML path, and
  `resolveBits` checks the resolved length against the actual parent
  (`parentBits < bits <= 32`). A `nil` map or missing key is still
  `ErrUnknownClassification` at lookup time.
- The package-level `Generate` function is **removed**, not kept as a wrapper.
  The package is unreleased and has no CLI; one API to document and test.
- `resolveBits` becomes a method: `(g *Generator) resolveBits(req Request) (int, error)`.
  It reads `g.parent.Bits()` and `g.classifications`. It stays pure in the sense
  issue 004 requires — no I/O, no parsing, just a map lookup and arithmetic.
- `Request` keeps `Allocated []string`, `Netmask int`, `Classification string`.
  It stays a named struct (two of three fields are optional-ish; room to grow
  without an API break).

## Tasks

- [ ] `cidrgen.go`:
  - New `Generator` struct: unexported `parent netip.Prefix`,
    `classifications map[string]int`. Doc comment states it is immutable after
    `New` and safe for concurrent use.
  - `New(parent string, classifications map[string]int) (*Generator, error)` —
    `parsePrefix` the parent (wrap failure as `fmt.Errorf("parent: %w", …)`),
    shallow-copy the map (leave `nil` as `nil`), return `*Generator`.
  - `Generate` becomes `func (g *Generator) Generate(req Request) (netip.Prefix, error)`:
    drop parent parsing; parse each `req.Allocated` entry; call
    `g.resolveBits`, `validateAllocated(g.parent, …)`, `firstFit(g.parent, …)`.
  - `resolveBits` becomes `func (g *Generator) resolveBits(req Request) (int, error)`.
  - Remove the package-level `Generate` function.
  - `Request`: remove the `Parent` and `Classifications` fields; update the
    struct doc comment.
- [ ] `doc.go`: update the package example and the "stateless" paragraph to the
  `New` / method form and the immutability guarantee.
- [ ] `docs/design.md`: rewrite the "Stateless" decision as "Immutable after
  construction" — no mutable state, callers still re-supply `Allocated`,
  concurrent `Generate` is safe. Update "Bounded parent pool, passed
  explicitly" to say the parent is supplied to `New`.
- [ ] `docs/api.md`: `New`, `Generator`, method signature, shrunk `Request`,
  updated examples (single-shot and the multi-block loop).
- [ ] `docs/algorithm.md`: "`Generate` runs four stages" → note the parent is
  parsed in `New`; `resolveBits` is now a method.
- [ ] `docs/classifications.md`: the map is passed to `New`, not `Request`;
  update the resolution-order table caption and the code example.
- [ ] `docs/development.md` and `CLAUDE.md`: update the layout table row for
  `cidrgen.go` (`Request`, `Generator`, `New`, `Generate`, `resolveBits`) and
  the "Stateless" design-constraint bullet to "immutable after construction; no
  package-level state; no input mutation".
- [ ] `README.md`: update the two examples and the "Stateless" bullet.
- [ ] `ISSUES/004-size-resolution.md`: note `resolveBits` is now a method on
  `*Generator`; its "pure (no I/O, no parsing)" property is unchanged.
- [ ] `ISSUES/README.md`: add the row for 007.

## Test deliverable

Tests are updated in the same PR (API change → existing tests must move; new
behavior in `New` ships with its own cases).

`cidrgen_test.go`:

- `TestNew`:
  - valid parent, `nil` map → `*Generator`, no error.
  - parent with host bits set (`10.0.0.9/16`) → stored canonical (`10.0.0.0/16`);
    assert via a subsequent `Generate` on an empty pool returning `10.0.0.0/…`.
  - unparseable parent, non-IPv4 parent, `/33` → `errors.Is(err, ErrInvalidPrefix)`.
  - caller mutates the passed map after `New` → a later `Generate` still resolves
    the original classification (proves the shallow copy).
- `TestResolveBits`: table-driven over `*Generator` values instead of
  `(req, parentBits)`. Same cases as today — netmask only, classification only,
  netmask wins, neither (`ErrNoSizeSpecified`), unknown classification,
  `nil` map (`ErrUnknownClassification`), `bits == parentBits`,
  `bits < parentBits`, `bits > 32`, negative netmask (`ErrInvalidPrefix`).
- `TestGenerate`: same cases as today, rewritten as
  `cidrgen.New(parent, classes)` then `g.Generate(Request{…})`. The
  "unparseable parent" case moves to `TestNew`. Every success case still calls
  `assertFits`.
- New: `TestGeneratorConcurrent` — one `Generator`, N goroutines each running a
  `Generate` with its own `Request`, under `-race`; assert every result still
  satisfies `assertFits` against that goroutine's own `Allocated`.
- The multi-block sequence (carve a `/16` into 256 `/24`s, feed each result
  back) runs through one `Generator`; invariant holds every iteration; 257th
  call → `ErrNoSpace`.

`allocate_test.go`, `validate_test.go`, `parse_test.go`,
`classification_test.go`: unaffected (they test unexported helpers directly),
except `classification_test.go`'s integration case feeds `LoadClassifications`
output into `New` instead of `Request.Classifications`.

## Done when

- `go mod tidy && go vet ./... && go test ./... -race -count=1` all green.
- `go test ./... -cover` at or above its current level (~98%).
- No package-level `Generate`; `grep -rn 'Request{' docs README.md` shows no
  `Parent:` or `Classifications:` keys.
- Every doc file listed above reflects the `New` / method API.
