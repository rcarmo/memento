# Memento delivery plan

**Status:** Proposed  
**Architecture:** [docs/implementation.md](docs/implementation.md)  
**Initial scope:** Deterministic shared-memory MCP service  
**Python:** 3.10–3.12

## Delivery rules

- Complete milestones in order unless a prerequisite can be isolated safely.
- Keep optional model tiers out of the runtime until the deterministic service is proven.
- Every mutation path must preserve authorization, expected revision, idempotency, audit and crash recovery.
- Each milestone ends with `make check`, `make typecheck`, package installation evidence and the milestone-specific tests.
- Mark behavior implemented only after tests pass; mark it deployed or live-verified only with deployment evidence.

## Milestone 0 — contracts and repository bootstrap

- [x] Add project instructions and local Python guidance.
- [x] Preserve the detailed implementation document.
- [x] Create this executable delivery plan.
- [ ] Define concept schema v1 and strict validation models.
- [ ] Define canonical Markdown fixtures and controlled vocabularies.
- [ ] Document MCP tools, envelopes, errors and pagination.
- [ ] Add threat model and trust-boundary diagram.
- [ ] Add sample knowledge bundle.
- [ ] Add MIT licence.
- [ ] Add Docker, Compose and systemd skeletons without claiming production readiness.

**Exit:** Schemas, contracts, authority boundaries and examples are reviewed before repository implementation begins.

## Milestone 1 — deterministic repository core

- [ ] Implement safe path normalization and containment.
- [ ] Reject traversal, symlinks, special files and reserved-file writes.
- [ ] Parse frontmatter and Markdown with established libraries.
- [ ] Implement strict concept validation and stable service-generated IDs.
- [ ] Implement canonical serialization with golden fixtures.
- [ ] Extract and resolve links without regular-expression Markdown rewriting.
- [ ] Generate deterministic directory indexes and root mutation log.
- [ ] Implement full repository audit.

**Required evidence:** serialization determinism; malformed frontmatter; duplicate IDs; containment attacks; rename/link rewriting; deterministic generated output.

## Milestone 2 — Git transactions and control plane

- [ ] Bootstrap bare authoritative repository and materialized checkout.
- [ ] Add SQLite WAL migrations for operations, proposals, scheduler runs and service state.
- [ ] Acquire one OS-level writer lease and expose read-only degradation.
- [ ] Journal operations and enforce principal-scoped idempotency.
- [ ] Apply mutations in temporary worktrees and stage exact paths.
- [ ] Publish `main` with `git update-ref` compare-and-swap.
- [ ] Materialize the committed revision and update operation results.
- [ ] Reconcile interrupted operations and abandoned worktrees at startup.

**Required evidence:** concurrent stale-write conflict; replay behavior; mismatched idempotency payload; injected crash recovery at every transaction boundary.

## Milestone 3 — derived search and graph

- [ ] Add rebuildable SQLite metadata and FTS5 schemas.
- [ ] Implement weighted lexical search with bounded snippets and cursors.
- [ ] Build links, backlinks, graph metrics, orphan and broken-link state.
- [ ] Add full rebuild and exact changed-path incremental update.
- [ ] Track repository and index revisions and strict freshness waits.
- [ ] Add clean-scan parity checker and corruption quarantine/rebuild.
- [ ] Apply authorization filters before ranking and output.

**Required evidence:** full/incremental parity; no hidden-result ranking leakage; index deletion and rebuild from Git alone.

## Milestone 4 — read-only MCP service

- [ ] Pin a released/tested uMCP version with Streamable HTTP support.
- [ ] Integrate `AsyncMCPServer` and trusted request-local principals.
- [ ] Implement standard success/error envelopes.
- [ ] Implement `memory_help`, `memory_status`, `memory_search`, `memory_read`, `memory_list`, `memory_graph` and `memory_audit`.
- [ ] Add read-only MCP resources and bounded health/readiness endpoints.
- [ ] Enforce role and namespace policy on every result.
- [ ] Run Piclaw adapter compatibility smoke tests.

**Exit:** One read-only canary serves Smith with revision-aware deterministic reads.

## Milestone 5 — proposals and curated writes

- [ ] Implement proposal create/get/list/review state transitions.
- [ ] Generate deterministic validation and exact diff previews.
- [ ] Enforce expiry, staleness, self-approval and namespace policies.
- [ ] Implement reviewed apply through the canonical Git transaction pipeline.
- [ ] Add curator-only create, patch and rename operations.
- [ ] Update inbound links atomically on rename.
- [ ] Emit resource update/list-change notifications after successful publication.
- [ ] Link principal, proposal, operation and Git revision in durable audit data.

**Exit:** Smith can curate; Flint can read and propose; stale and conflicting writes fail safely.

## Milestone 6 — production operations

- [ ] Build a non-root, read-only-root-filesystem container with one writable data mount.
- [ ] Add Portainer/Compose and hardened systemd deployment paths.
- [ ] Add TLS/reverse-proxy guidance and distinct principal configuration.
- [ ] Add structured logs, metrics, liveness and tiered readiness.
- [ ] Add graceful write draining and bounded recovery startup.
- [ ] Add backup, restore, retention, migration and rollback procedures.
- [ ] Produce SBOM, provenance, immutable image digest and release evidence.
- [ ] Run Docker/systemd parity and clean-host restore drills.

**Exit:** At least two Piclaw clients use distinct principals; backup/restore and crash drills pass; curated writes are monitored.

## Deferred roadmap

Start each tier behind an independent feature flag and only after Milestone 6.

1. **Deep read-only answers:** bounded traversal, exact citations and retained traces.
2. **Exact answer cache:** keys include revision, authorization scope, policy, prompt and tool versions.
3. **Hot working memory:** fresh concept reads, scoped recent answers and safe `UNKNOWN` fall-through.
4. **Model-assisted proposals:** models may draft proposals but cannot approve or write.
5. **Dream report-only scanner:** deterministic signals, revision watermarks and durable deduplication.
6. **Dream proposal mode:** bounded model proposals, never direct model-authored writes.
7. **Provider slots and fallback:** task-specific policies, explicit trust boundaries and transient model-level fallback only.

## Pilot acceptance checklist

- [ ] Git is the only canonical knowledge source.
- [ ] Control state and idempotency survive restart.
- [ ] Derived state rebuilds without knowledge loss.
- [ ] Distinct clients authenticate as distinct principals.
- [ ] Namespace policy prevents search and metadata leakage.
- [ ] Concurrent stale writes conflict without lost updates.
- [ ] Proposal review and apply are fully attributable.
- [ ] Piclaw connects through Streamable HTTP without legacy fallback.
- [ ] Python 3.10, 3.11 and 3.12 validation passes.
- [ ] One primary deployment mode and the restore procedure are live-verified.
