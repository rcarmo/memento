# Memento pure-Go replacement

## Product boundary

This branch ships Memento as a pure-Go service and container. Runtime artifacts must not depend on Python, Rust, CGo, native SQLite extensions, an external Git executable, or a shell.

Preserve the accepted client and persisted-state contracts:

- MCP Streamable HTTP, graph/admin and staging routes;
- bearer authentication, authorization and response/error schemas;
- `/etc/memento/config.json`, `/var/lib/memento`, `/models`, port 8000 and `/mcp`;
- Git repository, control SQLite and derived SQLite formats, including rollback readability by the previous image.

The NAS target is CPU-only. Do not package or enable Vulkan for NAS without an explicit new decision.

## Layout

- `go.mod`, `go.sum` -- primary Memento module at repository root.
- `cmd/<name>/` -- process entrypoints only.
- `internal/` -- all Memento-specific application, storage, protocol-adapter and model packages.
- `umcp/` -- reusable standalone nested module; it must not import Memento packages.
- `testdata/parity/` -- immutable compatibility fixtures shared by internal package tests.
- `deploy/` -- container deployment examples.
- `models/runtime-models.json` and `tools/prepare_runtime_models.py` -- CI-only model bundle retrieval and digest verification. Python is allowed here only as build tooling and is never shipped.
- `docs/evidence/` -- retained immutable acceptance evidence.

Do not recreate a `go/` source subtree or add root-level `.go` files. Avoid `pkg/`: this repository does not promise public application-package APIs. Only the nested `umcp` module is independently reusable.

## Project cache and temporary paths

The canonical project is `memento-go`; Python's historical `memento` worktree has a separate root. Source `tools/project-env.sh` before direct build/test commands, or run `tools/project-env.sh <command>`. Make recipes use the same vendored helper. CI requires no `/workspace/Makefile`.

Resolution occurs before changing child temporary variables. Absolute `PROJECT_TMP_BASE` selects `<base>/memento-go`; absolute `PROJECT_TMP_ROOT` is a compatibility override whose final component must be `memento-go`. If both are supplied they must agree. Invalid, symlinked or unusable overrides fail. CI uses writable `RUNNER_TEMP`, then the original inherited `TMPDIR`, then `/tmp`, appending `memento-go` even if `/workspace/tmp` exists. Local execution prefers usable `/workspace/tmp`, then `/tmp`. `PROJECT_ORIGINAL_TMPDIR` snapshots the original value and the resolved `PROJECT_TMP_ROOT` propagates to children without recursive nesting.

- `cache/{go-build,go-mod,go-tools,xdg,bun,npm,pip,uv,python,playwright}` contains rebuildable caches. The helper sets the applicable Go, XDG, Bun, npm, Python and Playwright variables.
- `build/<worktree-path-checksum>/` contains disposable binaries, coverage and release archives. The stable checksum separates checkouts; it is not a generated-output correctness gate.
- `runs/<purpose>/run-<time>-<unique>/` owns isolated scratch; `TMPDIR`, `TMP`, `TEMP` and `GOTMPDIR` point beneath it. `tests/` and `logs/` are reserved for disposable test/log scratch.
- `build/profiles/`, `build/release-*-evidence/`, `build/ui-audit/`, `docs/evidence/` and workspace notes retain profiles, matching binaries, logs and publication evidence separately. Do not delete or relocate existing evidence as part of cache cleanup. Model assets and installed dependencies are durable inputs.

`make clean` removes only this checkout's disposable `BUILD_ROOT`, never the project root, another checkout, caches or retained evidence. Do not clean while a job uses it. No automatic migration or deletion of historical paths is authorised. Runtime container `/tmp` mounts and production state/configuration are unchanged.

## Required gates

Use root Make targets rather than ad-hoc commands:

```sh
make quality
make audit
make performance
make model-test
make corpus-test
make release-check MEMENTO_VERSION=1.0.0 SOURCE_DATE_EPOCH=0
make go-container-contract MEMENTO_VERSION=1.0.0
```

Every production statement must remain covered, and every package containing production Go code must retain at least one meaningful fuzz target. Run formatting, vet, pinned Staticcheck, govulncheck, race, fuzz, cross-build and release checks before pushing.

## Mandatory Go test and pre-release profiling

Every Go test run must capture CPU and heap/allocation profiles and receive post-run analysis. This applies to ordinary feature and bug-fix work, focused/full tests, coverage, race, benchmarks, fuzzing, model/corpus runs, nested `umcp` tests, delegated work and failed runs. Overall pre-release checks must include the same profiling and analysis.

* Use profiling-aware Make targets. Update existing unprofiled targets/scripts before using them; direct `go test` is allowed only with CPU/memory capture and subsequent analysis. `-benchmem` and coverage alone are insufficient.
* Capture per-package profiles without filename collisions, normally with `-cpuprofile` and `-memprofile`, and retain matching test binaries and logs under ignored `build/profiles/<run>/<package>/`. Record the revision, toolchain, command, workload and sampling settings. Ensure caching does not substitute an old test result for a profiled execution. Multi-process tests and fuzz workers need explicit coverage of the relevant child workload; coordinator-only profiles do not describe child allocations.
* Inspect cumulative CPU time, `alloc_space` and `alloc_objects` after every run. Separate application hotspots from test setup/runtime overhead, identify avoidable allocations or repeated work, and compare equivalent workloads before accepting an optimisation. Record latency/throughput, bytes/op and allocs/op where benchmarks apply. Profile generation alone does not complete the analysis.
* If a build failure, timeout or abrupt exit prevents capture, retain the logs and report the missing profiles. Empty CPU samples also need an explicit note and a representative profiled workload before drawing performance conclusions. Failed or missing capture cannot pass the profiling gate.
* Include profiling results and engineering analysis in `quality`, `audit`, performance and overall pre-release validation. Record baseline comparisons, improvements and unresolved hotspots. Minimise allocations and improve measured performance without weakening correctness, authorisation, cancellation, corruption/recovery behaviour or numerical contracts.
* Update `tools/performance-budgets.json` only with measured evidence and a documented reason. Do not widen a ceiling to hide a regression or add flaky wall-clock gates across heterogeneous runners.

## Engineering rules

- Follow YAGNI; prefer small typed boundaries and standard-library code.
- Read relevant files before editing and test after every behavioral change.
- Keep native commands under `cmd/<name>` and reusable application logic under `internal/`.
- Preserve deterministic JSON, ordering, timestamps and error boundaries required by parity fixtures.
- Do not weaken security, cancellation, resource, corruption or recovery behavior for speed.
- Never use `git rebase`; merge/pull with `--no-rebase`.
- Commit as `Rui Carmo <rui.carmo@gmail.com>` after configuring local and global identity.
