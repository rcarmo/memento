# Memento delivery plan

**Status:** Implemented in tree; local acceptance evidence recorded; deployment and publication evidence pending
**Architecture:** [docs/implementation.md](docs/implementation.md)  
**Initial scope:** Deterministic shared-memory MCP service  
**Python:** 3.12-3.14

This file is the delivery and acceptance ledger for what already exists, what has local proof and what still needs live operational evidence. Architecture, rationale and longer design notes stay in [docs/implementation.md](docs/implementation.md).

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

**Exit:** The read-only service is implemented and locally validated. Live canary evidence is still a deployment task, not a completed claim.

## Milestone 5 -- proposals and curated writes

- [x] Implement proposal create/get/list/review state transitions.
- [x] Generate deterministic validation and exact diff previews.
- [x] Enforce expiry, staleness, self-approval and namespace policies.
- [x] Implement reviewed apply through the canonical Git transaction pipeline.
- [x] Add curator-authorised create, patch and rename operations, with curator-surface exposure kept execute-only.
- [x] Update inbound links atomically on rename.
- [x] Emit resource update/list-change notifications after successful publication.
- [x] Link principal, proposal, operation and Git revision in durable audit data.

**Exit:** The implementation is present and locally tested. Distinct deployed principals and operator-run write evidence remain pending.

## Milestone 6 -- production operations

- [x] Build a non-root, read-only-root-filesystem container with one writable data mount.
- [x] Add Portainer/Compose and hardened systemd deployment paths.
- [x] Add TLS/reverse-proxy guidance and distinct principal configuration.
- [x] Add structured logs, metrics, liveness and tiered readiness.
- [x] Add graceful write draining and bounded recovery startup.
- [x] Add backup, restore, retention, migration and rollback procedures.
- [ ] Produce published SBOM, provenance and registry digest evidence. A local immutable image ID is recorded in the load/release evidence.
- [x] Run a local read-only-root Docker smoke and clean temporary-root backup/restore drill; live systemd parity remains pending.

**Exit:** Local implementation and local operational validation are complete. Live deployment evidence for multi-client production use, artifact publication and restore drills is still pending.

## Semantic and progressive retrieval

- [x] Port GTE-small inference to a portable Rust runtime with FP32 model compatibility.
- [x] Add shared scalar/SIMD vector validation and cosine ranking kernels.
- [x] Add a stable C ABI and Python `ctypes` integration with cancellation and bounded batches.
- [x] Add a loadable SQLite `vector_cosine` extension over packed float32 fields.
- [x] Add revision-aware concept embeddings, semantic search and deterministic hybrid rank fusion.
- [x] Filter authorization scopes before semantic ranking and preserve lexical degradation.
- [x] Add compact progressive MCP disclosure, catalog/workflow resources and bounded `memory_execute` plans.
- [x] Preserve standard/full MCP tool surfaces as configuration modes.
- [x] Add repository-owned local load harnesses for direct reads, write contention, idempotent replay, proposals, backup/restore and optional authenticated HTTP drills.
- [x] Benchmark the vendored GTE-small model on the local AMD64 host with recorded semantic-load evidence.
- [ ] Repeat the production model benchmark on a deployed ARM64 host.

## Local orchestration model study

- [x] Pin and run the public Needle checkpoint and tokenizer fully offline on the local AMD64 host.
- [x] Measure checkpoint load, warm latency, peak RSS, environment size, JSON validity, routing accuracy, determinism and UNKNOWN behaviour.
- [x] Record the go/no-go decision in ADR 0002; the full-plan checkpoint does not meet production thresholds.
- [x] Fine-tune Needle for two epochs on a free, deterministic 1,500-example Memento routing/plan/UNKNOWN corpus using the local RTX 3060.
- [x] Rerun unchanged and unseen-family AMD64 gates; the experimental full-plan checkpoint remains below routing, abstention and plan-validity thresholds and is not shipped.
- [x] Train a family/entity-separated shallow router and add deterministic plan expansion; the untouched AMD64 test reaches 100% routing/validity/UNKNOWN recall with 0% false actions.
- [x] Vendor the passing checkpoint and explicit train/validation/test corpora through Git LFS without enabling the JAX runtime.
- [ ] Validate the exact checkpoint on ARM64 and a pinned embedded/Cactus runtime before production integration.

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
- [x] Python 3.12, 3.13 and 3.14 validation passes in clean containers.
- [x] Repository-owned load reports cover local scenario counts, percentiles, invariants and bounded CI-friendly checks.
- [ ] One primary deployment mode and the restore procedure are live-verified.
