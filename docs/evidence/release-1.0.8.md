# Memento 1.0.8 deployment and migration progress

Memento 1.0.8 is deployed and healthy on the DiskStation. Corrected legacy-embedding metrics, item-relative graph diagnostics and persisted similarity reads passed canary and production checks. Legacy conversion is still running; final migrated-state validation has not completed.

## Release and gates

- Source: `b122578c7ffa68473316bcee58819d80ad76cab0`.
- [Exact-commit CI 35761473681](https://github.com/rcarmo/memento/actions/runs/35761473681): passed, including amd64/ARM64 model gates, model-free audit, repeated Chromium/WebKit checks and container contract.
- [Release workflow 35763689568](https://github.com/rcarmo/memento/actions/runs/35763689568): passed, publishing both architectures, baseline-CPU validation, provenance and SPDX SBOM.
- [Release v1.0.8](https://github.com/rcarmo/memento/releases/tag/v1.0.8).
- OCI index: `sha256:f2774641a8373e7bd314fd2a6af1faf2d3695ffd545792320fcbae6e0ce3fb3b`.
- Installed amd64 image config: `sha256:c1de971d7335acc722fd10bfcaeebb9e5bba009393797b0164ff7269a8277532`.
- Production container: `f17f5daad31b6ebad47fafd9221b0da5d12222280c3a972b2ff528631473d05a`.

The runtime change from v1.0.7 is the missing `legacy` metrics status mapping, with exhaustive status-bucket and SQLite-to-HTTP regression tests. Test-only follow-ups separate [cold/steady-state GTE allocations](gte-benchmark-repeatability-2026-09-22.md), keep model-dependent benchmarks optional in the model-free audit, and give the injected-failure worker test a bounded storage-independent wait. Performance ceilings were not relaxed.

## Canary and cutover

The v1.0.8 canary reopened the qualified v1.0.7 state copy, preserving six ready chunked items and 15 cached pairs. Its exact metrics were ready=6, legacy=253, missing=1, other=0. The same state reported legacy=0 and other=253 on v1.0.7; the correction was verified on the immutable image.

The [v1.0.7 evidence](release-1.0.7.md) records the fresh backup, item-relative mutation tests, authenticated lifecycle, restart and SQLite cache proof. Those semantic implementations are unchanged in v1.0.8. The verified backup directory `/volume1/docker/memento/config/backups/20260922T145918Z`, prior stack payload, and v1.0.6 rollback index `sha256:0837a0559704d454f882c7df08b0d6311f683435ef61005014e1f94ab8b36699` remain available.

Production's repository revision was unchanged from the completed v1.0.7 validation when the corrective cutover began: `22b7497924acee09e217b63ef8c5a06d4e5a2964`. Only the image digest changed. A Portainer timeout occurred after creating the v1.0.8 container. Inspection confirmed the new digest in both stack and container; start/readiness were reconciled without repeating the stack mutation.

The running container is healthy, OOMKilled is false and automatic restart count is zero. Its user, ports, mounts, read-only root, capability drops, no-new-privileges and resource limits match the previous stack. Bind-array order changed but mount sources, destinations and access modes did not. The NAS kernel still does not enforce the requested PID limit.

## Production acceptance

Before queuing conversion:

- 258 visible concepts; repository and index revisions matched.
- Seven ready chunked items rendered 21 cached semantic connections, with no false repository-relative `embedding_stale` diagnostics.
- Three overview reads passed; times were 4403, 491 and 1393 ms on the storage-loaded NAS.
- Chromium selection, link navigation, scoped diagnostics, descending asset versions and sidebar alignment passed without page errors.
- Metrics collection succeeded and correctly reported ready=7, legacy=252, missing=1, other=0.
- All four stopped v1.0.7/v1.0.8 rollout helpers were removed without deleting volumes or state-copy/backup directories.

The 16 broken explicit-link occurrences are [pre-existing imported references](broken-link-regressions-2026-09-22.md); no content was rewritten.

## Conversion checkpoint — 22 September, 18:48 UTC

A single selected-scope request queued 253 incompatible items, including two trash items. The seven already-ready items were deliberately excluded. The original IDs, paths and embedding provenance are retained in `production/migration-targets.json` for comparison at completion.

Configuration is unchanged: `refresh_on_startup: false`, progressive generation enabled, 30-second pacing between batches, startup grace, interactive-idle checks and CPU admission limits. The selected queue is in memory; an unexpected restart must be followed by reconciliation of remaining incompatible IDs rather than replaying the original set.

At the checkpoint, metrics reported ready=12, legacy=247, missing=1, pending=0, stale=0, error=0 and other=0. Five queued items had completed; the worker was alive and pacing through `/skills/piclaw/addon-work-map.md`. Zero pending embedding rows does not mean an empty worker queue. The worker's `queued_paths` field records the submitted scope; completion must be judged from live row counts and worker state.

One earlier scrape returned collection-success=0 during storage load; subsequent collections succeeded. Check `memento_metrics_collect_success`, not HTTP status alone.

A muted 30-minute monitor (`task-c21a3bbd-1499-4808-b666-25b87678f820`) observes worker status and metrics without re-enqueuing work or scanning the full graph. Its first scheduled check is 19:07 UTC. Errors or sustained lack of progress require investigation. Once incompatible/error counts reach zero and the worker is idle, final qualification must verify chunk/cache integrity, unchanged provenance for the original seven items, graph results and restart persistence; then update this record and close the plan.

Evidence and monitor state: `/workspace/tmp/memento-rollout-108/`, including `migration-observations.jsonl`, `migration-latest.json`, canary metrics, production graph/browser evidence and the reconciled deployment payloads.
