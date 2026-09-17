# Memento 0.5.9 embedding worker recovery

Memento 0.5.9 is running on DiskStation, and its semantic worker has resumed embedding computation. It now retries transient SQLite contention during queue polling and execution, retains queued work, and reports whether the thread is alive. This fixes [#33](https://github.com/rcarmo/memento/issues/33).

## Failure and recovery

The 0.5.8 worker exited around 2026-09-17 07:18 UTC after `pending_embedding_paths` raised `sqlite3.OperationalError: database is locked`. Queue selection ran outside the exception handler. The graph endpoint continued to report pending work, `completed: 44` and no error even though the thread had stopped. The graph returned no semantic edges because its embedding revision lagged behind the repository, leaving the semantic force slider disabled.

A recovery baseline at 18:35 UTC contained 440 proposals, 462 proposal assets and 148 ready embeddings. The restart request stopped the container but timed out before startup; inspection confirmed it was stopped, and a start request recovered the same container without replacing its volumes or configuration. It began at 18:41:22 UTC and logged the MCP listener at 18:51:26 UTC. The worker resumed computation. By the pre-deployment snapshot at 19:30:11 UTC, 188 embeddings were ready.

## Release

* Pull request: <https://github.com/rcarmo/memento/pull/34>
* Release: <https://github.com/rcarmo/memento/releases/tag/v0.5.9>
* Commit: `e727bab601651f40fc3b6435682e6d407e5d9a6d`
* Published: 2026-09-17 19:19:29 UTC
* OCI index: `sha256:bebc0a3eaf935a5b4f07c3e060fd8e22a11dacff90cd55532ec04306c30e81bc`
* Exact-commit CI: <https://github.com/rcarmo/memento/actions/runs/35262878749>
* Release workflow: <https://github.com/rcarmo/memento/actions/runs/35263162979>

The local gate passed 444 tests, Ruff, mypy across 103 source files and about 86% branch coverage. The wheel built and imported from a clean environment. Chromium checks passed 17 tests with one skipped, including dead-worker reporting and semantic controls. The Python 3.12--3.14 matrix, container/runtime-model checks and release workflow passed. The downloaded OCI index hashed to the published digest, and the installed image labels identify the release commit. The runtime `model-assets-v1` release survived retention.

## Worker behaviour

Queue polling and pause checks now enter the worker's error handler. SQLite busy/locked errors preserve selected and full requests, report `database-busy` and wait up to one second before retrying; enqueue and shutdown interrupt that wait. Permanent work failures retain the existing consume-and-continue behaviour. Full requests use a generation counter so an earlier completion cannot discard a newer full request.

Status reads use in-memory state, with no SQLite call under the worker condition lock. Unexpected thread exit reports `alive: false` and an error; the graph coordinator reports unavailable and rejects new refresh requests. Runtime status includes liveness. The graph sidebar displays worker progress/errors and polls every 15 seconds while visible. Disabled semantic-force tooltips distinguish a hidden layer, a stopped worker, stale embeddings and a view with no semantic relationships.

Regression tests cover polling-lock recovery, selected/full request retention, status/enqueue during blocked polling, shutdown during retry, unexpected thread exit, pause-check errors, truthful graph availability, existing pacing gates and revision coalescing. No locks or worker failures were injected into production.

## Deployment and preserved state

Portainer endpoint 18, stack 111 received only the image-digest change. The update request timed out after three minutes, but inspection confirmed the new stack and running container; the update was not repeated. Startup briefly exceeded the health-check grace period while the normal index rebuild waited on storage.

* Container: `4c9dfd26a7af2575bdd8e89b4e24c5ca0637a2ef6a3ec501abd338ca16629742`
* Local amd64 image: `sha256:a18d2637181ce6d645ae84efa416d7343aeec708d2eedca3752c3e515e361e75`
* Started: 2026-09-17 19:33:19 UTC
* MCP listener: 19:41:16 UTC
* Healthy by: 19:41:33 UTC
* Restart count on the new container: zero

Before/after comparisons preserved all 440 proposal records and their checked author, base, creation/expiry, review, applied-operation and patch fields. All 462 proposal assets were byte-identical. Repository revision remained `7f2fb6d6b184dedd2e1a5d8d79f83a5e9ff25a71`; the text index matched it. Every ready embedding from the 188-row pre-deployment snapshot retained its vector hash. The snapshot after startup contained 189 ready rows; the previous worker could finish a job between snapshot and shutdown.

Mounts, stack/container environments, host resource/security settings and the original model volume were unchanged. Unauthenticated MCP returned HTTP 401 with a Bearer challenge. No proposal was submitted, reviewed, rebased or applied for verification, and no data volume or derived database was reset. [Machine-readable checks](embedding-worker-0.5.9.json) record the comparison without proposal bodies, credentials or asset bytes.

## Live progress and remaining backlog

Authenticated MCP reports version 0.5.9, 256 visible active concepts, 52 unresolved proposals and a current text index. The patched worker reported `alive: true`, `available: true`, and no error through startup, CPU sampling and computation. Its completed-job counter advanced from zero to one at 19:44:45 UTC and two at 19:46:00 UTC. MCP then reported 67 of 258 embeddings not ready, down from 69 at startup and 110 before recovery.

This verifies resumed computation, not completion of the semantic backlog. The existing 30-second pacing, CPU and interactive-idle gates remain unchanged. Semantic graph edges remain suppressed until the embedding and repository revisions match. Refresh the graph after the worker catches up; a pending flag alone is insufficient to infer progress or an ETA.

The existing Synology `PidsLimit: null` discrepancy and slow storage-bound startup are unchanged. The delegated review timed out, so no independent review approval is claimed. The fix is supported by manual inspection, regression tests, CI and observed live progress.
