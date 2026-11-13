# Memento 1.0.12 publication and deployment

Memento 1.0.12 was published and deployed to the NAS on 13 November 2026. Production MCP `ping` and case-insensitive Bearer scheme checks pass. Data, configuration, model volume and CPU-only GTE/Needle policy are unchanged.

## Source and release

- Main/tag source: `dd47078b6ed4fc77a23785e46e66834af456a7ac`, merged [PR #45](https://github.com/rcarmo/memento/pull/45). Issues #43 and #44 closed on merge. Gemma work remains on its separate feature branch.
- Annotated `v1.0.12` object: `1db6a924fba54a46ac12f33b5220a52799beb325`. The tag is immutable; this deployment record is a later documentation commit.
- [PR CI 43398351631](https://github.com/rcarmo/memento/actions/runs/43398351631), [main CI 43402119764](https://github.com/rcarmo/memento/actions/runs/43402119764) and [Release 43402127596](https://github.com/rcarmo/memento/actions/runs/43402127596): all jobs succeeded.
- [Release assets](https://github.com/rcarmo/memento/releases/tag/v1.0.12): AMD64/ARM64 static archives, SHA256SUMS and SPDX SBOM. Downloaded release checksums verified; SBOM SHA-256 `81168595c4660c17b7a9fcdd35de59774739806909fd6f08636df4ce99e92a7c`.
- Multiarch manifest: `sha256:2818379fcc621b2731dd6e998063e39dc8b540e3739b733e9a25d4078d7d7bf6`.
- AMD64 platform manifest: `sha256:dd0b47613e177c94a8dfbea3edece264f24424b52c834d48b09495ed1865707a`; deployed image ID: `sha256:a42e4e4d62c0f385198aaf918f1488067873fa419af1914d521baa464d8044e3`.

The patch adds shared protocol ping and RFC-compliant scheme parsing while retaining exact credential bytes, managed authority and session boundaries. Release qualification also isolated an existing filesystem-order-dependent coverage fixture, updated vulnerable Go 1.26.6 pins to patched Go 1.26.11, and aligned profiling scratch/disposal and release archive publication. No vulnerability or coverage gate was weakened.

## Gates and profiling

Complete local audit, root/nested tests, race, 61 fuzz targets, lint/vet, full production/library coverage, allocation budgets, model/subprocess tests, native CI models/corpus, browser audits, cross-builds, static archive/layout/ELF checks, container persisted-state contract and no-AVX smoke passed. Go 1.26.11 reports zero reachable vulnerabilities. Uncalled dependency advisories remain visible in govulncheck output.

CPU, alloc_space and alloc_objects were inspected for local, PR CI and release captures, including children/fuzz workers. Final release records: 185 packages, 327 processes, no nonzero test/analysis statuses; 58 short-process empty CPU warnings. Empty samples are not CPU evidence. Representative offline/race/model captures were nonempty.

Release service CPU: 22.14 s; alloc_space 4,175.71 MiB; 28.72 million objects. Runtime setup accounted for 1,878.32 MiB cumulative; SQL locking 31.57% cumulative CPU. Race: 150.80 s CPU, 5,454 MiB and 35.55 million objects; SQL locking 36.15%. Equivalent PR/local offline allocations were about 4,163–4,203 MiB. Broad setup JSON/catalog/SQLite and existing model arithmetic dominate; sampling/instrumentation prevent treating these as application latency comparisons. Existing allocation and numerical gates passed unchanged. No performance gain is claimed for the protocol/security patch.

AMD64 GTE benchmarks were about 16–18 ms warm, 32 B/2 allocations; cold workspace about 150 KiB/17 allocations. Needle generation about 74–77 ms, 1.77 MiB/387 allocations. These are host observations, not NAS latency claims. ARM64 full corpus captured 524.92 s CPU and passed; AXPY/attention and workspace construction remained the existing model hotspots. No model optimisation or GPU change was included.

Raw captures, matching binaries, disposable logs/probe fixtures and downloaded profile artifacts were removed after analysis. Analysed uploaded profile artifacts were deleted through the Actions API; no indefinite raw archive was retained. Concise working conclusions live in workspace notes `notes/projects/memento-release-1.0.12/`.

## Production deployment

Portainer endpoint 18, stack 111. NAS registry pull timed out and did not publish the new reference locally. A local digest pull verified the exact AMD64 image and OCI source labels; Docker save/load transferred that image successfully. The NAS is pinned to its verified immutable image ID above, not a mutable tag or an absent local registry mapping.

Only the compose image line changed. The previous pinned image `sha256:10d04fa937cc2a801024815b90b7a42d35dec9a9bc89f53c1ee4f27e770bc3b2` and rollback compose remain available. Config bytes, read-only config/secret mounts, writable state bind, preserved external models volume, UID, resource limits, ports, tmpfs, healthcheck, capabilities and security options all match the baseline.

Container `71a9a9af8392c3a688f6c94a57be2cc43cb7db5fbd3a5131e6462fd30b1892e8` started at 10:04:28 UTC. HTTP opened around 10:07:26, within the five-minute startup grace. Final status: healthy, zero restarts, no OOM. Sample memory was 213,385,216 bytes (~204 MiB); observed startup maximum 473,567,232 (~452 MiB), below the existing 512 MiB limit.

Live verification:

- Service reports 1.0.12. Canonical/lower/upper/mixed-case schemes each initialize and issue session-bound ping with `{}`.
- Missing/invalid credentials return 401; unknown session 404; bad protocol version 400; unknown method still `-32601`.
- Repository/index both remain `c9c3bb7eca06868b23aeae1b356c0c9f53349bd1`, `index_stale:false`, 112 visible concepts for the same infrastructure principal and backlog zero.
- Semantic search returned three results without warnings; Needle remains loaded. Existing 384-dimensional GTE and partial aggregate embedding readiness are unchanged. Startup refresh resumes through the already-configured policy; no manual full refresh or index migration was added.

Initial connection-reset probes during startup, the unavailable NAS registry pull and an incorrectly named semantic probe parameter were failures, not acceptance evidence. The corrected/ready live checks passed. Production rollback requires only restoring the retained 1.0.11 image line; there was no destructive data/schema operation.
