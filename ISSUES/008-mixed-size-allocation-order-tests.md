# 008 — Mixed-size sequential allocation: order tests

## Context

The suite has two sequential tests today, and both carve a pool into a single
uniform block size:

- `TestGenerateSequential` — a `/16` into 256 `/24`s through one `Generator`,
  feeding each result back into `Request.Allocated`.
- `TestFirstFitSequential` — the same shape one layer down, against `firstFit`.

Nothing exercises a **single `Generator` handling requests of different sizes in
sequence**, where each result is appended to `Allocated` and changes where the
next block can land. That is the normal way a caller uses the package: one
long-lived `Generator`, a growing allocation list, and a mix of block sizes and
classifications from call to call.

The interesting behavior this misses:

- A small block allocated at a low address forces a later, larger request to
  **re-align past it** — a `/32` at `10.0.0.0` pushes a subsequent `/16` to
  `10.1.0.0`.
- `/32` requests interleaved with `/16` and `/24` requests, all first-fit
  lowest, all correctly aligned to their own size.
- `Request.Classification` and `Request.Netmask` used on different calls to the
  same `Generator`.
- Top-of-address-space arithmetic under a `0.0.0.0/0` parent with large blocks
  (`/1`, `/2`, `/4`, `/10`) — the `uint64` range math in `prefixRange` /
  `firstFit` that only a single `TestFirstFit` case (`0.0.0.0/0`, one `/1`)
  touches today.
- A sequence that fills its parent exactly and then returns `ErrNoSpace`,
  including the case where even a `/32` no longer fits.
- `Request.Allocated` entries whose address is **not** on the network boundary
  for their prefix length (host bits set, e.g. `10.0.0.5/25`). `parsePrefix`
  runs `Masked()` on every entry, so the scan must see `10.0.0.0/25`. Today only
  one one-shot `TestGenerate` case (`10.0.0.5/24`) covers this, and its expected
  result is the same whether the entry is masked or not — it does not actually
  pin the behavior.

This is a **test-only** issue. `Generator`, `Generate`, `firstFit`,
`validateAllocated`, and every sentinel are unchanged. Per
[CLAUDE.md](../CLAUDE.md) rule 2 this still ships as its own PR with its test
deliverable; per rule 3 every success step asserts the invariant via
`assertFits`.

Depends on **007** (the `New` / `Generator` / method API these tests drive).

## Design decisions (settled)

- **One new test**, `TestGenerateMixedSizeSequence`, in `cidrgen_test.go`.
  `TestGenerateSequential`, `TestFirstFitSequential`, and every other existing
  test are left exactly as they are — the uniform-carve tests keep their single
  clear purpose.
- **`Generate`-level only.** No new `allocate_test.go` sequential case:
  `firstFit` / `validateAllocated` are unexported helpers already covered by
  `TestFirstFitSequential`, and a mixed-size test there would duplicate this one
  at lower fidelity to the real API.
- **Table-driven, one row per scenario.** Each row is
  `{name, parent, classes, seed []string, steps []step}`. `seed` holds
  host-bit-set entries that stay in `Allocated` for every step of the row (empty
  for rows that do not exercise masking). A row runs its steps in order through
  **one** `Generator`, threading a `[]string` of results that starts as a copy
  of `seed`:

  ```go
  type step struct {
      netmask        int
      classification string
      want           string // canonical CIDR; empty when wantErr is set
      wantErr        error
  }
  ```

  Row loop:

  ```go
  g, err := New(tt.parent, tt.classes)
  // fatal on err
  allocated := append([]string(nil), tt.seed...)
  seen := map[string]bool{}
  for i, s := range tt.steps {
      got, err := g.Generate(Request{
          Allocated:      allocated,
          Netmask:        s.netmask,
          Classification: s.classification,
      })
      if s.wantErr != nil {
          if !errors.Is(err, s.wantErr) {
              t.Fatalf("step %d: error = %v, want %v", i, err, s.wantErr)
          }
          break // error step is always last in a row
      }
      if err != nil {
          t.Fatalf("step %d: unexpected error: %v", i, err)
      }
      if got.String() != s.want {
          t.Fatalf("step %d: Generate = %s, want %s", i, got, s.want)
      }
      assertFits(t, g.parent, mustPrefixes(t, allocated...), got)
      if s.classification != "" && got.Bits() != tt.classes[s.classification] {
          t.Fatalf("step %d: got /%d, want /%d for %q",
              i, got.Bits(), tt.classes[s.classification], s.classification)
      }
      if seen[got.String()] {
          t.Fatalf("step %d: %s allocated twice", i, got)
      }
      seen[got.String()] = true
      allocated = append(allocated, got.String())
  }
  ```

