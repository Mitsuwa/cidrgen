# 009 — Tag and release workflow

## Context

The repo has no tags and no released version. `go get github.com/Mitsuwa/cidrgen`
resolves to a pseudo-version off `main`, and pkg.go.dev has nothing to index as a
real release — no version dropdown, no "License" line (there is no `LICENSE`
file), and the module is marked non-redistributable, which suppresses proxy
caching and full doc rendering.

The goal is a **well-indexed module on pkg.go.dev**: a proper `vX.Y.Z` tag on
every version bump, a recognized license, and the module proxy prompted to fetch
each new version immediately.

The mechanism is a `VERSION` file as the single source of truth. A contributor
edits `VERSION` in their PR; on merge to `main` a workflow reads it and, if a tag
for that version does not yet exist, runs the full test suite and then creates
the tag and a GitHub Release. Most PRs do not touch `VERSION` and produce no
release.

This is **infrastructure, not package behavior**. `cidrgen`, its API, and every
sentinel are unchanged, and there is no Go test to add. Per
[CLAUDE.md](../CLAUDE.md) it still ships as its own PR; the "test deliverable" is
the workflow's own first run on merge, which must tag `v0.1.0` and cut its
Release with the suite green.

Depends on **008** only in ordering (it is the next unit of work); no code
dependency.

## Design decisions (settled)

- **Release model: manual bump, auto-detect on merge.** The contributor bumps
  `VERSION` in their PR — a visible line in the diff, reviewed like any other
  change. No conventional-commit automation, no `release-please`, no
  `workflow_dispatch` bump. `VERSION` is the literal source of truth.
- **`VERSION` format: bare semver + trailing newline.** `0.1.0`, nothing else in
  the file — no `v` prefix, no changelog block. The tag is `v` + the trimmed
  file contents.
- **Detection: tag-existence check, not a diff.** The workflow reads `VERSION`
  and acts only if tag `v<VERSION>` does not already exist. Idempotent: a re-run,
  a no-op push, or a merge that does not touch `VERSION` all do nothing. Handles
  the bootstrap case (no tags yet) with no special-casing.
- **Self-contained test gate.** `release.yml` re-runs `go vet ./...` and
  `go test ./... -race -count=1` itself before tagging. No `workflow_run`
  chaining off `test.yml`. This closes the gap where a direct push to `main`
  could otherwise tag an untested tree.
- **Validation is a hard failure.** If `VERSION` is not valid semver, or if
  `v<VERSION>` sorts at or below the latest existing tag (a backwards bump), the
  workflow fails instead of creating a bad tag.
- **Annotated tag, `github-actions[bot]`, `GITHUB_TOKEN`.** `permissions:
  contents: write`, a `concurrency` group so two quick merges cannot double-tag.
  Tag and Release are created together via `gh release create`.
- **GitHub Release with auto-generated notes.** Normal release, not a
  pre-release — pkg.go.dev and `go get` treat `v0.1.0` as a real version
  regardless, and the `0.` already signals an unstable API. Notes are
  auto-generated from merged PR titles.
- **Force pkg.go.dev indexing.** A final step requests the module proxy
  (`https://proxy.golang.org/github.com/!mitsuwa/cidrgen/@v/v<VERSION>.info`,
  lowercased with `!`-escaped capitals) and pkg.go.dev, so a new version appears
  within seconds rather than on the next poll.
- **`LICENSE`: MIT, `Copyright (c) 2026 Bryan Nobuhara`.** SPDX-recognized so
  pkg.go.dev detects it and marks the module redistributable.
- **`test.yml` narrows to `pull_request` only.** `release.yml` is the
  authoritative run for `main`; a red `release.yml` on `main` is as visible as a
  red `test.yml` would be, and one run per merge is enough.
- **First tag is cut by the workflow.** Merging this PR adds `VERSION` = `0.1.0`
  with no prior tag; the workflow's first run tags `v0.1.0` and cuts its Release.
  No manual bootstrap tag.
