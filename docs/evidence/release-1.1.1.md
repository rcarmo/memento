# Memento 1.1.1 release qualification and NAS deployment

Version [1.1.1](https://github.com/rcarmo/memento/releases/tag/v1.1.1) is published and **production is healthy on 1.1.1**. Rui explicitly authorised the successful image-only activation at 21:25 UTC on 16 November 2026. An earlier attempt that day was rolled back after slow stack orchestration exceeded the request timeout; that history and the later verified activation are recorded separately below.

## Published release

- Source: `61c43cb8b44c401f49d5fc30a54ee58e259584f4`, merged PR [#47](https://github.com/rcarmo/memento/pull/47).
- Main CI [43638945362](https://github.com/rcarmo/memento/actions/runs/43638945362): all four jobs passed, including native amd64/ARM64 real models and replacement-container checks.
- Release [43638978169](https://github.com/rcarmo/memento/actions/runs/43638978169): all nine jobs passed, including pure-Go gates, static archives, multiarch images, no-AVX compatibility, provenance and publication.
- Published at `2026-11-14T21:10:37Z`. Freshly downloaded archives matched published `SHA256SUMS`; the published amd64 binary reports `memento-go 1.1.1`. Archives contain the existing five production commands and no Gemma or Needle 3 command.
- amd64 archive SHA-256: `8496a35c53418ed87270d56eff23bd2a23a66166d3d50744539cc17085a4cd98`.
- ARM64 archive SHA-256: `0cce5b00226dc18db2b7ab92d2943124447c09b8d2db6edd2953970a03028729`.
- Published SBOM SHA-256: `b825e6e560aa141dae4aa257987c1258c73924bbd826933eaaaba0dd93dc7756`.
- Multiarch index: `sha256:26046ef55d7678c411ecf71d24ef7e95829c57937d3e824a4ed405065fc4d71d`; its downloaded registry bytes independently matched that digest.
- amd64 manifest: `sha256:caad9e085bc1a839aec96e6b4582dce58354a61570653f232ccfaff5748294d0`; ARM64 manifest: `sha256:85589e5f03f5b02475471b2818ee134e2eed2bd0e74a764da8db723e6522947d`.
- Published amd64 image ID: `sha256:eef135e3f6fe6cfabc17218d65e695cb7478c355a8fadf3d8a461032aff2374d`, size 329,271,043 bytes (314 MiB). Version, source, architecture and non-root metadata were verified before use.

Older local archives bearing the same version have different digests. They were not substituted for the published assets.

## Profiling and measured tuning

Raw Actions profiles expired under one-day retention. Same-run job logs retain generated cumulative CPU, `alloc_space` and `alloc_objects` analyses, including nested modules, fuzz workers and model children. These textual analyses were reviewed; raw samples were not available for fresh reanalysis.

| Run | Package summaries | Profile summaries | Ordinary / fuzz / model processes | Test/analysis failures | Missing profiles | Empty CPU captures |
| --- | ---: | ---: | --- | ---: | ---: | ---: |
| Main CI | 241 | 1,197 | 261 / 134 / 4 | 0 | 0 | 70 |
| Tagged release | 202 | 1,071 | 223 / 134 / 0 | 0 | 0 | 66 |

Empty CPU captures are incomplete sampling for short processes. Longer service, race, model and corpus workloads contain samples. Supplementary local pre-PR profiles had 209 package summaries, no failed analysis, 229 ordinary and 134 fuzz-worker records; 63 packages included empty CPU samples. Their pre-PR revision and saved worktree patches do not replace the tagged-run provenance.

Largest service race CPU totals were 152.77 seconds (main) and 156.85 seconds (tag); SQLite locking accounted for 36.27% and 35.05% cumulative CPU. Largest service allocation totals were 5,363.94 and 5,365.68 MiB; repeated models-off runtime construction accounted for 37.07% and 37.40%. JSON decoding and SQLite/test-fixture construction remain the main costs in those workloads. ARM64 Needle corpus CPU was 530.18 seconds, with 82.98% in encoding; the real-model/corpus gates passed. Cumulative test allocation is not resident daemon memory.

The schema patch already addressed measured repeated registration: equivalent registration benchmarks improved median 9.197 to 6.252 ms (-32.0%), allocation 8.37 to 5.20 MB (-37.9%), and 82,409 to 48,396 objects (-41.3%), preserving registry-owned schema copies. See [the schema qualification](mcp-output-schemas-2026-11-14.md). The remaining large fixture/model/SQLite costs also appeared in 1.1.0; these different instrumented workloads establish no new runtime regression or whole-suite speed-up.

Downloaded logs and inactive local raw profiles, matching binaries and disposable fixtures are removed after this review. Concise measurements and intentional rollback records are retained. The unrelated older CI run 37366347422 still has no recoverable profiles; no retrospective profiling certification is inferred for it.

## Earlier NAS attempt and recovery

Portainer endpoint 18, stack 111. Initial production was healthy 1.1.0, pinned index `67b2474c…`, image `23a3ada8…`. A fresh read-only baseline covered inventory, semantic ranking, read/help/list, domain-error output, repository/index/model identity and runtime limits.

Two registry-pull paths stalled without making the new digest reference available on the NAS. The exact published amd64 manifest was instead pulled and verified on the build host, saved as a transport archive and loaded through the Docker API. The NAS image ID matched the published image. No replacement image was built. Because loading does not retain a registry digest alias, the candidate compose used the verified immutable image ID. The original compose and rollback image were preserved.

Only the compose image field changed, with `Prune:false`, `PullImage:false` and existing stack environment preserved. The update request timed out. The replacement container remained `created` during initial checks and MCP was unavailable, so rollback was requested. Subsequent Docker events show the 1.1.1 container did start and reached healthy state before the rollback took effect. It was then stopped cleanly. This sequence does not establish an application regression: the slow orchestration/start interval and request timeout need investigation before another deployment attempt. The incident included service interruptions; it was not a zero-downtime update.

Rollback container `58da99d997b987d35b133b0881421c6ab49b68d98c3ec631aa54d4e6661de0c3` started at `2026-11-16T14:16:54.032015466Z`. At the completed live comparison, it was running/healthy, with zero restarts and no OOM. At that point the original pinned 1.1.0 compose was restored and the loaded 1.1.1 image remained unused. The later explicit upgrade authorisation superseded that hold.

Verified after rollback:

- Same state/config/env/model mounts, non-root identity, command, read-only root, dropped capabilities, no-new-privileges, ports, PID limit and 512 MiB memory limit.
- Same 111 shared-reader concepts across two inventory pages, with identical inventory order.
- Repository/index/embedding revision `c9c3bb7eca06868b23aeae1b356c0c9f53349bd1`, schema 2, GTE `rust-gte`/384 ready, Needle loaded, proposal backlog zero.
- Same input/annotation/discovery metadata, successful read/help/list/search, and expected missing-read domain error.
- Same top-three semantic result IDs/order for the fixed query; measured maximum score difference zero.
- Four Bearer scheme capitalisations initialise as 1.1.0 and return `{}` from ping; unauthenticated ping returns 401.

No canonical repository write, index migration, model switch or forced refresh was performed. Workspace rollback receipts live under `notes/projects/memento-release-1.1.1/`. Those checks qualified the earlier rollback. The later activation and its live output-schema checks follow below.

## Authorised activation at 21:25 UTC

Rui requested `upgrade now`. A fresh baseline again verified healthy 1.1.0, 111 shared-reader concepts, unchanged repository/index/model identity and read/help/list/search/ping/domain-error contracts. The already loaded published image was checked against the exact registry manifest/config digests and canonical layer identities. No image was rebuilt or pulled again on the NAS.

Preflight initially stopped before mutation because Docker 24 on the NAS reported 329,266,781 image bytes, versus 329,271,043 on the Docker 29 build host. Exact config/image and layer identities matched. The control was corrected to use canonical asset identity plus the unchanged 512 MiB image cap, rather than cross-version equality of Docker's size-accounting field.

Exactly one image-only stack update was submitted at `2026-11-16T21:32:36.702Z`, with stack environment preserved and `Prune:false`, `PullImage:false`. Docker health was observed independently of the long-lived HTTP response for a bounded 600-second window. The stack request returned normally. The container started at `2026-11-16T21:36:54.187928378Z` and was observed healthy at 21:38:52 UTC, 376 seconds after submission. This was a disruptive recreate, not a zero-downtime deployment. The orchestration/start interval is measured; its underlying cause has not been isolated.

Active container: `2416cf46d9aabb55604570f6478bd25213e369828cf6ca4448c304fd4a65896e`. Active immutable image: `sha256:eef135e3f6fe6cfabc17218d65e695cb7478c355a8fadf3d8a461032aff2374d`, published version 1.1.1/source `61c43cb8b44c401f49d5fc30a54ee58e259584f4`. Final inspection confirmed healthy state, zero restarts and no OOM.

The live before/after comparison verified:

- Unchanged state/config/env/model mounts, environment, command, healthcheck, security, ports, resource limits and persisted compose; only the image reference changed.
- Same 111 concepts across two inventory pages, same inventory order and unchanged repository/index/embedding revision `c9c3bb7eca06868b23aeae1b356c0c9f53349bd1`.
- Schema 2, non-stale index, GTE ready at `rust-gte`/384, Needle loaded and proposal backlog zero; no migration or forced refresh.
- Same input schemas, annotations and discovery order; successful read/help/list/search/ping and expected missing-read domain-error responses.
- Advertised `outputSchema` on the stable visible help/status/search/read tools. The existing hidden list callback's runtime contract also passed; discovery was not expanded to advertise it.
- Identical top-three semantic IDs/order for the fixed query, maximum and mean score differences zero.
- Four Bearer capitalisations initialise as 1.1.1 and ping returns `{}`; unauthenticated ping returns 401.

The only changes on main after the published release were tests and documentation. The deterministic test fix `a1a3d57` also passed [CI 43956596732](https://github.com/rcarmo/memento/actions/runs/43956596732); it changes no production code. The deployed release therefore contains the latest production changes without rebuilding an immutable release for a test-only correction.

Rollback image `23a3ada8…` and the original 1.1.0 compose remain available with the same state/model/config mounts. Concise receipts are under `notes/projects/memento-release-1.1.1/deployment.json` and `upgrade-2026-11-16.md`. Temporary comparison IDs/scores, schema fixtures, scripts and logs were removed after analysis and safe owner checks. No GPU, Gemma activation, worker-reuse implementation or Needle 3 deployment occurred.

## Issue triage

[#37](https://github.com/rcarmo/memento/issues/37) now evaluates CPU-first pure-Go idle-capped embedding worker reuse. Old Python/Rust PRs #36/#38 were closed unmerged. The Go framed worker already handles multiple requests, but the service still starts a child per batch and has no idle-reuse setting. The revised issue requires an isolated CPU appraisal before any implementation approval; default cold workers, memory limits and failure/cancellation contracts stay explicit. No Vulkan work, warm-worker activation or deployment was started.

Needle 3 [#40](https://github.com/rcarmo/memento/issues/40) remains closed as not planned. Production retains Needle 2/GTE.
