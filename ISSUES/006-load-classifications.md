# 006 — LoadClassifications YAML helper

## Context

Callers keep the classification → prefix-length map in a YAML file loaded at
startup. Ship a helper so they do not each reinvent the parsing, and so the
schema is enforced in one place.

## Tasks

- [ ] `classification.go`: `LoadClassifications(r io.Reader) (map[string]int, error)`
  - Decode a document shaped as:
    ```yaml
    classifications:
      datanode: 28
      computenode: 27
    ```
  - `yaml.Decoder` with `KnownFields(true)` — unknown top-level keys are errors
    (catches `classification:` typos).
  - Empty document → error; missing `classifications` key → error.
  - Non-integer value → error (surfaced by the YAML decoder).
  - Value outside `0..32` → error.
- [ ] Add `gopkg.in/yaml.v3` to `go.mod` (only dependency).

## Test deliverable

`classification_test.go` + `testdata/classifications.yaml`:

- fixture file round-trips to the expected `map[string]int`.
- error cases: empty, missing key, unknown top-level key, non-integer value,
  prefix length `> 32`, negative, malformed YAML.
- integration: `LoadClassifications` output fed into `Generate` via
  `Request.Classifications` produces the expected block.

## Done when

`go test ./...` green; `go mod tidy` leaves `go.mod`/`go.sum` unchanged.
