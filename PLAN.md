# Memento delivery plan

**Python:** 3.12-3.14
**Architecture:** [`docs/implementation.md`](docs/implementation.md)

Memento's repository, transaction, MCP, proposal, search, model, debugger and container foundations are in place. This file keeps the remaining engineering and operational gaps together; the architecture documents and Git history hold completed milestone detail.

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
* Proposal review/apply, including authorised curator self-review, plus curator create, patch and rename
* Versioned Git asset packs, MCP-native base64 publication, raw-upload staging and complete skill recall
* Writer lease, idempotent replay, stale-write conflicts, backups and restore

### Retrieval And MCP

* Authenticated Streamable HTTP through uMCP
* Compact and full tool surfaces, catalog/workflow resources and `memory_execute`
* Bounded namespace inventory, execute-only local-manifest comparison, and generic asset metadata/parity inspection without server-side local file access or ZIP retrieval
* FTS5 search, backlinks, graph neighbourhoods and index rebuild/parity checks
* Local GTE semantic and hybrid search with persistent progressive state and short-lived low-priority single-item workers
* Fine-tuned Needle shallow routing through the Rust runtime
* Cited answers with versioned authorisation-scoped evidence, secret-first abstention, exact cache and bounded relational support chains, plus hot memory, proposal drafting and Dream modes behind independent settings

### Operations

* Non-root multi-architecture container with read-only root and one writable state mount
* Structured logs, metrics, health/readiness, graceful drain and recovery
* GHCR release pipeline for amd64 and arm64, including Westmere scalar inference
* Healthy immutable `0.3.26` Portainer deployment on the Intel J3455 DiskStation, with persistent POST sessions and SSE notification delivery recorded in [`docs/evidence/release-0.3.26.md`](docs/evidence/release-0.3.26.md)

### Visual Memory Debugger

* Trusted-LAN `/graph` view with progressive 2.5D rendering, provenance, explicit and semantic layers, diagnostics, embedding refresh and bounded exports
* Browser-native Three.js/Preact client with desktop, tablet and 2,000-node fixture checks
* Current-state graph deployed on the DiskStation profile; [ADR 0011](docs/decisions/0011-embed-a-gated-visual-memory-debugger.md) records the boundary and [`docs/graph-explorer-plan.md`](docs/graph-explorer-plan.md) keeps the implemented API plus deferred work

## Remaining Live Work

* Repeat model performance checks on a real ARM64 host.
* Enforce or explain the missing production PIDs limit requested by the DiskStation Compose profile.
* Decide when to enable protected read prefixes on the existing DiskStation configuration and migrate broad-reader grants explicitly.
* Run a live restore drill for the selected primary deployment path.
* Attach SBOM material to published releases.
* Add a TLS reverse proxy before exposing any HTTP surface beyond the trusted LAN.

## Later

* Revision playback and animated graph diffs
* Split comparison between relationship/force configurations
* Standalone interactive graph export
* ARM64 embedded-runtime measurements for Needle
