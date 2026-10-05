# Project-owned build and temporary paths

The post-release tooling change routes new disposable output through `memento-go`'s project root. It does not change the published 1.0.11 tag or production image.

The vendored resolver implements `PROJECT_TMP_BASE`, compatible `PROJECT_TMP_ROOT`, agreement checks, CI-first `RUNNER_TEMP`/original `TMPDIR`/system fallback and local workspace/system fallback. It rejects invalid overrides and symlinked roots. Make and direct shell/Node helpers share that resolver. Physical run paths preserve repository test symlink checks even when `/workspace` is a host alias. Root and nested uMCP builds use the same physical-checkout component. Existing source, models and retained evidence were not moved or deleted.

## Verification

Retained logs are under `build/release-1.0.11-evidence/path-policy*.log`; matching Go profiles, binaries and source snapshots are under `build/profiles/path-policy*`. Node profiles and matching JS sources are under `build/profiles/node/`.

- Thirteen resolver cases passed: explicit base/root/agreement, conflicting/relative/empty/wrong-name rejection, CI runner and inherited temp precedence, original-TMPDIR snapshot, local generic fallback, and symlink rejection.
- The tools package passed, including static archive reproducibility and release-layout checks; real amd64/arm64 command builds and nested uMCP cross-compilation passed.
- Focused control/query, service options/cursor and uMCP lifecycle checks passed with profiles. The model-worker build and all `TestRealChunk` cases passed with separate worker profiles under the new build root.
- Node graph tests (2) and diagnostics tests (5) passed with CPU/allocation profiles. The lifecycle-only interruption check passed using the profiled Go fixture and verified graceful shutdown, retained failure evidence and closed socket.
- Shell syntax, workflow YAML parsing, formatting, focused root vet and nested uMCP vet passed. CI uses the vendored bootstrap before Go setup and exports the resolved environment; container-builder caches/output are project-owned. The deployed runtime mount contract is unchanged.

## Profiles and failed attempts

The initial focused control check used the installed Go1.26.3 because a direct wrapper invocation did not yet set `GOTOOLCHAIN`. The helper now defaults to Go1.26.6. That short run's CPU samples were empty and allocations were profiling/init overhead. The later representative Go1.26.6 proposal query benchmark measured 265,607 ns/op, 22,481 B/op and 641 allocations/op. This matches the release allocation baseline of 22,480–22,481 B/op and 641 allocations; host timing variation prevents a latency conclusion. SQLite execution/text-row conversion dominates application cost. The profile has 1.13 seconds of CPU samples, 97.23 MiB allocated and 2.24 million allocation objects.

The corrected focused service run allocated 8.23 MiB and 139,485 objects; SQL row cleanup and cursor-scope construction dominate sampled application objects. The short uMCP run has an empty CPU sample and 3.79 MiB allocated, dominated by init and profiling. These are correctness checks, not CPU-performance measurements.

The full tools run allocated 131.66 MiB, with file reads contributing 98.51 MiB cumulatively; JSON decoding accounts for 227,376 of 458,986 allocation objects. Archive comparisons and immutable fixture manifest checks explain those allocations. A final resolver/release-script run allocated 54.28 MiB, of which file reads were 49.39 MiB. Its CPU captures only 30ms because compilation/build children spend most time outside the test process; no build-performance claim follows. Child compiler invocations are builds, not unprofiled test execution.

One service filter matched no tests and correctly failed verification. The first tools run failed because the existing test invoked a newly Bash-based release helper with `sh`; the test now invokes Bash and permits release output only beneath disposable build or isolated test-run roots. Passing reruns are retained separately.

The first browser lifecycle attempt failed on the `/workspace` symlink ancestor; physical scratch paths fixed this without relaxing repository checks. The next attempt hit a local WebGL graph timeout before it could send its lifecycle interrupt. The test watchdog killed the browser runner; the isolated Go fixture was then stopped through its owned stop-file protocol and flushed profiles. Its interrupted wrapper left an incomplete status table; CPU, `alloc_space` and `alloc_objects` were analysed manually. It allocated 106.07 MiB/760,787 objects; runtime construction accounted for 50.34 MiB and HTTP handling for 371,470 cumulative objects, with SQLite 42.86% cumulative CPU. No complete browser audit is claimed from that attempt.

The interruption test now starts a lifecycle-only fixture path and interrupts on explicit readiness. Full visual audits still run through the ordinary UI target; they have not been rerun locally after this tooling change. The final lifecycle fixture has 270ms CPU, 62.57 MiB allocated and 349,529 objects, dominated by model-off test runtime and graph-cache construction. Node profiles are dominated by idle time, module loading, test-runner work and garbage collection. Browser engine allocations are not captured by Node profiles. These limits are retained rather than reclassifying failed or short runs as performance evidence.

The relocated real-model worker capture has 1.63 seconds of CPU samples, 95.09% cumulative in embedding, and 271.07 MiB allocated, 95.57% cumulative in cold model loading. Its 28,552 allocation objects include 10,923 filesystem-stat objects. This is consistent with the release's cold-load-dominated worker baseline; no warm-worker improvement is claimed.

No performance budgets, numerical tolerances, historical oracle fixtures or test filesystem safety checks were relaxed. Retained evidence is excluded from `make clean`; cleanup removes only the current physical checkout's disposable build directory after validation.