- **`Allocated` is fed back exactly like `TestGenerateSequential`.** Append the
  canonical `got.String()` in allocation order; no shuffling, no pre-sorting.
  Unsorted / overlapping inputs stay out of this test — they are already
  one-shot cases in `TestGenerate` — so each failure here is unambiguous.
- **Masking rows use `seed`, not per-step inputs.** `got.String()` is always
  canonical, so the only way host bits enter `Allocated` is a `seed` entry. Seed
  entries persist across the whole row and are re-masked by `parsePrefix` on
  every `Generate` call. A masking row's `want` values are those of the masked
  seed; picked so a Generator that scanned the raw address would return a
  different block (or `ErrNoSpace`), which pins the `Masked()` behavior rather
  than merely tolerating it.
- **Every success step asserts four things** (in the loop above): exact
  `want` match, `assertFits` (inside parent, aligned to own size, overlaps
  nothing in `allocated`), a cross-step `seen` map catching any address handed
  out twice, and — for classification steps — `got.Bits()` equals the mapped
  prefix length.
- **Classification map** for the rows that use one:
  `map[string]int{"region": 16, "zone": 24, "host": 32}`. The existing
  `datanode`/`computenode` map (28 / 27) is too small to sit under a `/8` next
  to `/16`s and is left alone. Classification rows therefore only appear under
  `/8` parents (a `/16` classification cannot fit a `/24`-or-longer parent).
- **Error rows** only under a parent that can actually be filled
  (`10.0.0.0/24`). The `/8` and `0.0.0.0/0` parents are success-only — they
  cannot be exhausted in a test loop.

## Scenario rows

Each expected sequence below was traced through `firstFit` in `allocate.go`
(first-fit, lowest address, re-aligned up to the block size past each
allocation). `Size` is the `Netmask` value, or the classification name whose
mapped length is shown in parentheses.

| # | Parent | Seed (masked form) | Steps (size) | Expected sequence |
|---|---|---|---|---|
| 1 | `10.0.0.0/8` | — | `/32, /16, /24, /32, /16` | `10.0.0.0/32`, `10.1.0.0/16`, `10.0.1.0/24`, `10.0.0.1/32`, `10.2.0.0/16` |
| 2 | `100.0.0.0/8` | — | `host(/32), region(/16), zone(/24), host(/32), region(/16)` | `100.0.0.0/32`, `100.1.0.0/16`, `100.0.1.0/24`, `100.0.0.1/32`, `100.2.0.0/16` |
| 3 | `10.0.0.0/8` | — | `zone(/24), /32, region(/16), /24, host(/32)` | `10.0.0.0/24`, `10.0.1.0/32`, `10.1.0.0/16`, `10.0.2.0/24`, `10.0.1.1/32` |
| 4 | `10.0.0.0/16` | — | `/24, /32, /24, /32, /24` | `10.0.0.0/24`, `10.0.1.0/32`, `10.0.2.0/24`, `10.0.1.1/32`, `10.0.3.0/24` |
| 5 | `10.0.0.0/24` | — | `/25, /26, /26, /32` | `10.0.0.0/25`, `10.0.0.128/26`, `10.0.0.192/26`, then `ErrNoSpace` |
| 6 | `0.0.0.0/0` | — | `/1, /32, /2` | `0.0.0.0/1`, `128.0.0.0/32`, `192.0.0.0/2` |
| 7 | `0.0.0.0/0` | — | `/4, /10, /4, /10` | `0.0.0.0/4`, `16.0.0.0/10`, `32.0.0.0/4`, `16.64.0.0/10` |
| 8 | `0.0.0.0/0` | — | `/10, /4, /2, /32` | `0.0.0.0/10`, `16.0.0.0/4`, `64.0.0.0/2`, `0.64.0.0/32` |
| 9 | `10.0.0.0/24` | `10.0.0.5/25` → `10.0.0.0/25` | `/26, /26, /25` | `10.0.0.128/26`, `10.0.0.192/26`, then `ErrNoSpace` |
| 10 | `10.1.2.3/8` → `10.0.0.0/8` | `10.0.0.200/24` → `10.0.0.0/24`; `10.1.50.99/16` → `10.1.0.0/16` | `/24, /16, /32` | `10.0.1.0/24`, `10.2.0.0/16`, `10.0.2.0/32` |
| 11 | `100.0.0.0/8` | `100.0.0.77/24` → `100.0.0.0/24` | `zone(/24), host(/32), region(/16)` | `100.0.1.0/24`, `100.0.2.0/32`, `100.1.0.0/16` |

