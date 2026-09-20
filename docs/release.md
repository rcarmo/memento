# Release

This page starts with the current pure-Go release procedure. Version-specific sections below preserve the checks and compatibility notes for earlier Python/Rust releases; they are history, not instructions for building `v1.0.0`.

`v1.0.0` changes the implementation behind `ghcr.io/rcarmo/memento` from Python/Rust to pure Go while preserving the client and persisted-state contracts. It is a major release because implementation-specific commands and environment variables are removed, not because MCP clients, mounted configuration or state require migration.

## Local release checklist

* `python3 tools/prepare_runtime_models.py`
* `make audit`
* `make performance`
* `make model-test`
* `make corpus-test`
* `make release-check MEMENTO_VERSION=1.0.0 SOURCE_DATE_EPOCH=<epoch>`
* `make go-container-contract MEMENTO_VERSION=1.0.0`
* inspect the generated amd64/arm64 archives, image metadata and SHA-256 manifests
* use a disposable copy of production state for final target-host acceptance; do not modify the live volume during release qualification

## Packaging notes

The OCI image is built with pinned Go and distroless Debian 12 base manifests. Its runtime contains five static Go executables--the daemon, GTE worker, Needle worker, Needle sidecar converter and skill importer--plus the `memento-embed` compatibility alias and release-prepared model assets. It contains no shell, Python, Rust libraries, CGo dependencies or Git executable. The converter runs while the image is built; normal routing consumes the generated sidecar through the short-lived Needle worker. The image runs as UID/GID 65532 with the existing read-only-root and `/var/lib/memento` volume contract. `memento-go` is PID 1 and handles SIGINT/SIGTERM directly; its native `healthcheck` subcommand replaces the Python socket probe.

The image and static archives target amd64-v1 and arm64. Automatic inference dispatch is AVX2 -> SSE2 -> NEON -> scalar, and the amd64 image is checked under a no-AVX Westmere CPU model. See [ADR 0008](decisions/0008-build-for-baseline-cpus.md).

The NAS release is CPU-only by explicit operator decision. Retained [Mesa 22 Vulkan evidence](evidence/vulkan-nas-bookworm-2026-09-19.md) is historical reference, not a packaging dependency or an enabled backend. Vulkan on unrelated hardware remains a separately authorised future question.

* Existing `schema_version: 2` configuration is accepted directly, including obsolete FFI path fields that are syntactically validated but unused by Go.
* Existing `repo.git`, `current`, `control.sqlite`, `derived.sqlite`, proposal, access and asset state is opened without conversion. The contract test proves the previous `0.5.9` image can reopen the same disposable volume after Go, preserving rollback.
* `MEMENTO_ADMIN_MASTER_KEY`, configured principal token variables, optional provider-key variables, `MEMENTO_GTE_MODEL`, Needle model overrides and `MEMENTO_SIMD` retain meaningful behaviour. Python/Rust tuning variables are not carried forward.
* The DiskStation secret file remains mounted at `/run/secrets/memento.env`; the native `--env-file` option reads it without requiring a shell.
* Asset submissions that fit the configured 72 MiB MCP request limit use `zip_base64`; larger packs use the principal-bound raw-upload staging path.

## CI and publication

The `main` branch CI runs the complete model-independent Go audit plus real-model/allocation gates on native amd64 and ARM64 runners, the 360-case Needle corpus on ARM64, and the replacement-container contract on amd64. The final pre-release result is recorded in [`docs/evidence/go-v1-validation-2026-09-19.md`](evidence/go-v1-validation-2026-09-19.md). The release workflow derives the cache key from `models/runtime-models.json`, restores or downloads the pinned `model-assets-v1` bundle, verifies the archive and each file digest, uploads the three source files once, and feeds that artifact to both native image builders. Each image build creates and verifies its architecture-independent FP32 Needle sidecar from the pinned NDL file.

A stable `v1.0.0` tag publishes native `linux/amd64` and `linux/arm64` manifests to the existing GHCR repository, assembles the multi-architecture index, attests its provenance, generates an SPDX JSON SBOM, attaches that SBOM to the GitHub release and tags the index as `1.0.0`, `1.0`, `1` and `latest`. Release cleanup retains five application releases while protecting fresh digest-first child manifests.

## Runtime model asset policy

Workflow checkouts require only ordinary Git. A single model-preparation job derives a cache key from `models/runtime-models.json`, restores the three runtime image artefacts (GTE model, Needle NDL and tokenizer) from the Actions cache, or downloads the pinned `model-assets-v1` GitHub Release bundle on a cache miss. The archive and every extracted file are SHA-256 checked against the committed manifest before being uploaded once as a one-day uncompressed Actions artifact. Native image builders consume that artifact. Training JSONL, vocabulary and Python checkpoint files are never downloaded by CI or release jobs.

