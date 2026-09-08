# Issues

Ordered, test-first breakdown of the work. Each file is one PR-sized unit. Later
issues depend on earlier ones. Every issue lists an explicit **Test deliverable**;
per [CLAUDE.md](../CLAUDE.md) a PR does not merge without it.

| # | Title | Depends on |
|---|---|---|
| 001 | Scaffold and CI workflow | — |
| 002 | Domain types, parsing, normalization | 001 |
| 003 | Input validation: containment and overlap | 002 |
| 004 | Size resolution: netmask and classification | 002 |
| 005 | First-fit allocation | 003, 004 |
| 006 | LoadClassifications YAML helper | 004 |
