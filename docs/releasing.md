# Releasing

`cidrgen` is versioned by a single file, [`VERSION`](../VERSION), and released by
[`.github/workflows/release.yml`](../.github/workflows/release.yml). The goal is a
well-indexed module on [pkg.go.dev](https://pkg.go.dev/github.com/Mitsuwa/cidrgen):
a real `vX.Y.Z` tag per version, a recognized license, and the module proxy
prompted to fetch each version immediately.

The API is **not yet stable** — every `v0.x` release may change it.

## Cutting a release

1. In a PR, edit `VERSION` to the new version. It is bare semver, no `v` prefix,
   one line, trailing newline:

   ```
   0.2.0
   ```

2. Merge the PR to `main`.
3. `release.yml` runs on the push to `main`:
   - `go vet ./...` and `go test ./... -race -count=1`.
   - Reads `VERSION`, forms the tag `v<VERSION>`.
   - If that tag already exists, the job exits without doing anything.
   - Otherwise it creates an annotated tag `v<VERSION>` as `github-actions[bot]`,
     pushes it, and runs `gh release create` with auto-generated notes from the
     merged PR titles.
   - Finally it requests the version from `proxy.golang.org` and hits
     `pkg.go.dev` so indexing happens within seconds rather than on the next poll.

Most PRs do not touch `VERSION`; on those merges the job runs the tests, finds
the tag already present, and exits.

## Rules the workflow enforces

- **Valid semver.** `VERSION` must be `MAJOR.MINOR.PATCH`. Anything else fails
  the job before tagging.
- **Forward only.** `v<VERSION>` must sort strictly above the highest existing
  `v*` tag. A backwards or repeated version fails the job.
- **Idempotent.** The release is keyed on tag existence, not on a diff of
  `VERSION`. Re-running the workflow, or a push that does not change `VERSION`,
  is a no-op.

## Fixing a mistake

- **Wrong version merged, not yet tagged** (job failed validation): open a
  follow-up PR correcting `VERSION` and merge it. The next run tags the corrected
  version.
- **Wrong version already tagged and released:** delete the GitHub Release and
  the tag (`git push origin :refs/tags/vX.Y.Z`), then bump `VERSION` to a new,
  higher version in a PR. Do not re-point an existing tag — the module proxy
  caches tag contents and will keep serving the original.

## Version selection

Pre-`v1.0.0`: bump the patch for fixes, the minor for anything else (there is no
API-compatibility guarantee yet). At `v1.0.0` and beyond, follow semver against
the exported API in [api.md](api.md). A `v2+` major version additionally requires
a `/v2` module path suffix in `go.mod` and imports — out of scope until it
happens.
