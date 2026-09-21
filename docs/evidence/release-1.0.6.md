# Memento 1.0.6 production validation

Memento 1.0.6 is running on the DiskStation with the graph diagnostics and navigation fixes. State-copy canary, authenticated production lifecycle, restart, graph/browser and metrics checks passed on 21 September 2026.

## Build and test evidence

- Source: `4c1f4453a17d446070e7fd81dfd2b241b0fa8256`.
- [Exact-commit CI](https://github.com/rcarmo/memento/actions/runs/35663625709): full Go audit, 100% statement coverage, amd64/ARM64 real-model and allocation gates, ARM64 corpus parity, amd64 replacement-container contract.
- Browser CI: two Chromium and two WebKit runs, each 18/18, plus diagnostics unit tests, Gherkin binding checks and interruption cleanup. The intentionally interrupted run records a failure and is separately asserted by the lifecycle test.
- [Release workflow](https://github.com/rcarmo/memento/actions/runs/35665448739): native multi-architecture image, provenance attestations and SPDX SBOM published.
- [Release](https://github.com/rcarmo/memento/releases/tag/v1.0.6).
- OCI index: `sha256:0837a0559704d454f882c7df08b0d6311f683435ef61005014e1f94ab8b36699`.
- amd64 manifest: `sha256:6c3c9e958d22607bba8112078d8c939b01deedb84895b32ee038231db2443856`.
- arm64 manifest: `sha256:dd025794ba863242896adf764f3d6fd33bb3738f7d623d05590a61b6d5480a9e`.
- Installed amd64 image config: `sha256:9a8c5162c14c768c4930e6fffb9c36df1025680c737b4e91b505d5bc3fba96c1`.

The [diagnostics audit](diagnostics-ui-audit-2026-09-21.md) records the reproduced defects and fixes. [Browser reproduction instructions](../../tools/browser/README.md) use the same commands as CI.

## Backup and rollback

Production v1.0.5 was stopped for a consistent native backup, then restarted while the backup was independently verified. Backup directory: `/volume1/docker/memento/config/backups/20260921T230154Z`; repository revision: `ad74840c18ceebcc27b1e1ddc51288b9175df6c2`.

| File | Verified SHA-256 |
| --- | --- |
| control.sqlite | `d0f3694b84d507e360789b1ab60738047b2dc20763d8db8c1256efa8b8177150` |
| derived.sqlite | `b013c0d563eb8cca1968cb6601b2b4a646a080b71e9813284e2e28f192fad87c` |
| repo.git.tar.gz | `a3836a9a99dfa389488c36bede3baac9cc287466b3df2c1d1ed76cd13e8cc699` |

Both SQLite integrity checks and Git fsck passed on the exported copy. The canary used a separate restored directory, verified against the same manifest. The previous image remains available at `sha256:57299ee202db5389957d7a4bcb635953ed959ee285e3bdb867ff2c0ad780cd84`; the prior stack payload is retained locally. No rollback was needed after qualification.

## Canary and production

The immutable-image canary passed authenticated create, patch, rename/read, trash, restore, purge and empty-marker search. Restart retained index and embedding convergence. Cold startup temporarily refused connections while loading; health subsequently became healthy with no OOM or automatic restart. Two canary metrics collections succeeded.

Only the image digest changed in Portainer endpoint 18, stack 111. Production container `77cd4afa64fb3827875daced898580a387ecf161d1c687998df0e808e3fc2e82` retains the same ports, mounts, UID/GID, read-only root, dropped capabilities, no-new-privileges, memory limits and requested PID limit. The NAS kernel still does not enforce the requested PID limit.

Production passed create, patch, rename, restart/read, trash, restore, trash, purge and search. Indeterminate writes were reconciled using their original idempotency keys. No uncertain mutation was blindly resubmitted. Final repository/index revision: `1f301b7f6a9829367da503f0c5d3e8499f6a1abe`; visible concepts returned to 257 and the validation marker search returned no results. Git history retains the disposable lifecycle commits.

Read-only derived-state inspection found zero disposable concepts, 259 ready embeddings, and all ten persisted chunks for `/skills/documents/bento-slides.md`. Each vector remains 1536 bytes (384 FP32 values). Derived SQLite was 8,978,432 bytes with zero WAL bytes; control SQLite was 69,644,288 bytes with an 82,432-byte WAL.

## Live graph and metrics

API and Chromium checks passed against both canary and production:

- Website Device Screenshots has one explicit outbound link, is not orphaned and has only its own size diagnostic in the selected sidebar.
- Overview reports 16 unresolved explicit links; the 53 accepted asset edges are excluded from that total. Stored Markdown was unchanged.
- Detail and neighbourhood diagnostics target only their returned scope. Show Trash returns the larger view.
- Inspector link navigation works, asset versions are newest first, and sidebar controls are left aligned.
- No browser page errors were observed. Local/CI failure injection covers retry, inaccessible details, failed neighbourhood loading and asynchronous races.

Final graph requests completed in 600–889 ms. Three production scrapes spaced 30 seconds apart returned `memento_metrics_collect_success 1` in 155 ms, 8 ms and 10 ms. The cold-storage caveat from [1.0.5](release-1.0.5.md) still applies; HTTP 200 alone does not indicate collection success.

Container health is healthy, automatic restart count is zero and OOMKilled is false. The sampled memory usage was 110,628,864 bytes, peak 356,216,832 bytes, below the 536,870,912-byte limit.

All four stopped 1.0.6 rollout helpers were removed without deleting volumes or backup directories. Four stopped 1.0.5 helpers had also been removed. Scripts, raw evidence, manifests and screenshots are retained under `/workspace/tmp/memento-rollout-106/`; production state was not used as a browser fixture.
