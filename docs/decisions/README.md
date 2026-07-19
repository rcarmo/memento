# Architecture decisions

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-keep-operation-worktrees.md) | Assemble canonical writes in detached Git worktrees | accepted |
| [0002](0002-needle-feasibility.md) | Use fine-tuned Needle only as an embedded shallow router | accepted, opt-in |
| [0003](0003-separate-knowledge-control-and-derived-state.md) | Separate Git knowledge, SQLite control state and rebuildable indexes | accepted |
| [0004](0004-use-proposals-for-shared-writes.md) | Use proposals and curator review for shared writes | accepted |
| [0005](0005-use-umcp-streamable-http.md) | Use uMCP and Streamable HTTP as the MCP boundary | accepted |
| [0006](0006-keep-lexical-search-primary.md) | Keep lexical search primary and semantic ranking optional | accepted |
| [0007](0007-attach-generic-versioned-assets.md) | Attach generic versioned assets to ordinary concepts | accepted |
| [0008](0008-build-for-baseline-cpus.md) | Build release images for baseline CPUs and select SIMD at runtime | accepted |
| [0009](0009-run-gte-in-batched-short-lived-workers.md) | Run GTE in mmap-backed, batched short-lived workers | accepted |
| [0010](0010-use-tiled-matrix-kernels-for-local-inference.md) | Use tiled matrix kernels for GTE and Needle inference | accepted |

## Discussed but not adopted

* **Gitea browser/mirror:** a one-way read-only mirror is possible, but no mirror worker or repository policy has been implemented.
* **Autonomous model curator:** tested with a local Gemma model and rejected for state-changing review or apply operations.
* **Automatic skill installation:** rejected; Memento stores and recalls packs, while the client decides whether to import them.
