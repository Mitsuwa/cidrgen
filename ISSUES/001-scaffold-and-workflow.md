# 001 — Scaffold and CI workflow

## Context

Empty repo. Before any behavior lands we need the module, the package skeleton,
the working agreement, and CI that actually runs the tests — so every subsequent
PR is enforced from day one.

## Tasks

- [ ] `go.mod`: `module github.com/Mitsuwa/cidrgen2`, `go 1.27`, `gopkg.in/yaml.v3`.
- [ ] `doc.go`: package doc comment for `cidrgen`.
- [ ] `CLAUDE.md`: the test-first / tests-with-every-PR rule and the design
      constraints.
- [ ] `README.md`: usage sketch.
- [ ] `.gitignore`: build/test artifacts, scratch `main.go`.
- [ ] `.github/workflows/test.yml`: on push to `main` and all PRs, run
      `go mod tidy` + `git diff --exit-code`, `go vet ./...`, and
      `go test ./... -race -count=1`.
- [ ] `ISSUES/` populated with this breakdown.

## Test deliverable

`cidrgen_test.go` with a single trivial passing test (e.g. asserts the package
compiles / a placeholder), so CI is green on the first push.

## Done when

`go test ./...` passes locally and the workflow file is valid YAML.