Real GTE and Needle model coverage runs on native amd64 and ARM64 CI hosts using the exact assets later copied into the image; the amd64 replacement-container contract then exercises semantic subprocess readiness and Needle routing inside the built image. Updating a runtime model requires publishing the matching pointer-keyed release bundle before merging the manifest change.

## Embedding worker recovery for 0.5.9

Issue #33 fixes the semantic worker exiting on a SQLite lock during queue polling. Polling and pause checks now share the worker's error boundary. Transient busy/locked failures retain queued work and use an interruptible retry wait; status reads no longer access SQLite under the worker condition. Worker liveness is exposed in runtime/graph diagnostics, and stopped workers reject enqueue attempts. The graph sidebar shows worker state and explains missing semantic forces.

Tests cover polling recovery, full/selected work retention, nonblocking status/enqueue while polling, close during retry, unexpected worker exit and truthful graph availability. Existing pacing, failure, revision-coalescing and shutdown tests remain required.

## Historical proposal bases for 0.5.8

Live 0.5.7 verification found a retained proposal whose base revision was the literal string `null`. Conflict refresh tried to diff it and returned `repo_unavailable` for status and queue reads. Version 0.5.8 treats an invalid or unavailable historical base as a conflict on that proposal, with `base_revision_unavailable` guidance. Its content/assets remain unchanged, it stays reviewable, and a curator can reject it or request fresh submission. Rebase fails closed. Tests cover `null`, an empty base and a missing commit hash.

## Proposal continuity for 0.5.7

Issue #28 keeps unresolved proposals visible after repository advancement. Clean older proposals become `needs_rebase`; overlapping paths become `conflicted`. Both remain in the unresolved queue/backlog and permit rejection or requested changes. Original proposers or scoped curators can use execute-only `proposal_rebase` to update a clean base in place without uploading assets again. Approval must be repeated.

Control schema 10 adds append-only `proposal_events` without rewriting existing proposal or asset records. Rebase and keyed review commit state, audit and operation results atomically; `operation_get` reconciles owned rebase operations for proposers. A repository-scoped reentrant lock serialises review, rebase and apply with Git publication. Legacy stale rows are classified during normal refresh. The timeout handler returns `indeterminate` while the worker completes.

Regression checks include disjoint concurrent applies, rebase/reject races, asset preservation, unchanged author/creation/expiry fields, deleted target conflicts, original-key replay, transaction rollback, worker timeout reconciliation, legacy schema migration and execute failures after a control commit. Deployment must preserve existing proposals and use read-only inspection for their live checks.

## Typed execute references for 0.5.6

Issue #23 separates plan placeholders from concrete operation validation. Saved references now work in numeric, boolean and nested argument fields; resolved values must satisfy the original operation schema before any service call. Direct tools retain their existing schemas. Invalid plans report bounded operation/field errors rather than every branch of the operation union. Post-commit resolution/validation failures retain committed revisions and reconciliation IDs.

Regression tests cover chained file offsets and version/digest references, negative and oversized values, wrong numeric types, unknown/malformed/out-of-range references, null EOF offsets, nested values, unchanged authorisation and failures after a successful mutation.

## Accepted-asset retrieval checks for 0.5.5

Accepted assets now support manifest-only, bounded file and archive-range reads. Resumed requests pin the version and ZIP digest. Small default archives retain their full ZIP/manifest response; larger clients must follow `next_offset` and verify the assembled digest. `memory_status.limits.assets` advertises retrieval limits. See [accepted-assets.md](accepted-assets.md) for compatibility, integrity and response-budget details.

Validation covers manifest inspection without ZIP reads, binary and UTF-8 chunks, EOF and final ranges, version changes, digest mismatch, namespace denial, filesystem/ZIP symlinks, undeclared files, Trash/purge and pruned versions. Proposal file reads reuse the same safety helpers. `asset_get` is registered through execute and manifests are not silently sliced by its generic record limit.

## Trash visibility check for 0.5.4

Live verification with archived fixtures exposed a missing browser request header: Show Trash changed local state without requesting trashed nodes. The browser now sends `X-Memento-Include-Trash: true`, and a regression checks that toggling the view both adds and removes the returned Trash nodes. This patch includes the reviewed archival and graph-force changes below.

## Graph force and opacity checks for 0.5.3

