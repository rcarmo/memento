# Memento 0.5.0 validation and deployment

## Release

* Release: https://github.com/rcarmo/memento/releases/tag/v0.5.0
* Commit: `0f2c57f9faaff2b46c6f55d34142aa3220a8bf6c`
* Published: 2026-09-05 22:19:16 UTC
* OCI index: `sha256:4b542c77fe666f8cee7c998964573398774fe0465cac68dddd1e65aace612ea6`
* CI: https://github.com/rcarmo/memento/actions/runs/33995154255
* Release workflow: https://github.com/rcarmo/memento/actions/runs/33995270757

Both workflows passed, including Python 3.12--3.14, native amd64/arm64 images and baseline CPU smoke checks. Downloading the registry index and calculating SHA-256 independently reproduced the published digest. The index includes both architectures and their attestations.

Local `make check` and coverage passed all 346 Python tests, with approximately 85% coverage. Rust checks and browser build checks passed. The 0.5.0 wheel built and imported from a clean installed virtual environment.

## Audit fixes

Regression tests cover canonical-path authorisation, dangling symlinks, UTF-8 concept limits, Markdown path suffixes, unpublished crash recovery, post-commit projection failures, Markdown-preserving rename, effective stale status and bounded proposal queries, offline restore and required checksums, stack configuration preservation, model response bounds, and SQLite argument types.

Memory calls use worker-owned control connections rather than disabling SQLite thread checks. Tests exercise event-loop responsiveness, bounded admission, real worker commits, concurrent reconciliation and shutdown draining. The response deadline does not cancel a transaction that may already have published: clients must reconcile the original idempotency key. Git commands have 30-second timeouts. Workers drain before runtime cleanup.

Proposal cursors are encrypted and scoped to the caller's policy, filter and repository revision. They expire on server restart. Pages can be shorter than the requested limit when scanned records are inaccessible; callers must follow the cursor until absent.

## DiskStation

Endpoint 18, stack 111 received a single image-reference change. Existing environment entries, configuration, mounts and security settings were retained; no configuration helper or manual embedding rebuild was run.

* Container: `439679854b18666f342597539bf71a2be2ec0e024b6567dee3cfcf73d8be5b94`
* Local image: `sha256:7ee3a88dd5edd26a35c562b402644a8219bb4a37d388bdd47add7aa56adb615b`
* Started: 2026-09-05 22:24:17 UTC
* Status: running, healthy, zero restarts
* OCI labels: `v0.5.0` and the release commit above
* UID/GID: `65532:65532`; read-only root; all capabilities dropped; `no-new-privileges:true`

Before and after replacement, repository and index revision were `de807e21aa5f00da6c762828b2c1784ee9fa4c28`, with 178 visible concepts and no stale content index. MCP rediscovery returned 23 configured tools. Live worker-backed proposal listing and operation reconciliation succeeded. A two-page stale listing advanced to a different proposal through its encrypted cursor. Reading `/./projects/memento.md` returned `forbidden` before filesystem access.

These live checks were non-committing. Crash injection, rename mutations, invalid writes and restore tests ran only against disposable local fixtures, not production.

## Limits and follow-up

The pre-deployment 0.4.2 service already reported 19 of 178 embeddings not ready. The same count remained after deployment; semantic search is degraded, not fully ready. Its embedding revision marker changed from an older revision to `partial` on startup, while repository and content-index revisions remained unchanged. Needle is loaded and the SQLite vector extension is enabled.

Docker still reports `PidsLimit: null`, an existing Synology/Compose discrepancy. This release does not claim to fix that host-level enforcement issue. Clean-host restore, real ARM64 performance, native fuzzing and TLS/network isolation of the optional unauthenticated graph debugger remain operational follow-ups.

There was no independent delegated review because the configured delegation policy did not permit one. Passing tests and these live checks do not constitute exhaustive proof against all crash schedules or hostile native inputs.
