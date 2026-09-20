# Memento delivery plan

**Release:** pure-Go `v1.0.0`, Linux amd64-v1 and arm64
**Architecture:** [`docs/implementation.md`](docs/implementation.md)

Memento's repository, transaction, MCP, proposal, search, model, debugger and container foundations are in place. This file keeps the remaining engineering and operational gaps together; the architecture documents and Git history hold completed milestone detail.

## Pure Go replacement branch

The end-to-end pure-Go replacement is complete and verified on `main`. [Port index](docs/go-port/README.md), [parity matrix](docs/go-port/parity.md), [test gates](docs/go-port/testing.md) and [SIMD results](docs/go-port/simd.md) record the accepted boundaries. The daemon, standalone uMCP module, storage/recovery, GTE, Needle, intelligent service handlers and release tooling build with `CGO_ENABLED=0`; checked-in fixtures preserve the pinned Python/Rust oracle results without retaining either runtime. Production replacement is still a separately authorised release operation.

## Working Rules

* Shared concepts are Markdown in Git. Operation and proposal records live in `control.sqlite`; search, graph and embedding data can be rebuilt.
* Mutations carry an expected revision and idempotency key, run through the writer lease and update the readable checkout and indexes before returning.
* Search filters by the caller's namespace before ranking.
* Models may route, retrieve, answer or draft proposals. Service code checks their output and performs any resulting operation.
* `make quality`, `make audit`, `make performance`, `make model-test`, `make corpus-test`, `make release-check`, the container contract and `git diff --check` are required before release. `make audit` is model-independent; `make performance` runs the pinned real-model allocation gates after model preparation. CI also runs native multi-architecture model jobs and the no-AVX image check.

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
* Fine-tuned Needle shallow routing through the pure-Go NDL1/SentencePiece runtime
* Cited answers with versioned authorisation-scoped evidence, secret-first abstention, exact cache and bounded relational support chains, plus hot memory, proposal drafting and Dream modes behind independent settings

### Operations

* Non-root multi-architecture container with read-only root and one writable state mount
* Structured logs, metrics, health/readiness, graceful drain and recovery
* GHCR release pipeline for amd64 and arm64, including Westmere scalar inference
* Healthy immutable `0.5.9` Portainer deployment on the Intel J3455 DiskStation, with preserved Git/control/derived state and resumed semantic-worker progress recorded in [`docs/evidence/release-0.5.9.md`](docs/evidence/release-0.5.9.md); the earlier persistent POST/SSE acceptance remains in [`docs/evidence/release-0.3.26.md`](docs/evidence/release-0.3.26.md)

### Visual Memory Debugger

* Trusted-LAN `/graph` view with progressive 2.5D rendering, provenance, explicit and semantic layers, diagnostics, embedding refresh and bounded exports
* Browser-native Three.js/Preact client with desktop, tablet and 2,000-node fixture checks
* Current-state graph deployed on the DiskStation profile; [ADR 0011](docs/decisions/0011-embed-a-gated-visual-memory-debugger.md) records the boundary and [`docs/graph-explorer-plan.md`](docs/graph-explorer-plan.md) keeps the implemented API plus deferred work

## Remaining Live Work

* Enforce or explain the missing production PIDs limit requested by the DiskStation Compose profile.
* Decide when to enable protected read prefixes on the existing DiskStation configuration and migrate broad-reader grants explicitly.
* Run a live restore drill for the selected primary deployment path.
* Add a TLS reverse proxy before exposing any HTTP surface beyond the trusted LAN.

## Later

* Revision playback and animated graph diffs
* Split comparison between relationship/force configurations
* Standalone interactive graph export
* Keep the NAS CPU-only, including the Go replacement; retain the Mesa 22 results as reference and do not package, enable or retest NAS Vulkan without an explicit reversal
* Evaluate the Mali-G720-Immortalis Vulkan path on `orangepi6plus.local` separately from the accepted CPU baseline if separately authorised
