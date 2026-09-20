# Validation reports

The original reports below were generated on 2026-07-17 from a Linux x86_64 development host. Thresholds are bounded regression checks for that environment, not production service-level objectives.

* `load-operational-local.json` -- 250 concepts, 16 workers and 1,000 direct requests, followed by same-base write contention, an idempotent replay storm, proposal concurrency and a backup/restore drill. All scenarios passed.
* `load-http-local.json` -- the bounded direct/operational checks plus 3,010 authenticated Streamable HTTP tool calls over 10 seconds at eight concurrent workers. The HTTP mix was 40% status and 60% search. No HTTP operation failed.
* `load-semantic-local.json` -- 100 concepts and 200 direct requests plus 200 searches through the vendored GTE-small model, Rust FFI and SQLite vector index. Semantic readiness was true and no degradation warning was emitted.

Each JSON document records the Git revision, host and Python information, operation counts, throughput, latency percentiles, errors, invariants and threshold results. `passed=true` means every included scenario and invariant passed.

The historical Python 3.14 local container rebuild produced image ID `sha256:2b508ad4e469d272bf9d43559fcbc2e1825f5b31f60c83f1ed2940b457e1726d`, and the image-contained model digest matched `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`. That image ID belongs only to the recorded local run. That statement applies to the historical Python/Rust releases. The `v1.0.0` replacement builds with pinned Go and distroless Debian 12 bases; each GitHub release records its immutable multi-architecture OCI digest, provenance attestation and SPDX JSON SBOM.

Graph evidence includes the 2,000-node fixture performance record and a DiskStation browser capture under [`graph/`](graph/). The real-target MCP benchmark is [`diskstation-memory-benchmark-2026-07-19.json`](diskstation-memory-benchmark-2026-07-19.json).

Go-port records include the [final pure-Go v1.0.0 validation](go-v1-validation-2026-09-19.md), [Go/Python 1,000-concept benchmark](go-python-models-off-benchmark-2026-09-19.json), [x86-64 and ARM64 scalar/SIMD model results](go-real-model-simd-2026-09-19.json), and the retained [NAS Bookworm Vulkan route](vulkan-nas-bookworm-2026-09-19.md). The Vulkan report belongs to the separate Rust/wgpu experiment: it preserves a working dependency recipe but does not claim Go parity, warm benefit or full-service memory suitability.

## Release and deployment evidence

[`release-0.5.9.md`](release-0.5.9.md) records the latest live DiskStation deployment and issue #33 embedding-worker recovery, lock handling, liveness and resumed computation. The pure-Go `v1.0.0` branch has passed local and CI replacement/rollback contracts, but no live production replacement report exists yet. [`release-0.5.8.md`](release-0.5.8.md) records issue #28 proposal visibility, asset/state preservation and historical-base handling. [`release-0.5.6.md`](release-0.5.6.md) records issue #23 typed-reference checks and the earlier prolonged storage-bound startup. [`release-0.5.5.md`](release-0.5.5.md) records issue #21 manifest/file/range retrieval checks. [`release-0.5.4.md`](release-0.5.4.md) records reviewed archival of the issue #20 fixtures and semantic opacity/shared-force verification. [`release-0.5.1.md`](release-0.5.1.md) records the earlier graph and Trash checks, restored runtime-model archive and preserved DiskStation model volume. [`release-0.5.0.md`](release-0.5.0.md) records the preceding safety and recovery release. [`release-0.4.2.md`](release-0.4.2.md) records the earlier issues 14--19 and graph-audit checks.

[`release-0.3.27.md`](release-0.3.27.md) records the previous issue 13 contract and diagnostic checks, pinned uMCP dependency and state-preserving deployment. [`release-0.3.26.md`](release-0.3.26.md) records the earlier persistent Streamable HTTP checks and state-preserving deployment. [`release-0.3.25.md`](release-0.3.25.md) records the earlier embedding-preserving deployment, live graph and semantic state, and execute-only asset metadata checks. [`release-0.3.24.md`](release-0.3.24.md) records the bounded inventory and manifest comparison deployment, including progressive embedding convergence. [`release-0.3.23.md`](release-0.3.23.md) remains the historical answer, redeployment and tagged-source corpus record. These reports distinguish release gates from checks that ran against the live service.

## Historical load reports

The `load-*.json` files were produced by the removed Python reference harness and remain immutable historical evidence. They are not reproducible from this pure-Go branch. New performance evidence uses repository-owned Go benchmarks, pprof profiles and explicit target-host acceptance records; it must be written to new files rather than replacing these baselines.

## Access evidence

Access-management evidence should record principal names, roles and outcomes only. Never capture one-time credentials or the master key. Reproduction environments create disposable test principals and revoke them afterwards.
