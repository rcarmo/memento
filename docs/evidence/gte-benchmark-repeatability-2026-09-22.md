# GTE cold and steady-state allocation measurements

The ARM64 CI allocation failure on the metrics-only patch came from charging first-call workspace allocation to a host-dependent benchmark iteration count. The runtime and model output were unchanged.

[CI run 35755959417](https://github.com/rcarmo/memento/actions/runs/35755959417) measured GTE at 2438, 2438 and 2400 B/op against the existing 2400 B/op budget. Model correctness passed. On the local amd64 host, fixed-count runs of the same code measured 15028 B/op at 10 iterations and 5030 B/op at 30 iterations: a roughly 150 KB workspace allocation was being amortised by `b.N`.

`BenchmarkRealGTEEmbed` now warms the reusable workspace before resetting counters. A separate `BenchmarkRealGTEColdWorkspace` empties the model's workspaces between calls without charging model loading to inference. Both run in the performance gate.

| Benchmark | 10 calls | 30 calls |
| --- | --- | --- |
| Steady-state | 33 B/op, 2 allocs/op | 32 B/op, 2 allocs/op |
| Cold workspace | 150010 B/op, 17 allocs/op | 149997 B/op, 17 allocs/op |

Three two-second steady-state samples measured 32 B/op and 2 allocs/op. The existing steady-state ceiling remains 2400 B/op and 2 allocs/op. The new independent cold ceiling is 155000 B/op and 18 allocs/op, allowing about 3% overhead above the measured cold allocation. No runtime code or model thresholds changed.

Raw evidence: `/workspace/tmp/memento-rollout-108/gte-cold-amortization.log`, `gte-warm-cold-fixed.log`, and `performance-fixed.log`.
