# Optional Gemma on main

Rui authorised preserving Gemma on `main`, off by default, after the [NAS comparison](gte-gemma-nas-2026-11-13.md). This integrates the existing native engine, optional CLI, checkpoint manifest, numerical fixtures, comparison harness and measured reports. No service backend switch, production model migration, release or deployment is included.

## Integration boundary

Merged `main` at `3bca73d` into the Gemma branch at `c265830` using merge commit `3f929df`. This retains the 1.0.12 ping/Bearer fixes, patched Go toolchain and profiling disposal rules. The only textual merge conflict was whitespace in `AGENTS.md`; all instructions were preserved.

Gemma remains an explicit offline option:

- `.DEFAULT_GOAL := help` prevents the early `gemma-test` target from becoming the implicit plain-`make` command.
- Standard build/install, five-command Linux release archives and the production Docker image exclude both Gemma commands and its checkpoint. Runtime-model preparation does not fetch it. `go list -deps` verifies all five production commands have no `internal/embeddinggemma` dependency.
- The service keeps its existing `rust-gte` model ID, 384 dimensions and subprocess worker path. The default semantic-enabled setting is unchanged. An `EMBEDDINGGEMMA_MODEL` environment value cannot activate the service or replace GTE; an injected-environment regression test covers this.
- `make gemma-embed` explicitly builds the offline CLI; other `gemma-*` targets explicitly inspect, test, benchmark or reference-check a separately supplied manifest-pinned checkpoint. Serial workers remain the default. No automatic fallback or production backend selector was added.
- Normal source test/vet/lint/race checks include Gemma's model-independent packages. Real-model tests require `EMBEDDINGGEMMA_MODEL`. `gemma-coverage` reports the optional packages separately. The existing production zero-uncovered-statement gate retains its package set, including GTE; only the four newly integrated offline Gemma/comparison packages are outside that gate.

New regression tests inspect actual default build/install/release recipes, Docker/model preparation paths, default Make selection and resolved production Go dependencies. They fail if Gemma enters a normal artifact or startup path.

## Gate fixes

The isolated branch had not previously passed main's staticcheck/fuzz-coverage rules. Renamed internal Go tensor constants `Q4_K`/`Q5_K`/`Q6_K` to `Q4K`/`Q5K`/`Q6K`; numeric encoding IDs and external GGUF string labels are unchanged. Removed an unused width constant. Kept intentional nil-context rejection tests with local staticcheck annotations rather than removing those contract checks. Added meaningful CLI-admission and corpus-validation fuzz targets; every runtime Go package now has a fuzz target.

## Verification

Verification uses pinned Go 1.26.11. All listed completed gates passed:

- Formatting, module/layout checks and per-package fuzz-target coverage.
- Vet, pinned staticcheck and govulncheck. Govulncheck reports no reachable vulnerabilities; it also reports one imported-package and twelve required-module findings outside called code.
- Complete root `go test ./... -count=1` and `go test -race ./... -count=1`.
- Real-checkpoint Gemma engine/CLI tests, independent references and optional coverage. Engine coverage is 93.1%, inspect CLI 87.5%, embedding CLI 82.2%, comparison harness 28.8%; combined optional coverage is 81.9%. No zero-uncovered claim is made for offline experimental code. Model-independent optional coverage is lower because real-checkpoint paths are skipped.
- Production coverage remains 100.0% with the existing zero-uncovered-statement rejection.
- Root/nested-module fuzz gates: 66 discovered targets, 500 executions per target, two workers. This is a bounded integration smoke, not the default 10,000-execution campaign or an exhaustive fuzz proof. New admission targets also passed standalone 500-execution checks.
- Default build, baseline SSE2 self-check, Linux amd64/ARM64 cross-builds, reproducible release/layout checks and actual archive inspection. Neither archive contains Gemma.
- Nested uMCP `make check`, including its tests/race/vet/cross checks; nested source remains identical to main.
- Local production-container contract passed, including startup, image hardening/state persistence and live HTTP/MCP checks. This is a local disposable image test, not a NAS deployment. A transient readiness probe connection reset was retried by the contract harness; the contract completed successfully.
- Existing allocation performance budgets passed. Real GTE model, chunk tail/subprocess refresh and Needle model/tokenizer/generation/mapped-generation gates passed. No full Needle corpus rerun or browser/UI campaign was needed for this source-preservation change; those unrelated surfaces were not modified.

## Profile review and disposal

The integration ran the existing profile-capable coverage, model, performance and bounded fuzz helpers. Reviewed cumulative CPU, `alloc_space` and `alloc_objects`; 110 package profile rows across root/nested/focused runs have zero test/analysis error statuses. There are 32 empty-CPU process summaries among short focused/coverage/fuzz runs. These are explicitly incomplete CPU evidence for those short processes; they are not silently counted as sampled passes. Longer model/performance/service captures have real CPU samples. Focused default-off tests are too short for CPU attribution; their allocations are test/profile overhead, not a model benchmark.

Measured application patterns match the prior qualified service/model baselines: service coverage ~4,246 MB sampled allocation space, ~30 million objects, `buildModelsOffRuntime` ~44.8% cumulative allocation bytes, SQLite locking ~41.6% cumulative CPU; GTE loading ~127 MB decoded weight allocations; real chunk work ~95% cumulative CPU in embedding refresh; Needle real-model CPU ~61% flat attention, with tensor conversion/file loading as material allocation costs. These existing service/model paths were not changed or linked to Gemma. No new runtime performance improvement is claimed from identifier/lint/default-target changes. The measured parked-worker improvement and NAS SSE2 limitations remain in their separate comparison reports.

Fuzz allocation outliers (~238–261 MB per process summary in the largest cases) are predominantly Go fuzz worker coordination/client calls (~92–95% cumulative), with no new unbounded model workload. Performance allocation budgets pass; the optional model remains outside production dependencies. No further numerical/kernel tuning was introduced as part of this merge.

Once analysis and all owning jobs finished, raw profiles, matching binaries, generated releases/coverage, temporary fixtures, disposable logs and caches were removed. The local contract image was removed. Source, pinned durable model assets and concise reports remain. NAS production stays unchanged on 1.0.12/GTE.
