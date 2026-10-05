# Release 1.0.11 qualification

The proposal backlog/listing fix passed local and GitHub release gates and was deployed on 5 October 2026. Publication and production verification are recorded below. The earlier limits in [the initial proposal investigation](proposal-list-backlog-2026-10-05.md) describe the state before worker profiling was added; the results below supersede those limits.

## Changes

- Count submitted and approved proposals as actionable backlog; expose unresolved and per-status counts separately.
- Add pending/all/unresolved/status selection, exclusions, stable tuple ordering and permission-bound cursors; scan hidden candidates to fill visible pages.
- Capture CPU and allocation profiles for each package process, fuzz worker, model subprocess and deliberately killed repository helper. Test-only overlays do not enter release artifacts.
- Retain profiles, matching binaries, commands, toolchain, source snapshots, logs and cumulative reports in CI artifacts for seven days, including both model architectures and the final UI-focused Go check.

Historical Python oracle fixtures, production storage formats, numerical tolerances and performance ceilings are unchanged. NAS remains CPU-only.

## Completed gates

The tested source is `e7c44e173b90b65b202a522f1ae48e82cb2a7743` plus the retained worktree patch and untracked source snapshots. Final documentation and CI artifact placement were reviewed afterward; application and test source did not change.

| Gate | Evidence under `build/release-1.0.11-evidence/` |
| --- | --- |
| Full audit: formatting, layout, vet, pinned Staticcheck, offline suites, coverage, nested uMCP, vulnerability scan, race, fuzz, core allocation budgets, SSE2 smoke and cross-builds | `audit-final.log` |
| All 61 fuzz targets, 10,000 requested iterations each, two workers | `fuzz-complete.log`; repeated in `audit-final.log` |
| Root production coverage 100.0%; nested uMCP library coverage 100.0% | `audit-final.log`; example still runs in normal/race tests |
| GTE, subprocess chunking, Needle model/tokenizer/generation and full corpus | `models-retry.log` |
| Static amd64/arm64 archives, checksums, ELF/layout and executable smoke | `release-check.log` |
| Pure-Go image contract, persisted archive/readability checks | `container.log` |

The final audit contains 79 profiled invocations and 336 process records: 214 coordinators/helpers and 122 fuzz workers. All final package test/analysis statuses and all process capture statuses are zero. CPU sampling is Go's default 100 Hz; allocation sampling is 524,288 bytes; toolchain is Go 1.26.6. Logs and profiles remain under `build/profiles/release-audit-final/` and `build/profiles/release-model-retry/`.

## CPU and allocation analysis

The representative offline service run (`run-20261005T173341Z-KMgtkq`, process 642843) has 15.34 seconds of CPU samples over 56.38 seconds. SQL locking accounts for 39.18% cumulative CPU and SQLite execution 33.96%; syscall time is 25.49% flat. Its 4,046 MiB allocated includes 2,085 MiB cumulative JSON decoding and 1,852 MiB runtime-model construction; these overlap. Of 29.92 million allocation objects, JSON unmarshalling accounts for 21.05 million cumulatively. Repeated test runtime/catalog setup dominates these broad-suite allocation totals.

The equivalent full service race workload (`run-20261005T173926Z-Mk0E3P`) retains 102.20 seconds of CPU samples over 150.35 seconds. SQL locking is 40.73% cumulative CPU. Allocation totals are 5,298 MiB and 35.34 million objects, with runtime-model construction accounting for 2,007 MiB and 17.59 million objects cumulatively. Race instrumentation and different exercised scheduling prevent treating the difference from the offline run as an application regression.

Control fuzz profiles include actual worker processes. The inspected worker in `run-20261005T174507Z-So7RAx` spends 95.24% cumulative CPU in fuzz coordination and 88.10% in its JSON decode path, with 301.86 MiB allocated largely by the fuzz mutator. This describes mutation/IPC overhead; proposal-query benchmarks isolate the application path.

The 350.86-second Needle corpus run (`release-model-retry/run-20261005T171907Z-tNrAv2`) has 350.94 seconds of CPU samples. Attention is 64.78% flat and 73.51% cumulative CPU; AVX2 AXPY is 23.02% flat. Of 3,678 MiB allocated, resizing float buffers is 1,766 MiB flat and router encoding 1,299 MiB flat. Tokenizer parsing accounts for 32,780 of 97,205 allocation objects. These model paths were not changed by this patch. Reusing per-request projection buffers merits separate model work, with parity and cancellation gates preserved.

The separately captured GTE worker (`release-model-retry/run-20261005T171846Z-7822Ae`, worker 580720) allocates 266.45 MiB, of which model loading accounts for 256.76 MiB cumulatively. This is cold subprocess model-load cost; it does not establish warm-worker performance or justify enabling NAS acceleration.

The same-workload proposal benchmark comparison is retained in the initial investigation: bounded slice preallocation reduced 31,300 to 22,480–22,481 B/op and 646 to 641 allocations/op, about 28% fewer bytes. Repeated timings were 225,127–236,592 ns/op; host variability prevents a latency-improvement claim. Final core benchmarks retain zero allocations for SIMD, tokenizer vocabulary lookups and semantic cosine; lexical search reports 42,482 B/op and 866 allocs/op. All existing allocation budgets passed unchanged. Further JSON/catalog and SQLite row-conversion allocation reductions are separate work.

