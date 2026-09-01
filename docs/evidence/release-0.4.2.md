# Release 0.4.2 validation

The 0.4 minor series was published and deployed on 2026-08-31 and 2026-09-01 UTC. The scheduled initial deployment occurred exactly one hour after `0.4.0` publication. Production acceptance then found two bounded-discovery defects; `0.4.1` and `0.4.2` corrected them and were deployed as immediate patch releases.

## Release identity

* Initial minor tag: `v0.4.0` at `bd171299a842338e3ee1615f01aa10d873754ffd`
* Final tag: `v0.4.2` at `32e5ab33f52e437907d44ff1f4a0b40663c75c11`
* Image: `ghcr.io/rcarmo/memento:0.4.2`
* Published multi-architecture manifest: `sha256:94dd3b148c3354593f2d6f5cb3e79f86f52377d9e11e5a1a9df3dd2821642b6d`
* Local linux/amd64 image ID: `sha256:f011abb045f9372c94d08524ff3522cbf2906e13f2409e6e3e6428320d8feda2`
* GitHub release: <https://github.com/rcarmo/memento/releases/tag/v0.4.2>

The exact final commit passed [ordinary CI run 33483045320](https://github.com/rcarmo/memento/actions/runs/33483045320). Python 3.12, 3.13 and 3.14 validation, coverage, the wheel build and installation, Rust checks, the container build and runtime-model smoke all passed. [Release run 33483267539](https://github.com/rcarmo/memento/actions/runs/33483267539) then passed tag validation, the Python matrix, native amd64 and arm64 builds, the no-AVX Westmere smoke, multi-architecture publication, GitHub release publication and retention.

Local final validation passed Ruff, formatting, mypy, 327 tests, browser graph checks, Rust formatting, Clippy, tests and doc-tests. The coverage run passed all 327 tests at 84.83%. Building and installing `memento-0.4.2-py3-none-any.whl` succeeded. The `0.4.2`, `0.4`, `0` and `latest` registry tags all resolved to the manifest above. Its OCI index contains amd64 and arm64 images plus both provenance attestations.

The initial `0.4.0` CI and release runs were [33436019369](https://github.com/rcarmo/memento/actions/runs/33436019369) and [33436257491](https://github.com/rcarmo/memento/actions/runs/33436257491). The production proposal archive exposed a status-filter pushdown gap, corrected by `0.4.1` at `be07d6c46444fc9ba4794b5d54f7e3da60c21317`. Final acceptance then exposed incomplete compact workflow discovery, corrected by `0.4.2`.

## Issue acceptance

### Issue 14 -- bounded proposal appraisal

`proposal_list` and `proposal_get` remain execute-only on the compact surface. Their summaries contain bounded change and asset metadata rather than concept bodies or ZIP payloads. SQL status pushdown in `0.4.1` avoids deserialising the historical proposal archive before applying the requested bound.

Live `memory_execute` checks completed without a timeout for submitted, stale and applied status filters. `submitted`, limit 1 returned an empty page and no cursor. `stale`, limit 5 returned exactly five summaries and a stable next cursor. `applied`, limit 1 returned one bounded summary and a next cursor. Detailed retrieval of one stale proposal returned per-change conflict metadata and asset digests without returning the staged ZIP.

### Issue 15 -- stale proposal revision

A live detailed read of stale proposal `39baa614-f68b-4b4e-b4f6-0f6c337b72b6` returned two indexed changes, current/base revisions and deterministic conflicts for each target. The compact operation catalogue and curate workflow advertise `proposal_revise`, whose contract requires an explicit conflict-free change-index subset and the fresh expected revision. Regression tests cover successful body-and-asset-complete subset revision, rejected conflicting/incomplete selections, authorisation, stale revisions and immutable originals. No production proposal was revised during acceptance.

### Issue 16 -- staged proposal asset retrieval

Live `proposal_asset_get` read the staged asset `9482776c-cff7-4148-9779-81edc023bf61` through `memory_execute`. The result reported the generic concept path, kind, version, ten-entry manifest, total uncompressed size and ZIP SHA-256. A request for `SKILL.md` with offset 0 and limit 64 returned exactly 64 UTF-8 bytes, `truncated: true`, `next_offset: 64`, the full-file digest and a digest for the returned chunk. The operation is now present in both compact curate and asset-pack guidance.

### Issue 17 -- commit reconciliation

`operation_get` is execute-only on the compact surface and appears in curate and asset-pack guidance. A live lookup of an operation attached to another principal returned `forbidden` rather than leaking its state, confirming principal scoping. Regression tests cover committed, in-progress, conflict, failed-before-mutation and indeterminate outcomes, durable idempotency-key lookup, bounded polling metadata and cross-principal denial. The asset-pack workflow now directs callers to reconcile an interrupted apply under the same principal before retrying.

### Issue 18 -- proposal-first curation

Live `memory_help` reported compact curate operations in proposal-first order: list, inspect, inspect a staged file, revise, reconcile, review and apply, followed only then by exceptional prune/direct-admin mutations. The asset-pack workflow similarly places staged-file inspection before review and reconciliation after apply. Generic digest parity and direct-mutation guards use concept paths and asset metadata rather than `/skills/`, `asset_kind="skill"` or namespace-specific logic. No direct production mutation was used during acceptance.

### Issue 19 -- MCP initialisation and recovery

After each replacement, a fresh MCP gateway connection discovered all 23 configured tools and immediately completed authenticated `memory_status`, `memory_help` and `memory_execute` requests without scripts, raw protocol calls or a process restart. The final service reported `0.4.2`. An unauthenticated request from inside the production container returned HTTP 401 with a Bearer challenge. Regression tests cover cold-start initialisation, one shared in-flight initialisation attempt, failed-initialisation cleanup and a subsequent successful retry.

## Graph audit acceptance

A live execute-only `audit` request with severity `info` and limit 1 returned one policy-visible graph diagnostic and a next cursor. Its repair guidance was explicitly read-only and directed the caller to inspect current memory and draft a normal proposal; no repair action ran. The response retained the current repository and index revisions.

## DiskStation deployment

The `0.4.0` GitHub release was published at `2026-08-31T20:35:35Z`; its scheduled deployment began at `2026-08-31T21:35:35Z`, exactly one hour later. The `0.4.1` and `0.4.2` images were immediate production-acceptance patches rather than new scheduled feature deployments.

Portainer endpoint 18, stack 111 was finally updated from the immutable `0.4.1` digest to the immutable `0.4.2` digest. The captured Compose diff changed that digest and nothing else. The final update request ran from `2026-09-01T07:55:46.950Z` to `2026-09-01T08:01:16.379Z`.

The replacement reported:

* container ID `11f4498fee97a3f03600e7260c11406522b6709ecd33a3e12ae3d5381eced918`;
* the linux/amd64 image ID and published repository digest above;
* OCI version `v0.4.2`, revision `32e5ab33f52e437907d44ff1f4a0b40663c75c11` and the expected source repository;
* running and healthy status with zero restarts;
* UID/GID `65532:65532`, read-only root, all capabilities dropped, `no-new-privileges:true` and init enabled;
* the original read-only configuration and secret mounts, writable state bind and runtime-model volume;
* the existing 512 MiB memory limit and 256 MiB reservation.

The container started at `2026-09-01T08:01:02.929Z`, logged `serve_starting` at `08:17:46Z`, then became healthy without a restart. Docker still reports `PidsLimit: null`; the existing Synology/Compose discrepancy is unchanged.

## Preserved state and readiness

Before patch replacement, the authenticated service reported 176 visible concepts, repository and index revision `88592a72e38d4c1fb0ef8844dadab802a759384b`, no proposal backlog and `index_stale: false`. Final `0.4.2` acceptance reported the same values.

Semantic search remained ready through `rust-gte`, with 384 dimensions, the SQLite vector extension and the preserved embedding revision. The Needle router remained loaded through Rust FFI. No configuration helper, model preparation, index rebuild or embedding refresh was invoked during deployment.

## Remaining operational work

* Enforce or explain the missing production PIDs limit.
* Migrate principal grants before enabling protected read prefixes on the existing deployment.
* Run a clean-host restore drill.
* Measure semantic and Needle performance on real ARM64 hardware.
* Attach SBOM material to published releases.
* Add TLS before exposing any HTTP surface beyond the trusted LAN.