Semantic opacity defaults to 60%, independently adjustable without reheating layout or changing explicit links. Shared namespace, type and provenance now have bounded relationship edges alongside shared tags; their controls show available counts and disable when inactive. Degree normalisation is per force kind so unrelated edges do not dilute slider effects. Tests verify alpha on both dashed and selected semantic links and actual layout motion with each shared force enabled and disabled. This release includes the reviewed archival workflow from 0.5.2.

## Reviewed archival checks for 0.5.2

Issue #20 adds `kind: trash` proposal changes and a bounded, policy-scoped `archival_impact` report. Curators inspect affected inbound references and retained accepted assets before review/apply. Archival proposals require current revisions and fresh indexes; stale reviews, mixed changes and duplicate targets fail. Apply uses the existing transaction and reconciliation pipeline. Tests cover assets, permissions, backlink visibility, stale revisions, approval and replay. Git history remains unchanged by archival.

## Graph and Trash checks for 0.5.1

External URLs no longer count as broken repository links; existing derived indexes reclassify them on migration. The graph displays cluster names, deduplicates neighbour references, obeys the semantic toggle for selected nodes too, and runs a cancellable force simulation until movement settles. Force controls expose strength, repulsion and preferred distance. Browser tests cover those interactions and the existing navigation, picking and touch paths.

Curators can move concepts to Trash, restore them, or explicitly purge their current Markdown and accepted assets while retaining Git history. Original namespace permissions apply inside Trash, restore rejects collisions, and every mutation requires a fresh revision and durable idempotency key. Regression tests cover the round trip, denied access, replay, current-asset deletion, retained historical content and index updates. See [trash.md](trash.md) for the contract. The unauthenticated graph debugger only displays Trash; mutations remain on authenticated MCP tools.

## Safety and recovery checks for 0.5.0

The 0.5 release rejects noncanonical paths before authorisation, blocks dangling symlinks and enforces the serialised UTF-8 concept limit and Markdown filenames. Rename preserves Markdown source and excludes generated indexes. Recovery no longer treats an unchanged worktree as a published mutation, and execute errors after a commit retain its operation information.

Proposal pages use effective lifecycle status and bounded row batches. Cursors are encrypted, scoped to the principal, grants, filter and repository revision, and expire when the server restarts. A page can contain fewer visible results than its limit; follow its cursor until it is absent.

Memory tool work uses worker-owned SQLite connections and bounded admission. Calls have a 30-second response deadline; timed-out writes can continue, so reconcile the original idempotency key rather than retrying. Shutdown drains active workers before releasing runtime dependencies. Git subprocesses and model response sizes are bounded. The SQLite vector extension rejects non-BLOB arguments.

Restore requires the writer lease and mandatory checksums, retaining the lock inode during state replacement. The deployment helper preserves existing Compose and environment settings, changes only the Memento image reference, and verifies the pulled release digest. Configuration migration is not part of routine deployment.

## Proposal curation and graph audit checks for 0.4.0

The 0.4.0 curation path keeps proposal appraisal inside MCP without returning full bodies by default. Release validation covers 200-item summary bounds, cursor pagination, generic concept-body digest matching across attached asset entries, manifest-only metadata, bounded file chunks, stale per-path conflict checks, body/asset-complete subset revision, and current-policy filtering for proposals, staged assets and operation reconciliation. The 0.4.1 production-acceptance patch pushes an explicit proposal status filter into the control query so routine submitted/stale appraisal does not deserialize the historical proposal archive before applying the bound. The 0.4.2 discovery patch includes staged-file inspection, subset revision and operation reconciliation in the compact curation workflow, and includes staged-file inspection and reconciliation in the asset-pack review sequence.

Commit reconciliation must distinguish committed, in-progress, conflict, failed-before-mutation and indeterminate outcomes by the caller's operation ID or idempotency key. Direct mutations return proposal-first guidance, and body patches to concepts with published assets are rejected before the transaction begins. The checks use ordinary project, instance and template concepts; none of these contracts depends on a `/skills/` namespace or skill asset kind.

`memory_audit` requires the literal `proposer` role. Its graph diagnostics are filtered to namespaces the caller can both read and write, exclude protected paths without an explicit grant, reject stale or mismatched cursors, and never run a repair action. The release also loads the SQLite vector extension through SQLite's supplied extension API table rather than linking a second SQLite implementation into the process.

## Answer-evidence checks for 0.3.23

The 0.3.23 answer path adds deterministic query profiling, secret-first abstention, authorisation-scoped evidence, hybrid top-5/top-10 escalation and bounded relational support chains to `memory_answer`. It also fixes natural-language lexical normalization and typed `memory_graph` edge serialization.