## Capture limits and failed attempts

There are 75 empty CPU samples in the final audit's short-lived processes. Heap captures exist, but these runs cannot establish CPU performance. Representative service/control, race, fuzz and model workloads have non-empty CPU samples. Repository helpers destined for SIGKILL flush at the pre-kill rendezvous; no post-kill sampling is possible. This retains abrupt-recovery semantics rather than substituting a graceful exit.

Earlier compile failures, interrupted race execution, the initial nested example coverage failure, an interrupted fuzz campaign and initial model-worker capture failures remain recorded as failed/incomplete evidence. Passing retries replace their acceptance role without deleting their logs. The nested example is excluded only from the library coverage denominator; it remains tested. Source snapshots use non-Go suffixes so vet does not discover generated test hooks as extra packages.

Independent review attempts timed out, including the final read-only attempt. No delegated approval is claimed. Manual review found and fixed missing model-job artifact retention and moved quality artifact upload after the UI-focused Go check. Final shell syntax, whitespace and workflow structure checks passed before publication.

## Deployment contract

Retain the current production rollback image `ghcr.io/rcarmo/memento@sha256:34ba60106ec745c7cc9eee340abbe52a6e345ad409a237ba796af8c29e209857`. Portainer endpoint 18, stack 111 uses the existing `/volume1/docker/memento/state` bind mount, read-only configuration/secret mounts and preserved external model volume. Only the image reference changed; production data was not migrated or erased.

## Publication and production verification

Tag `v1.0.11` points to `8f53c4ecf1f517652d53fc2b823684af6d105b7f`. [CI run 37353118541](https://github.com/rcarmo/memento/actions/runs/37353118541) and [release run 37353149955](https://github.com/rcarmo/memento/actions/runs/37353149955) passed, including native amd64/arm64 model gates, ARM64 corpus, no-AVX compatibility, UI audits, provenance and SBOM publication.

[Release 1.0.11](https://github.com/rcarmo/memento/releases/tag/v1.0.11) records multi-architecture digest `sha256:8b24b9cbff8da335ca3a15d08cf880f24a89e5695d6b2b2fc987ff2035e6bca2`. The SBOM SHA-256 is `2786364bc366133d6dd774eee84024a6dd11428835b8ca784666b290fe315daf`.

NAS Docker pull requests timed out without making the digest reference available. A local pull verified the published digest; Docker save/load transferred that exact amd64 image. Although the load and stack-update HTTP calls also exceeded their wait limits, subsequent inspection reconciled their successful effects. The NAS stack is pinned to immutable image ID `sha256:10d04fa937cc2a801024815b90b7a42d35dec9a9bc89f53c1ee4f27e770bc3b2`, matching the locally verified release image. It does not rely on a mutable tag or an absent NAS registry digest mapping. The previous pinned image and rollback compose are retained. A future host must pull the registry digest or import this image before using its local image-ID pin.

The new container `4194e2a7e9ab14529e995695c7991ee12cff88f12e54c4567807a03c1e082271` is running and healthy with zero restarts. Config/secret mounts remain read-only, state and preserved model volume are unchanged, and the original user, limits, capabilities, port and healthcheck remain in force. Deployment receipts are in workspace notes `notes/projects/memento-release-1.0.11/`.

Live MCP reports version 1.0.11, 264 visible concepts, backlog zero, unresolved zero and counts `{applied:340, expired:105, rejected:15}`. Repository and index both read `e4aa94b3b6af9b18a037221286ff027f35181611`, with `index_stale:false`. Default/pending and unresolved lists are empty; history exclusions, created-at descending order and cursor continuation returned expected distinct records. Reusing an updated-at descending cursor with ascending order was rejected as invalid/stale. Accepted 1.0.2 skill assets and preserved CCK were verified during proposal cleanup.

Semantic search still reports embedding revision `partial`, `ready:false` and SQLite vector support disabled, as it did before deployment. Needle routing is loaded. Deployment did not repair or claim full semantic readiness.

## GitHub profile review

Retained artifacts include `release-test-profiles` (1,252,481,074 bytes), `go-test-profiles` (1,255,518,433), amd64 model profiles (95,504,930), ARM64 model profiles (91,880,868) and UI audit evidence (6,330,960). Release and both model artifacts were downloaded to retained evidence. Release capture has 327 processes, including 122 fuzz workers, with no nonzero capture statuses and 66 short-process CPU warnings. Empty CPU captures remain unsuitable for performance claims.

The equivalent offline service CI run has 20.97s CPU samples and 4,052.08 MiB allocated versus the local 4,046.39 MiB workload; allocation objects are 29.19 million versus 29.92 million. JSON/runtime setup and SQLite remain the principal costs; sampled variation does not establish a regression. The CI race workload has 142.75s CPU samples and instrumentation overhead and is not a latency baseline. AMD64 model CPU remains attention/AXPY-dominated; model tests allocate 786.86 MiB and 167,424 objects, with tokenizer construction prominent. Core allocation budgets and native model parity passed without wider budgets.

The separately committed project-path policy is post-release tooling work; it does not move this immutable release tag or alter the deployed binary. Its focused profiling and unresolved local visual-audit limits are recorded in `project-paths-2026-10-05.md`.
