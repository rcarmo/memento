# Deterministic semantic-worker scheduling tests

CI [43885982511](https://github.com/rcarmo/memento/actions/runs/43885982511) failed `TestSemanticWorkerSelectedAndFull` with four completed refreshes where the test expected three. The production worker was behaving correctly: a second request arriving after its path enters a refresh must survive that in-flight refresh. The test assumed the second enqueue happened while the path was still queued.

## Explicit interleavings

`internal/derived/semantic_worker_test.go` now uses cancellation-aware channel barriers to exercise both schedules:

- Block `/b`, enqueue the already queued `/a`, release: calls `/b,/a,/c`, completed count 3.
- Block `/a` in flight, enqueue `/a` again, release: calls `/b,/a,/a,/c`, completed count 4.

Both cases check exact call order, idle state, idempotent close and enqueue rejection after close. Cleanup cancels a blocked fake on assertion failure. No sleeps or production-code changes were introduced. The existing one-second idle wait, production timeouts and numerical/resource gates are unchanged.

The original test passed 3,000 local race repetitions without spontaneously reproducing the CI failure. The controlled cases demonstrate why both completion counts are valid under different schedules. The fixed test passed 3,000 top-level race repetitions (6,000 subcases) with `-cpu=1,2,4`; worker/error tests also passed 100 shuffled race repetitions. Those runs took 8.915 and 3.980 seconds respectively. They are correctness stress results, not equivalent-workload performance comparisons.

The authorised retry of the original commit completed successfully as attempt 2. That success does not remove the scheduling-dependent test assumption; the explicit regression cases are still needed.

## Local qualification

Go 1.26.11, GOMAXPROCS 4, bounded local service with four-CPU quota and 8 GiB memory limit. The final qualification ran from 20:36:01 to 20:57:12 UTC on 16 November, with the source test patch recorded beside the profile metadata.

Passed `make audit`, `performance`, `model-test`, `corpus-test`, `release-check` and `go-container-contract`. This includes root/nested-module formatting, layout, vet, lint, coverage, race, fuzz, vulnerability, cross-build, numerical/model and allocation-budget checks. Existing Rust output/error corpus parity remained 360/360; this is parity, not a new argument-accuracy claim. Both static release archives passed their checks. The local container contract passed with 22.7 MiB idle container charge and 36.8 MiB daemon RSS. Its initial connection reset was handled by the existing bounded startup wait; no NAS operation was involved.

An initial qualification launcher lacked Go/gofmt in systemd's PATH and stopped before testing. Its cause was diagnosed, the launcher PATH corrected, and its empty/failed artifacts disposed before the final run. No failed test was blindly retried.

## Profile review and disposal

Inspected cumulative CPU, `alloc_space` and `alloc_objects` for 222 package summaries and 379 processes: 242 ordinary processes, 135 fuzz workers and two model workers. All 379 CPU/allocation-space/allocation-object triplets were present and analysed, with zero test/analysis failures or missing-profile reports. There were 81 empty CPU samples for short processes; those paths have incomplete CPU sampling. No allocation profile was empty.

Largest CPU workloads were the full Needle corpus (344.66 seconds, 83.36% cumulative in encoding), service race tests (97.06 seconds, 38.10% cumulative in SQLite locking), and GTE fuzz work (60.19 seconds). Largest service allocation history was about 5,383 MiB and 34,977,168 objects, dominated by repeated fixture/runtime construction and JSON decoding: models-off runtime construction accounted for 38.08% of bytes, JSON decoding for 40.75% of bytes and unmarshalling for 73.02% of objects. These are cumulative test costs, not resident service memory.

The hotspot pattern matches the previously reviewed fixture/model/SQLite costs in [the release reconciliation](release-1.1.1.md). No production path changed, all existing performance/allocation gates passed, and no new material runtime outlier was identified. No runtime speed-up or optimisation gain is inferred from differing workloads or sample totals. The fix preserves the newer-request generation semantics instead of changing production to satisfy a racy assertion.

After analysis and confirming no owner remained, raw profiles, matching binaries, temporary fixtures and run logs were removed. Only concise conclusions and source are retained; installed toolchains, pinned models, durable datasets and intentional rollback assets are preserved.

## Delivery boundary

This is a test-only correction. It does not change the published 1.1.1 release, authorise another NAS upgrade attempt, implement worker reuse or revive Needle 3. Production remains on its verified 1.1.0 rollback baseline; the NAS orchestration/start-delay hold is separate.