What each row is for:

- **1** — netmask-only; the `/32` at `10.0.0.0` forces step 2's `/16` to
  `10.1.0.0`; step 3's `/24` fills the low gap before `10.1.0.0/16`; step 4's
  `/32` takes `10.0.0.1`; step 5's `/16` re-aligns past everything to
  `10.2.0.0`.
- **2** — same shape as row 1 driven entirely by `Classification`; also asserts
  `got.Bits()` matches `region`/`zone`/`host` on every step. Different parent
  (`100.0.0.0/8`) to prove the base address is not special-cased.
- **3** — `Netmask` and `Classification` steps interleaved through one
  `Generator`; the `/24` at base forces the `region` (`/16`) to `10.1.0.0`.
- **4** — the row-1 re-alignment idea at `/24`/`/32` scale under a tight `/16`
  parent; `/32`s pack into `10.0.1.0` then `10.0.1.1` while `/24`s march
  `10.0.0.0`, `10.0.2.0`, `10.0.3.0`.
- **5** — fills `10.0.0.0/24` exactly (`128 + 64 + 64` addresses) and then a
  `/32` request still returns `ErrNoSpace`.
- **6** — `0.0.0.0/0` with a `/1`, a `/32`, and a `/2`; the trailing `/2` lands
  at `192.0.0.0` (`end == 0xFFFFFFFF`), exercising the `uint64` guard.
- **7** — `/4` and `/10` interleaved; step 2's `/10` slots directly above
  `0.0.0.0/4` at `16.0.0.0`; step 3's `/4` re-aligns past the `/10` to
  `32.0.0.0`; step 4's `/10` fills the gap at `16.64.0.0` before `32.0.0.0/4`.
- **8** — a `/10` at base, then `/4` re-aligns to `16.0.0.0`, then `/2` to
  `64.0.0.0`, then a `/32` drops into the low gap at `0.64.0.0`.
- **9** — masking discriminator. Seed `10.0.0.5/25` masks to `10.0.0.0/25` =
  `[0,127]`, so step 1's `/26` lands at `10.0.0.128/26`. A scan of the raw
  `[5,132]` would re-align to `10.0.0.192/26` instead. Step 3's `/25` then finds
  the pool full → `ErrNoSpace`.
- **10** — host bits set in the parent (`10.1.2.3/8`) and both seeds; masked
  forms are `10.0.0.0/8`, `10.0.0.0/24`, `10.1.0.0/16`. Step 2's `/16` re-aligns
  past the seeded `/24` and `/16` to `10.2.0.0`; step 3's `/32` drops into the
  low gap at `10.0.2.0`.
- **11** — a host-bit seed (`100.0.0.77/24` → `100.0.0.0/24`) feeding
  classification steps; also asserts `got.Bits()` per step.

## Tasks

- [ ] `cidrgen_test.go`: add the `step` type, the `seed` row field, and
  `TestGenerateMixedSizeSequence` with the eleven rows above and the row loop
  from "Design decisions".
- [ ] `docs/development.md`: under "Testing patterns", add a bullet next to the
  `TestFirstFitSequential` note:
  > - `TestGenerateMixedSizeSequence` drives one `Generator` through an ordered
  >   mix of `/16`–`/32` requests (and classifications), feeding each result
  >   back into `Allocated`, and asserts the exact block returned at every step
  >   plus the `assertFits` invariant.
- [ ] `ISSUES/README.md`: add the row for 008 (depends on 007).

No source files, no other docs: behavior and design constraints are unchanged.

## Test deliverable

`TestGenerateMixedSizeSequence` in `cidrgen_test.go`, as specified above:
table-driven, one `Generator` per row, results threaded through
`Request.Allocated` on top of the row's `seed`, every success step asserting
`want` + `assertFits` + no-double-allocation + (for classification steps)
`got.Bits()`. Rows 5 and 9 end in `errors.Is(err, ErrNoSpace)`; rows 9–11 carry
host-bit `seed` entries whose `want` values only hold if the Generator masks
them.

## Done when

- `go mod tidy && go vet ./... && go test ./... -race -count=1` all green.
- `go test ./... -cover` at or above its current level (~98%).
- `TestGenerateMixedSizeSequence` runs all eleven rows; each row's expected
  sequence matches without adjustment (if a traced value is wrong, fix the
  table, not the algorithm — there is no source change in this issue).
- `docs/development.md` and `ISSUES/README.md` updated as listed.
