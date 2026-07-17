# Memento delivery plan

**Status:** Proposed  
**Architecture:** [docs/implementation.md](docs/implementation.md)  
**Initial scope:** Deterministic shared-memory MCP service  
**Python:** 3.10–3.12

This file is the current delivery and acceptance record. Architecture, rationale and longer design notes stay in [docs/implementation.md](docs/implementation.md).

## Delivery rules

* Complete milestones in order unless a prerequisite can be isolated safely.
* Keep optional model tiers out of the runtime until the deterministic service is proven.
* Every mutation path must preserve authorization, expected revision, idempotency, audit and crash recovery.
* Each milestone ends with `make check`, `make typecheck`, package installation evidence and the milestone-specific tests.
* Mark behaviour implemented only after tests pass; mark it deployed or live-verified only with deployment evidence.

## Milestone 0 -- contracts and repository bootstrap

- [x] Add project instructions and local Python guidance.
- [x] Preserve the detailed implementation document.
- [x] Create this executable delivery plan.
- [x] Define concept schema v1 and strict validation models.
- [x] Define canonical Markdown fixtures and controlled vocabularies.
- [x] Document MCP tools, envelopes, errors and pagination.
- [x] Add threat model and trust-boundary diagram.
- [x] Add sample knowledge bundle.
- [x] Add MIT licence.
- [x] Add Docker, Compose and systemd skeletons without claiming production readiness.

**Exit:** Schemas, contracts, authority boundaries and examples are reviewed before repository implementation begins.

## Milestone 1 -- deterministic repository core

- [x] Implement safe path normalization and containment.
- [x] Reject traversal, symlinks, special files and reserved-file writes.
- [x] Parse frontmatter and Markdown with established libraries.
- [x] Implement strict concept validation and stable service-generated IDs.
- [x] Implement canonical serialization with golden fixtures.
- [x] Extract and resolve links without regular-expression Markdown rewriting.
- [x] Generate deterministic directory indexes and root mutation log.
- [x] Implement full repository audit.

**Required evidence:** serialization determinism; malformed frontmatter; duplicate IDs; containment attacks; rename/link rewriting; deterministic generated output.

## Milestone 2 -- Git transactions and control plane

- [x] Bootstrap bare authoritative repository and materialized checkout.
- [x] Add SQLite WAL migrations for operations, proposals, scheduler runs and service state.
- [x] Acquire one OS-level writer lease and expose read-only degradation.
- [x] Journal operations and enforce principal-scoped idempotency.
- [x] Apply mutations in temporary worktrees and stage exact paths.
- [x] Publish `main` with `git update-ref` compare-and-swap.
- [x] Materialize the committed revision and update operation results.
- [x] Reconcile interrupted operations and abandoned worktrees at startup.

**Required evidence:** concurrent stale-write conflict; replay behaviour; mismatched idempotency payload; injected crash recovery at every transaction boundary.

## Milestone 3 -- derived search and graph

- [x] Add rebuildable SQLite metadata and FTS5 schemas.
- [x] Implement weighted lexical search with bounded snippets and cursors.
- [x] Build links, backlinks, graph metrics, orphan and broken-link state.
- [x] Add full rebuild and exact changed-path incremental update.
- [x] Track repository and index revisions and strict freshness waits.
- [x] Add clean-scan parity checker and corruption quarantine/rebuild.
- [x] Apply authorization filters before ranking and output.

**Required evidence:** full/incremental parity; no hidden-result ranking leakage; index deletion and rebuild from Git alone.

## Milestone 4 -- read-only MCP service

- [x] Pin a released/tested uMCP version with Streamable HTTP support.
- [x] Integrate `AsyncMCPServer` and trusted request-local principals.
- [x] Implement standard success/error envelopes.
- [x] Implement `memory_help`, `memory_status`, `memory_search`, `memory_read`, `memory_list`, `memory_graph` and `memory_audit`.
- [x] Add read-only MCP resources and bounded health/readiness endpoints.
- [x] Enforce role and namespace policy on every result.
- [x] Run Piclaw adapter compatibility smoke tests.
- [x] Add compact progressive MCP tool disclosure, catalog resources and bounded declarative `memory_execute` plans.