Release validation must cover lexical misses, stop-word-only input, repeated terms and explicit `query_syntax="fts5"`; namespace isolation across retrieval and graph traversal; secret abstention before cache or model access; stale/current conflicts and supersession; prompt-injection excerpts marked as untrusted; exact-revision citations; and complete graph anchor chains.

The disposable 23-concept corpus passed all five policy gates with GTE-small ready. A post-release rerun at the exact `0.3.23` commit reached 333052 KiB peak RSS against the saved 332920 KiB `0.3.22` baseline; query-phase growth was 6408 KiB against 5928 KiB. Plain lexical top-5 mean recall rose from 0.0000 to 0.8333 while hybrid top-5 recall remained 0.8974. The complete release, production and corpus record is [`docs/evidence/release-0.3.23.md`](evidence/release-0.3.23.md).

The production compact surface has answers disabled and no configured provider slots, so live acceptance verifies the absent `memory_answer` tool rather than claiming a model or secret-abstention result. Those paths remain release-test requirements whenever the feature is enabled.

## Inventory and comparison checks for 0.3.24

The 0.3.24 read path adds direct bounded `memory_inventory`, execute-only `compare_manifest` and optional protected read namespaces. It also preserves the original operation ID across failed idempotent retries, serialises status-only `deprecated` and `tombstone` patches correctly, and returns repository audit issues from slotted dataclasses.

Release validation must cover deterministic inventory pagination and field projection; digest and byte parity with `memory_read`; asset summaries; namespace pruning before parsing; 50-row manifest and namespace ceilings; explicit matching, differing, local-only and Memento-only classifications; timezone-aware likely-newer decisions; and rejection of paths that escape the authorised prefix. `compare_manifest` must remain absent from the direct tool surface and available through `memory_execute`.

Protected-prefix checks must show that a broad `/` reader loses protected paths, an explicit equal or nested grant restores them and `admin` bypasses the mask without inheriting ordinary content roles. Status-only patch replay and failed-operation retry tests must verify idempotent operation identity. The complete release and live deployment record is [`docs/evidence/release-0.3.24.md`](evidence/release-0.3.24.md).

## MCP access contract checks for 0.3.27

The managed-access schema now gives clients the exact principal-name, role and namespace-prefix constraints. `access_principal_create` also repeats its five required fields in the tool description because some Codex catalog paths reduce a structured MCP schema to an unknown argument type. An empty call returns the complete missing-field list and the `/path/` prefix syntax in one response; it cannot recover values that the client omitted.

Release validation sends both empty and complete create requests through raw JSON-RPC dispatch against a disposable control database. Live checks use deliberately invalid principal names or `memory://` prefixes so production validation reaches the expected rule without creating a credential or principal.

## Persistent transport checks for 0.3.26

The 0.3.26 transport path pins uMCP `v0.2.2` at `9c89a708d14ae804e32aa65de10af7c02922617d`. Wheel validation must install the `mcp` extra in a clean environment and confirm that Memento, uMCP and `aioumcp` resolve from that environment.

Memento's integration tests cover its advertised server identity and capability object, multiple POST requests on one persistent HTTP/1.1 connection and MCP session, session resumption on a replacement POST connection, subscribed resource notifications over a reconnectable GET/SSE stream and authenticated session deletion. Live acceptance additionally checks a longer repeated-request sequence and abrupt SSE replacement. An abrupt closure is detected when a notification or periodic keepalive write fails, so replacement GETs may return `409` in the interim; reconnecting does not replay missed notifications. The complete release and live deployment record is [`docs/evidence/release-0.3.26.md`](evidence/release-0.3.26.md).

## Progressive embedding release checks

Release validation covers pointer-only/rebuild reuse, restart-derived pending work, model-revision invalidation, manual priority, startup and interactive idle gates, `/proc/stat` CPU sampling with I/O wait treated as idle, pacing, `nice` command construction, and one-thread native environments. The operator-run DiskStation deployment preserves `/var/lib/memento`, then checks status and graph revision fields including `pause_reason`, `current_path` and `completed`.

## Provenance and software bill of materials

Base-image manifests and GitHub Actions are pinned. The release workflow attaches BuildKit provenance to the OCI index, generates an SPDX JSON SBOM from the published multi-architecture image and includes that SBOM in the GitHub release.

## Access-management release checks

Release validation must cover the v7 control migration, bootstrap rename to `sandbox`, admin-only tool discovery, `/admin` authentication, one-time credential behaviour and the explicit offline master-key rotation command. Runtime deployment requires `MEMENTO_ADMIN_MASTER_KEY`; per-principal environment tokens are bootstrap/recovery inputs.
