# Memento 0.5.8 deployment and proposal continuity

Memento 0.5.8 is running on DiskStation. The five proposals reported in [#28](https://github.com/rcarmo/memento/issues/28) are visible as `needs_rebase`, with clean per-change checks and matching skill/body digests. All retain their original IDs, authors, base revisions, content and assets. None was rebased, reviewed or applied during deployment verification.

## Release and checks

* Release: <https://github.com/rcarmo/memento/releases/tag/v0.5.8>
* Release commit: `7821612eb6dd6c7e57a208c82dbffa8a83bc554e`
* Published: 2026-09-16 23:52:34 UTC
* OCI index: `sha256:2f26efad9c4411f9bf03144814aa93c7f1ad88b8f88464cd78a4d9a9e078f8c9`
* Exact-commit CI: <https://github.com/rcarmo/memento/actions/runs/35163489724>
* Release workflow: <https://github.com/rcarmo/memento/actions/runs/35163678591>

The final local gate passed 436 tests, Ruff, mypy across 103 source files, and about 86% branch coverage. The wheel built and imported from a clean environment. Python 3.12--3.14 CI and container/runtime-model checks passed. The downloaded OCI index independently hashed to the published digest; the installed image labels identify the release commit. The `model-assets-v1` release survived retention.

The continuity implementation is in [PR #29](https://github.com/rcarmo/memento/pull/29). Main CI caught a timing-sensitive test: its simulated deadline could expire before the worker reached the journal checkpoint. [PR #30](https://github.com/rcarmo/memento/pull/30) synchronised that test, followed by ten successful targeted runs and passing exact-commit CI before 0.5.7 publication.

Live 0.5.7 verification found a historical proposal with the literal string `null` as its base revision. Queue refresh tried to diff that value and made status fail. [PR #31](https://github.com/rcarmo/memento/pull/31), released as 0.5.8, keeps such records visible as conflicts with `base_revision_unavailable` guidance and refuses to rebase unverifiable changes. The published 0.5.7 tag was not changed.

## DiskStation

Portainer endpoint 18, stack 111 received digest-only image changes. Before/after comparisons against the pre-0.5.7 baseline confirmed identical stack and container environments, mounts, host resource/security settings and the original explicitly mapped `/models` volume. No data volumes were replaced or removed.

* Container: `1768dad45eb51ead618b62137cd5204ecd42478b1e0288dd167878647a6f7fdc`
* Local amd64 image: `sha256:ea76102eae55ab9fd841ace22dae69f6920aeefb7d79396c9be52d644dd7055b`
* Started: 2026-09-16 23:57:55 UTC
* MCP listener logged ready: 2026-09-17 00:02:08 UTC
* Docker health confirmed: 2026-09-17 06:36:29 UTC
* Restart count: zero

The client was interrupted during startup polling. The next inspection confirmed the correct running image, so the deployment was not repeated. Unauthenticated MCP returned HTTP 401 with `WWW-Authenticate: Bearer`.

## Preserved state

The baseline at 2026-09-16 23:18:48 UTC contained 421 proposals, 446 proposal assets and 180 ready embedding rows. Production had changed since the issue was reported; its repository and index revision was `0ed441a481d2ec0aae74c97406ad36ab4f2e5560`. MCP then reported 255 active indexed concepts and 77 embeddings not ready.

The final comparison at 2026-09-17 06:41:07 UTC found:

| Check | Result |
| --- | --- |
| Repository revision | Unchanged |
| Proposal records | All 421 retained |
| Author, original base, creation/expiry, review and applied-operation fields | Unchanged for every proposal |
| Stored proposal patch hashes and complete patch JSON hashes | Unchanged for every proposal |
| Proposal asset bytes, manifest hashes, IDs and creation times | All 446 identical |
| Previously ready embedding hashes | All 180 preserved |
| Control schema | Migrated from 9 to 10 |
| Derived index and embedding revision | Both match the repository |
| Final embedding readiness | All 258 stored embeddings ready |

The normal startup index refresh reports 256 active concepts, plus two in Trash. No Git content was created, altered, archived, restored or purged by verification. The index-count increase therefore reflects refreshed derived state, not a deployment-created concept. No manual index or embedding rebuild was invoked.

The final proposal states are 299 applied, 43 rejected, 22 expired, 42 conflicted and 15 needs-rebase. Compared with the baseline, 58 old stale records became 57 visible unresolved records and one expired record. No stale rows remained. Existing applied/rejected decisions were retained.

The machine-readable [deployment comparison](proposal-continuity-0.5.8.json) contains counts, preservation checks and the five reported IDs, without proposal bodies, review comments, credentials or asset bytes.

## Live proposal checks

Authenticated MCP reports `service_version: "0.5.8"`, `proposal_backlog: 57`, a current index, ready semantic search and a loaded Needle router, with no warnings. `proposal_list(status="unresolved", limit=20)` returns a bounded page containing both conflicted and needs-rebase proposals, with a continuation cursor.

Each of the five reported proposals returned `needs_rebase`, two clean changes and `concept_body_matches_asset: true`:

* `99a9a029-5951-4a9f-9562-cd405a9c65ed`
* `986a753e-ce1c-43c4-8a71-96ece2f2c352`
* `233dead8-f9ac-40fa-a0ad-ef510e5de58e`
* `b536d533-1c50-47a4-ab0a-7731d010fda9`
* `83620a10-732c-4379-8361-1cb264411b9b`

Detailed inspection of the first record returned a `system` / `repository_advanced` history event from `stale` to `needs_rebase`, with the original/current revisions and clean conflict results. Its review fields and applied-operation fields remain null. The historical invalid-base proposal is separately visible as `conflicted`, retains `base_revision: "null"`, and returns guidance to inspect current content and file a fresh proposal.

These calls exercised status, queue listing, proposal inspection, conflict classification, asset parity metadata and audit visibility. Live calls did not change a proposal's base, author content or review decision. Status refresh wrote classification/expiry events through the service's normal path.

## Mutation and failure-path coverage

Disposable-repository tests cover original-proposer rebase without curator privileges, stable proposal identity and creation/expiry, unchanged ZIP bytes and skill parity, review after rebase, rejection/requested changes after revision advancement, deleted-target conflicts, forbidden cross-principal or cross-namespace operations, keyed review replay, control-transaction rollback, and schema migration.

Concurrency tests exercise two applies at one revision, duplicate concurrent rebases, and rebase/reject races. Worker tests simulate a lost response, verify `in_progress` while the writer holds the lock, and reconcile the committed outcome through the original principal/key. Execute tests retain a successful control-state operation when a later reference fails. The published-image CI runs the runtime/model smoke checks; these mutation scenarios were not replayed against production proposals.

## Operational limits

The first status request after resumption timed out while legacy records were being classified. Read-only diagnostics showed filesystem wait states during the refresh. It completed without a restart; subsequent status, queue and proposal reads succeeded. Large first-time refreshes on this storage can exceed a client deadline. No latency guarantee is inferred from Docker's TCP health check.

The existing Synology `PidsLimit: null` discrepancy is unchanged. A bounded independent review found no remaining issue in the serialised rebase/review code slice; wider delegate attempts timed out. This report does not treat that focused review as an audit of the entire service.