- **Deferred to 010.** `doc.go` review, converting the README usage snippets
  into testable `Example` functions, and a pkg.go.dev badge — the doc-surface
  polish — is a separate follow-up PR, kept out of this one.

## Tasks

- [ ] `VERSION`: `0.1.0` plus a single trailing newline. Nothing else.
- [ ] `LICENSE`: MIT, `Copyright (c) 2026 Bryan Nobuhara`.
- [ ] `.github/workflows/release.yml`, on `push` to `main`:
  - `permissions: contents: write`; `concurrency` group keyed on the workflow so
    overlapping runs serialize.
  - Checkout with full history (`fetch-depth: 0`) and tags, so tag comparison
    works.
  - `actions/setup-go@v5` with `go-version-file: go.mod`.
  - `go vet ./...` then `go test ./... -race -count=1`.
  - Read `VERSION` into `VERSION` (trimmed). Validate it is `MAJOR.MINOR.PATCH`
    semver; fail otherwise.
  - If tag `v$VERSION` already exists (`git tag --list` / `git rev-parse`), exit
    0 without tagging.
  - Otherwise validate `v$VERSION` sorts strictly above the latest existing tag
    (`git tag --sort=-v:refname | head -1`, `sort -V` comparison); fail on a
    backwards or equal bump.
  - Configure git as `github-actions[bot]`, create an **annotated** tag
    `v$VERSION`, push it.
  - `gh release create v$VERSION --generate-notes --title v$VERSION` (normal
    release, not `--prerelease`), using `GITHUB_TOKEN`.
  - Final step: `curl -fsS` the module proxy `@v/v$VERSION.info` endpoint and
    `https://pkg.go.dev/github.com/Mitsuwa/cidrgen@v$VERSION`; do not fail the
    job if these are slow to respond (`|| true` on the pkg.go.dev hit).
- [ ] `.github/workflows/test.yml`: change `on:` from `push: [main]` +
      `pull_request` to `pull_request` only.
- [ ] `docs/releasing.md`: how a release is cut — bump `VERSION` in a PR, merge,
      the workflow tags `v<VERSION>` and cuts the Release; the idempotency and
      validation rules; how the module proxy / pkg.go.dev pick it up; how to fix
      a mistaken bump (delete the unreleased tag before the next merge).
- [ ] `README.md`: add a short "API is not yet stable (v0.x)" note; link
      `docs/releasing.md` in the Documentation table.
- [ ] `docs/README.md`: add the `releasing.md` row.
- [ ] `docs/development.md`: add a "Releasing" pointer to `docs/releasing.md`
      near the CI section.
- [ ] `ISSUES/README.md`: add the row for 009.
- [ ] Update `CLAUDE.md` layout table if the new `docs/` file warrants a mention
      (release process now lives in `docs/releasing.md`).

## Test deliverable

No Go test — this changes no package behavior. The equivalent gate:

- `release.yml` and the edited `test.yml` are valid YAML and pass
  `actionlint` if available locally.
- The `VERSION` → tag → validation logic is exercised as a shell snippet in the
  PR description (or a scratch run) against the three cases: tag absent (tags),
  tag present (no-op), backwards bump (fails).
- On merge, the workflow's first run tags `v0.1.0`, cuts the `v0.1.0` GitHub
  Release with generated notes, and the proxy request returns the version info.
  `go test ./... -race` is green in that run.

## Done when

- `VERSION`, `LICENSE`, `release.yml` are on `main`; `test.yml` runs on PRs only.
- Merging the PR produces tag `v0.1.0` and a matching GitHub Release, created by
  the workflow, with the suite green.
- `https://pkg.go.dev/github.com/Mitsuwa/cidrgen` shows `v0.1.0` in the version
  selector and an "MIT" license line within a few minutes of merge.
- `docs/releasing.md` exists and is linked from `README.md`, `docs/README.md`,
  and `docs/development.md`.
