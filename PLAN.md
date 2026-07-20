# Memento delivery plan

**Python:** 3.12-3.14
**Architecture:** [`docs/implementation.md`](docs/implementation.md)

Memento's repository, transaction, MCP, proposal, search, model and container foundations are in place. This file tracks current work and the few remaining deployment gaps; the architecture documents and Git history hold the completed milestone detail.

## Working Rules

* Shared concepts are Markdown in Git. Operation and proposal records live in `control.sqlite`; search, graph and embedding data can be rebuilt.
* Mutations carry an expected revision and idempotency key, run through the writer lease and update the readable checkout and indexes before returning.
* Search filters by the caller's namespace before ranking.
* Models may route, retrieve, answer or draft proposals. Service code checks their output and performs any resulting operation.
* `make check`, wheel installation and `git diff --check` are required before release. Container changes also run the multi-architecture and no-AVX image checks.

## Available Today

### Repository And Writes

* Strict concept schema, stable IDs, links, path containment and repository audit
* Git worktree transactions with compare-and-swap publication and restart recovery
* Proposal review/apply plus curator create, patch and rename
* Versioned Git LFS asset packs and complete skill recall
* Writer lease, idempotent replay, stale-write conflicts, backups and restore

### Retrieval And MCP

* Authenticated Streamable HTTP through uMCP
* Compact and full tool surfaces, catalog/workflow resources and `memory_execute`
* FTS5 search, backlinks, graph neighbourhoods and index rebuild/parity checks
* Local GTE semantic and hybrid search with short-lived batched workers
* Fine-tuned Needle shallow routing through the Rust runtime
* Cited answers, exact cache, hot memory, proposal drafting and Dream modes behind independent settings

### Operations

* Non-root multi-architecture container with read-only root and one writable state mount
* Structured logs, metrics, health/readiness, graceful drain and recovery
* GHCR release pipeline for amd64 and arm64, including Westmere scalar inference
* Portainer deployment on the Intel J3455 DiskStation

## Visual Memory Debugger

[ADR 0011](docs/decisions/0011-embed-a-gated-visual-memory-debugger.md) records the decision, and [`docs/graph-explorer-plan.md`](docs/graph-explorer-plan.md) contains the API, rendering and release details. The Plan sidebar tracks the active implementation phase.

The current work adds a trusted-LAN `/graph` view with progressive 2.5D rendering, provenance, explicit and semantic layers, diagnostics, embedding refresh and bounded exports. Revision playback and animated diffs follow after the current-state view.

## Remaining Live Work

* Complete and deploy the visual debugger, then capture desktop and tablet screenshots plus browser performance results.
* Exercise distinct deployed principals through the write/proposal workflow.
* Repeat model performance checks on a real ARM64 host.
* Run a live restore drill for the selected primary deployment path.
* Attach SBOM material to published releases.

## Later

* Revision playback and animated graph diffs
* Split comparison between relationship/force configurations
* Standalone interactive graph export
* ARM64 embedded-runtime measurements for Needle
