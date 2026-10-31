<!-- RUI-PROFILE-LIFECYCLE-20261005 -->
## Current profiling and cleanup rule — supersedes older text below

Rui's explicit rule: **profile and tune during pre-release tests; remove profiling data immediately after analysis/use.** Ordinary development tests do not require profiling on every run. Targeted diagnostic profiling is optional when useful and follows the same disposal rule.

- During pre-release verification, capture CPU and heap/allocation behaviour, analyse hotspots and tune avoidable allocations/repeated work. Compare equivalent workloads without weakening correctness/security/numerical contracts. For Go, inspect CPU, alloc_space and alloc_objects; coverage alone is not profiling.
- **Act on measured outliers before declaring profiling complete.** Rank material CPU, allocation, memory, latency and repeated-work outliers; investigate their cause and implement safe fixes, then remeasure equivalent workloads and verify correctness. Recording a hotspot and deleting captures is not completion. Judge optimizations by their measured overall benefit, not a zero-regression rule for every metric. Minor, quantified regressions in secondary performance/resource metrics are acceptable when justified by a larger verified improvement or a concrete incremental path to it. Compare equivalent workloads, account for measurement noise, document absolute and relative changes, downstream impact and the next validation step; do not assume speculative future gains. Preserve correctness, cancellation, security, isolation and numerical contracts. Revert changes with no demonstrated benefit or unacceptable net regressions; do not automatically reject a useful increment merely because another metric moves slightly backwards. Prioritise completing the release unless measurements show a major performance issue; do not expand optimisation indefinitely. Minor residual issues and justified secondary regressions may be documented as non-blocking follow-ups under Rui’s release direction. If a major outlier cannot safely be fixed within scope, record its measurements, cause or uncertainty, attempted fixes, concrete blocker and an owned follow-up; explicitly disclose it as unresolved and obtain Rui’s acceptance before treating pre-release profiling as complete. Do not silently defer it because an API contract or harness is involved; investigate contract-preserving options first. Dispose of captures only after the current analysis/comparison is finished; retain concise before/after conclusions, not raw archives.
- Keep captures and matching artifacts only while the current analysis needs them. Once used, immediately delete raw profiles, traces, matching test binaries, temporary fixtures and disposable logs, including failed/probe artifacts after diagnosis. Retain only concise conclusions and important measurements/limitations. Do not keep indefinite raw archives or copy them into exports/reports/evidence to evade cleanup.
- Remove all completed disposable cache/build/test/run data promptly. Never delete files still in use: finish or safely pause the owning job and clean at a safe boundary. Preserve source, installed toolchains, durable datasets/checkpoints and intentional release assets. Minimise disk usage; no random exports or redundant snapshots.
- This rule overrides **every conflicting older paragraph in this file and linked local guidance**, including “profile every test”, “unprofiled tests prohibited”, “retain all raw evidence”, “never delete profiles” and cleanup exclusions based only on an evidence/profiles directory name. Update helper/CI cleanup behaviour accordingly; do not weaken pre-release analysis.
- Existing no-agent-contact and execution-pause rules remain unchanged. This policy grants no unsolicited coordination or automatic job restart.
<!-- /RUI-PROFILE-LIFECYCLE-20261005 -->



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
- `docs/evidence/` -- concise acceptance conclusions; raw profiling artifacts are deleted after use.

Do not recreate a `go/` source subtree or add root-level `.go` files. Avoid `pkg/`: this repository does not promise public application-package APIs. Only the nested `umcp` module is independently reusable.

## Project cache and temporary paths

The canonical project is `memento-go`; Python's historical `memento` worktree has a separate root. Source `tools/project-env.sh` before direct build/test commands, or run `tools/project-env.sh <command>`. Make recipes use the same vendored helper. CI requires no `/workspace/Makefile`.

Resolution occurs before changing child temporary variables. Absolute `PROJECT_TMP_BASE` selects `<base>/memento-go`; absolute `PROJECT_TMP_ROOT` is a compatibility override whose final component must be `memento-go`. If both are supplied they must agree. Invalid, symlinked or unusable overrides fail. CI uses writable `RUNNER_TEMP`, then the original inherited `TMPDIR`, then `/tmp`, appending `memento-go` even if `/workspace/tmp` exists. Local execution prefers usable `/workspace/tmp`, then `/tmp`. `PROJECT_ORIGINAL_TMPDIR` snapshots the original value and the resolved `PROJECT_TMP_ROOT` propagates to children without recursive nesting.

- `cache/{go-build,go-mod,go-tools,xdg,bun,npm,pip,uv,python,playwright}` contains rebuildable caches. The helper sets the applicable Go, XDG, Bun, npm, Python and Playwright variables.
- `build/<worktree-path-checksum>/` contains disposable binaries, coverage and release archives. The stable checksum separates checkouts; it is not a generated-output correctness gate.
- `runs/<purpose>/run-<time>-<unique>/` owns isolated scratch; `TMPDIR`, `TMP`, `TEMP` and `GOTMPDIR` point beneath it. `tests/` and `logs/` are reserved for disposable test/log scratch.
- build/profiles, release-evidence, ui-audit and workspace notes must not accumulate raw captures or test binaries: remove them immediately after analysis. Preserve durable models, installed dependencies, concise conclusions and intentional release artifacts.


`make clean` and explicit cleanup must target only owned inactive disposable build/cache/run/profiling output; remove it promptly after use. Preserve source, durable model/data assets, concise conclusions, intentional releases and other checkouts. Do not remove active-job files or production state/configuration.

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

## Pre-release profiling and tuning

During pre-release tests, capture CPU and heap/allocation behaviour, analyse hotspots and tune avoidable work using equivalent workloads. Ordinary development tests do not require profiling. Delete raw captures, matching binaries, traces and disposable logs immediately after analysis/use; retain only concise conclusions and measured results. Missing or incomplete capture is a limitation, not a profiling pass. Preserve active-job files until a safe boundary, source, durable datasets/checkpoints and intentional release artifacts.

* During pre-release verification use CPU/heap-capable Make targets and analyse cumulative CPU/alloc_space/alloc_objects; coverage/benchmem alone are insufficient. Relevant nested modules and subprocess/fuzz workloads need explicit coverage or a stated limitation. Ordinary development tests may run without profiles.
* Capture per-package CPU/heap without collisions and record source/toolchain/command/workload/sampling details for the current comparison. Analyse and tune application hotspots against equivalent workloads without weakening correctness/security/cancellation/numerics or budgets.
* Immediately after analysis/use, delete raw profiles, matching binaries, copied sources and full logs, including failed captures after diagnosis. Keep concise conclusions and missing/empty-capture limitations only. No raw evidence archives or random exports.

## Engineering rules

- Follow YAGNI; prefer small typed boundaries and standard-library code.
- Read relevant files before editing and test after every behavioral change.
- Keep native commands under `cmd/<name>` and reusable application logic under `internal/`.
- Preserve deterministic JSON, ordering, timestamps and error boundaries required by parity fixtures.
- Do not weaken security, cancellation, resource, corruption or recovery behavior for speed.
- Never use `git rebase`; merge/pull with `--no-rebase`.
- Commit as `Rui Carmo <rui.carmo@gmail.com>` after configuring local and global identity.
