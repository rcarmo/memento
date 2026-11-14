# Memento 1.1.0 release and NAS deployment

Published [v1.1.0](https://github.com/rcarmo/memento/releases/tag/v1.1.0) and installed it on the NAS on 14 November 2026. Native Gemma is preserved as an explicit offline source/CLI option, off by default. Production continues to use GTE/Needle with its existing configuration and indexes.

## Release

- Source: `8c924d985cba56a65657ec307c72010b70df7fdf`; annotated tag object `dd744d6053eabfc699c89dfa0c6af575872cc93b`.
- Main CI [43531374694](https://github.com/rcarmo/memento/actions/runs/43531374694): all four jobs passed, including repeated Chromium/WebKit audits, native amd64/ARM64 models, replacement container and exact ARM64 Needle corpus.
- Release [43531488674](https://github.com/rcarmo/memento/actions/runs/43531488674): all nine jobs passed, including pure-Go audit, reproducible static archives, amd64/ARM64 image builds, Westmere no-AVX compatibility, provenance and publication.
- Actual published archives passed `SHA256SUMS`; amd64 reports `memento-go 1.1.0`. Both contain the existing five production commands and no Gemma executable/checkpoint. SBOM SHA-256: `4bf437ef3159750cab5ee87fab8bfa794d42dbe99abaf295d276d4f36acf00d9`.
- Multiarch image: `ghcr.io/rcarmo/memento@sha256:67b2474c444ab2c30d93309852e260e0ca4d176183690530a2cc9ac36e1186b9`.
- amd64 manifest: `sha256:e5f87b78b9bff74c2de076a499952ce6d949367c5723ec8ccf4d199e373015d9`; ARM64 manifest: `sha256:2daf190c77bcceffb6098976a1ade8158bfd9924cfddd0d077bbb3452140feda`.
- NAS image ID: `sha256:23a3ada842a66ffebdad8711a5ef31fcd95222711272fb4aaadf2c85e95b2d29`; inspected amd64/version/source/non-root metadata matches the tagged build.

The superseded main run43531066409 was cancelled by normal same-ref concurrency after the version bump. It supplies partial diagnostic captures only; it is not passing release evidence.

## Profiling and disposal

Downloaded complete tagged/main profile ZIPs, checked ZIP integrity, reviewed CPU flat/cumulative, `alloc_space`, `alloc_objects`, child and fuzz-worker records, then deleted raw local/Actions copies. An initial download timed out during extraction; final conclusions use the subsequent complete integrity-checked capture.

| Capture | Package/process summaries | Bad statuses | Short/empty CPU summaries |
|---|---:|---:|---:|
| Superseded main partial | 61 | 0 | 15 |
| Final main amd64 models | 18 | 0 | 0 |
| Final main ARM64 models/corpus | 19 | 0 | 0 |
| Final main quality | 208 | 0 | 70 |
| Tagged release quality | 202 | 0 | 64 |

Empty CPU captures are incomplete sampling for those short paths. Longer service, model, race and performance profiles contain real samples. Main process manifests include 132 fuzz workers/228 other records; tagged manifests include132/222. Browser-runner profiles measure Node/driver work rather than browser-engine allocation.

Measured hotspot shape matches the existing qualification: main service ~17.88 s CPU/~4151 MB cumulative allocation, SQLite locking ~31.5% cumulative CPU; main race ~5463 MB allocation, models-off runtime construction ~2074 MB/38.0%. Tagged service ~22.78 s CPU/~4196 MB allocation; tagged race155.90 s CPU/~5342 MB allocation. Repeated model loading, JSON fixture/runtime construction and SQLite dominate; real GTE/Needle allocation budgets and corpus checks pass. These existing paths were not changed or linked to Gemma. No new runtime performance improvement is inferred from differences between instrumentation/workloads. The earlier measured Gemma worker optimisation stays isolated and optional; [integration qualification](gemma-main-option-2026-11-13.md) and [NAS comparison](gte-gemma-nas-2026-11-13.md) retain its numerical/resource trade-offs.

UI artifacts contained two full 18/18 Chromium and two full18/18 WebKit passes, plus the deliberately interrupted lifecycle-only Chromium probe. The repeated UI job passed. Raw artifacts and disposable download/build/test/log data were removed after review; only concise conclusions and intentional release/rollback assets remain.

## NAS installation

Portainer endpoint18/stack111. Saved the previous compose/image/mount/security/resource baseline and retained the v1.0.12 image. Pulled the new immutable manifest before installation. The generic pull workflow rejected digest syntax; the raw Docker API pull succeeded. Updated only the compose image field with `Prune:false`, `PullImage:false`.

New container `079170706dadc376f68dc4691c5fe1e331d88e84860f2c4c47f77912e780fa62` started at `2026-11-14T00:40:37.848937536Z`. Final state: running/healthy, zero restarts, no OOM. Sampled container charge265,519,104 bytes (~253 MiB), below the unchanged512 MiB limit. This single memory snapshot is not a new long-run peak measurement.

Verified unchanged:

- State bind `/volume1/docker/memento/state`, read-only config/env mounts and external model volume.
- Non-root65532, read-only root filesystem, dropped capabilities, no-new-privileges, ports and memory/resource limits.
- Repository/index/embedding revision `c9c3bb7eca06868b23aeae1b356c0c9f53349bd1`, schema2, non-stale index and111 visible concepts for the same shared-reader principal.
- GTE `rust-gte`/384 ready, Needle loaded, proposal backlog0. No forced refresh or canonical repository write.
- Four Bearer capitalisations initialise as1.1.0 and `ping` returns `{}`; missing authentication returns401.
- Identical semantic query preserves top3 IDs/order; measured maximum score difference0.

The first HTTP probe arrived while startup was incomplete and received a connection reset. A bounded35-second wait/retry passed; no restart was needed. No Gemma backend or index migration occurred.

Rollback uses the retained image `sha256:a42e4e4d62c0f385198aaf918f1488067873fa419af1914d521baa464d8044e3` and previous compose with the same mounts/limits. Workspace qualification and rollback materials live under `notes/projects/memento-release-1.1.0/`; no credentials are recorded in this report.
