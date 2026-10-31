# Single-core Gemma blocked packing and fast GELU

Contiguous eight-token packing blocks and a bounded GELU approximation reduced single-core text-to-vector latency by about 6–8% on the fixed host workloads. The checkpoint and full-model reference tolerances remain unchanged. CPU affinity is one logical core, `GOMAXPROCS=1`, and `Workers=1`; parallelism does not contribute to these results. Production was untouched.

## Equivalent-workload results

Baseline `ecad51f`; Linux amd64, Intel Core i7-12700, Go 1.27.1, CGO disabled, automatic AVX2/FMA, affinity CPU 0. Same GGUF, corpus, prompts, token counts, warm-up and seven/five/three timed samples. Timing includes tokenisation, recommended prefixes and normalised 768-dimensional output; load/index/IPC overhead is excluded. Model digest reading warms filesystem pages. Other host workloads were not suspended.

Reverse-order repeat, candidate then baseline:

| Workload | Tokens | Baseline median | Candidate median | Reduction |
|---|---:|---:|---:|---:|
| Query | 15 | 62.949 ms | 58.846 ms | 6.5% |
| Paragraph | 69 | 279.412 ms | 259.484 ms | 7.1% |
| Longer chunk | 254 | 1,044.089 ms | 967.789 ms | 7.3% |

A prior baseline-then-candidate pair measured 64.697→61.855 ms, 290.374→260.475 ms and 1,049.439→975.408 ms, reductions of 4.4%, 10.3% and 7.1%. Final checked profiled medians were 58.717/258.194/966.960 ms. Host variation is quantified rather than attributed to an optimisation. The gain is modest but consistent across all three inputs and both orders.

The final process used 6.33 s wall, 6.26 s user and 0.07 s system time, about one CPU-second per wall second. This is genuinely single-core. The earlier four-worker ≤2× GTE result is a separate CPU-budget experiment.

## Measured hotspots and changes

Fresh baseline CPU profile: 6.43 s samples; matrix scatter assembly 54.59% flat and GELU 10.42% cumulative. Larger token tiles made the former column-major activation stride skip between token groups during each matrix inner loop.

The new layout packs each eight-token group into a contiguous `columns*8` FP32 block. Indexing is `(token/8)*columns*8 + column*8 + token%8`. Matrix kernels advance 32 bytes per column, reducing stride/cache work; output order, FP32 FMA sequence and valid-token suppression are unchanged. Packing and consuming kernels are switched together for both serial and opt-in parallel projections. Unsupported CPUs retain the previous non-packed fallback. No weight matrix is expanded and no scratch capacity is added.

`fastGELU` replaces repeated `math.Tanh` calls with cubic Hermite interpolation on a shared immutable ~73 KiB table over the transformed positive tanh argument. The transform and final float32 operation sequence remain the same. Finite saturated tails, ±Inf, NaN classification and signed zero match the original behaviour. This is a numerical approximation, not a universal byte/ULP equality guarantee.

A 2,272,385-point dense/random/knot-boundary sweep measured max absolute GELU error `4.47e-8`, RMS `3.56e-11`, and maximum six output ULPs near `x=-1.53015995`, where negative-tail cancellation magnifies ULP counts. Standalone regression gates explicitly reject nonfinite surprises and absolute error above `1e-7`, with RMS at most `1e-8`; ULP distance is reported rather than substituted for the absolute bound. The full-model independent RMS/max/cosine gates were not widened.

Three zero-allocation GELU benchmark samples measured reference 10.44–10.62 ns/op versus candidate 4.748–4.774 ns/op (~2.2× kernel speed-up). Ablation with identical blocked matrix code measured direct GELU→fast GELU at 66.027→60.982 ms, 289.345→274.692 ms and 1,118.670→1,056.680 ms. This isolates a 5–8% request benefit in that pair; it is not added to the overall comparison as an independent multiplicative claim.

Simple matrix-loop unrolling did not demonstrate an acceptable overall gain and was reverted. Early GELU-only probes were also inconclusive; the retained implementation is justified by ablation and equivalent full-request comparisons. No rejected unrolling variant remains active.

Final CPU profile: 5.92 s samples; blocked scatter arithmetic 61.99% flat, `fastGELU` 6.08% cumulative. Matrix computation remains the dominant concrete follow-up. No speculative future gain is included.

## Resource measurements

Per-request allocations stayed equal: query 21,464 bytes / 217 objects; paragraph 140,312 / 1,466; longer chunk 474,584 / 5,224. The 73 KiB table is static shared read-only-by-convention data, outside Go heap allocation counters. No additional request scratch or worker is introduced.

Unprofiled paired workload heap differed by −304 bytes and post-GC model heap was effectively equal. Longer-workload RSS changed 166,686,720→167,059,456 bytes (+0.36 MiB); peak RSS was 171,966,464→167,059,456 bytes in that pair. The final profiled process peaked at 172,228,608 bytes / 168,192 KiB (~164.25 MiB), about 0.25 MiB above the unprofiled baseline peak. Runtime/page/profiling effects exceed the table's fixed size, so no retained-heap/RSS saving is claimed. Maximum-context/concurrent/NAS resources are not qualified.

CPU, alloc_space and alloc_objects captures were analysed. Sampled process totals were 83.44→85.38 MiB and 777,325→581,597 objects, dominated by tokenizer/parser startup and GC/sampling publication boundaries. These totals are not equivalent allocation histories or a savings claim; direct per-call counters are unchanged.

## Correctness and verification

All 11 independent model reference fixtures pass without regeneration or threshold changes: RMS `3e-7`, max absolute `1e-6`, cosine at least `1 - 1e-9`. Largest displayed reference error was `2.43e-7`; the 1,028-token local-window case remains within the original limits. Generated-vector hashes do not gate acceptance.

Passing checks:

- Independent blocked-layout matrix/packing references and guard pages covering multiple token blocks, tails and exact final input/output extents. New wrappers validate dimensions/slice lengths before unsafe assembly calls.
- Odd-column decoded panels, actual valid-token lengths 1–7 including whole suppressed blocks, malformed dimensions and short buffers. Token-major output gaps remain untouched.
- GELU sweep, exceptional values and 20,000 `FuzzFastGELU` iterations with the stricter absolute gate; zero steady-state kernel/projection allocations.
- Original independent full-model fixtures, serial and opt-in parallel reference tests, focused race/cancellation/Close tests, AVX/FMA-disabled reference suite, explicit Go 1.26.6 real-checkpoint package/CLI/comparison tests, Linux ARM64/RISC-V cross-compilation, vet and `CGO_ENABLED=0 go test ./... -count=1`. Non-amd64 binaries were not executed.
- Read-only review found no production blocker; its suggestions for stricter GELU regression predicates and new kernel-boundary coverage were implemented and passed. The initial GELU delegate timed out after writing files; the main task reviewed and independently ran all acceptance checks.

The fixed diagnostic retrieval scores are unchanged: 16/16 top results at 768/512/256 dimensions, 15/16 at 128. This authored small set does not qualify production-corpus quality. Serial Gemma remains slower than GTE-small on short inputs. Production and parallel defaults are unchanged.

Raw profiles, matching binaries, ablation/source probes and disposable JSON/logs were removed after analysis. Source, pinned models, independent fixture and these conclusions remain.
