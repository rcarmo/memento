# Embedded Needle router performance

The embedded Needle runtime is a latency-oriented, single-request router. It is not a general completion model, and it does not replace Memento's deterministic policy or storage code.

## Measured Intel single-core result

The 2026-07-18 release build was pinned with `taskset -c 0` to one logical CPU of an Intel Core i7-12700. One process loaded the 52.8 MB NDL1 model once and processed the untouched 360-case family-separated corpus serially.

| Measure | Result |
|---|---:|
| Tool-decision parity | 360/360 |
| Warm p50 | 510.8 ms |
| Warm p95 | 554.6 ms |
| Warm maximum | 575.7 ms |
| Sustained serial throughput | 1.95 requests/s |
| Peak RSS | 163.4 MiB |
| Cold process + load + first request | 669 ms |

Per-request latency excludes initial model and tokenizer loading. Wall time, throughput and peak RSS include loading. CPU frequency and host contention were not fixed, so the values are observations from this host, not portable guarantees. The machine-readable record is [`evidence/needle/rust-router-single-core-i7-12700.json`](evidence/needle/rust-router-single-core-i7-12700.json).

## Planning projections

These ranges are capacity-planning estimates anchored to the measured i7-12700 result. They are not benchmark results. Actual latency depends on clock policy, cache, memory bandwidth, compiler target, thermal limits, prompt length and generated token count. Validate on the deployment hardware before setting an SLO.

| CPU class | Expected warm p50 | Expected serial throughput | Rationale |
|---|---:|---:|---|
| Recent Intel P-core (Alder/Raptor/Meteor class, AVX2/FMA) | 0.45-0.65 s | 1.5-2.2 req/s | Closest to measured host; clock and cache dominate variance. |
| Recent AMD Zen 3/4/5 core (AVX2/FMA) | 0.40-0.65 s | 1.5-2.5 req/s | Similar SIMD width; newer cores may improve scalar/front-end work and memory access. |
| Older Intel Haswell/Skylake core (AVX2/FMA) | 0.65-1.00 s | 1.0-1.5 req/s | Lower IPC/clock and smaller effective cache than the measured P-core. |
| ARM server core (Neoverse N1/V1/N2 class, NEON) | 0.60-1.00 s | 1.0-1.7 req/s | Native NEON path exists, but narrower vectors and platform clocks vary widely. |
| Apple M-series performance core under Linux/macOS-equivalent native build (NEON) | 0.40-0.70 s | 1.4-2.5 req/s | Strong single-core and memory subsystem; no repository hardware measurement yet. |
| Modern ARM SBC performance core (Cortex-A76/A78 class, NEON) | 1.0-1.8 s | 0.55-1.0 req/s | Lower sustained clock and bandwidth; thermal throttling can widen the range. |
| Low-power x86 core (Atom-class or older mobile AVX2) | 1.2-2.5 s | 0.4-0.8 req/s | Lower single-thread IPC and sustained clock despite AVX2 availability. |

The router currently serialises each model instance for predictable scratch-buffer and cancellation behaviour. Horizontal process replicas can raise throughput if memory permits; do not infer linear scaling from the single-core measurement because shared last-level cache and memory bandwidth become limiting factors.

## Reproduction

After fetching Git LFS artefacts:

```bash
cd rust
cargo build --release -p memento-needle --bin needle-eval
cd ..
taskset -c 0 rust/target/release/needle-eval \
  models/needle/memento-router.ndl \
  models/needle/needle.model \
  < models/needle/test.jsonl > /tmp/needle-results.jsonl
```

Capture process RSS and wall time externally, and compare the emitted tool name in every result with the corresponding `answers` field. The committed evidence used the SHA-256 `9ffeb303574fa6bd24718adc42c7a3d8c4632e3cf78685d14886c7b24b2ddca9` corpus.