**Exit:** One read-only canary serves Smith with revision-aware deterministic reads.

## Milestone 5 -- proposals and curated writes

- [x] Implement proposal create/get/list/review state transitions.
- [x] Generate deterministic validation and exact diff previews.
- [x] Enforce expiry, staleness, self-approval and namespace policies.
- [x] Implement reviewed apply through the canonical Git transaction pipeline.
- [x] Add curator-only create, patch and rename operations.
- [x] Update inbound links atomically on rename.
- [x] Emit resource update/list-change notifications after successful publication.
- [x] Link principal, proposal, operation and Git revision in durable audit data.

**Exit:** Smith can curate; Flint can read and propose; stale and conflicting writes fail safely.

## Milestone 6 -- production operations

- [x] Build a non-root, read-only-root-filesystem container with one writable data mount.
- [x] Add Portainer/Compose and hardened systemd deployment paths.
- [x] Add TLS/reverse-proxy guidance and distinct principal configuration.
- [x] Add structured logs, metrics, liveness and tiered readiness.
- [x] Add graceful write draining and bounded recovery startup.
- [x] Add backup, restore, retention, migration and rollback procedures.
- [ ] Produce SBOM, provenance, immutable image digest and release evidence.
- [ ] Run Docker/systemd parity and clean-host restore drills.

**Exit:** Local implementation and tests are complete. Live deployment evidence for multi-client production use, artifact publication and restore drills is still pending.

## Semantic and progressive retrieval

- [x] Port GTE-small inference to a portable Rust runtime with FP32 model compatibility.
- [x] Add shared scalar/SIMD vector validation and cosine ranking kernels.
- [x] Add a stable C ABI and Python `ctypes` integration with cancellation and bounded batches.
- [x] Add a loadable SQLite `vector_cosine` extension over packed float32 fields.
- [x] Add revision-aware concept embeddings, semantic search and deterministic hybrid rank fusion.
- [x] Filter authorization scopes before semantic ranking and preserve lexical degradation.
- [x] Add compact progressive MCP disclosure, catalog/workflow resources and bounded `memory_execute` plans.
- [x] Preserve standard/full MCP tool surfaces as configuration modes.
- [ ] Benchmark the production GTE-small model on deployed AMD64 and ARM64 hosts.

## Deferred roadmap

Start each tier behind an independent feature flag and only after Milestone 6.

1. [x] **Deep read-only answers:** bounded traversal, exact citations and retained traces behind an independent feature flag.
2. [x] **Exact answer cache:** keys include revision, authorization scope, policy, prompt and tool versions behind an independent feature flag.
3. [x] **Hot working memory:** fresh concept reads, scoped recent answers and safe `UNKNOWN` fall-through behind an independent feature flag.
4. [x] **Model-assisted proposals:** models may draft proposals but cannot approve or write.
5. [x] **Dream report-only scanner:** deterministic signals, revision watermarks and durable deduplication.
6. [x] **Dream proposal mode:** bounded model proposals, never direct model-authored writes.
7. [x] **Provider slots and fallback:** task-specific policies, explicit trust boundaries and transient model-level fallback only.

## Pilot acceptance checklist

- [x] Git is the only canonical knowledge source in implementation and tests.
- [x] Control state and idempotency survive simulated restart.
- [x] Derived state rebuilds without knowledge loss in local tests.
- [ ] Distinct deployed clients authenticate as distinct principals.
- [x] Namespace policy prevents search and metadata leakage in adversarial tests.
- [x] Concurrent stale writes conflict without lost updates.
- [x] Proposal review and apply are fully attributable in the control plane.
- [x] Piclaw's bundled MCP SDK connects through Streamable HTTP without legacy fallback in a local authenticated smoke test.
- [x] Python 3.10, 3.11 and 3.12 validation passes in clean containers.
- [ ] One primary deployment mode and the restore procedure are live-verified.
