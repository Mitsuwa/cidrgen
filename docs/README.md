# cidrgen documentation

`cidrgen` allocates non-overlapping IPv4 CIDR blocks from a parent pool.

| Document | Contents |
|---|---|
| [design.md](design.md) | The problem, the settled design decisions, and why each was made |
| [api.md](api.md) | `Request`, `Generate`, `LoadClassifications`, the sentinel errors |
| [algorithm.md](algorithm.md) | How first-fit allocation works: alignment, the scan, `uint32` arithmetic |
| [classifications.md](classifications.md) | The classification YAML format and how a request's size is resolved |
| [development.md](development.md) | Local workflow, the test-first rule, CI |
| [releasing.md](releasing.md) | How `VERSION` becomes a tag and a pkg.go.dev release |

Start with [design.md](design.md) for the mental model, then [api.md](api.md) to use it.

See also the top-level [README.md](../README.md) for a quick usage sketch,
[CLAUDE.md](../CLAUDE.md) for the working agreement, and [ISSUES/](../ISSUES/) for
the planned work.
