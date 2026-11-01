# Single-core Gemma six-row matrix kernel

A six-row, sixteen-token AVX2/FMA kernel reduced complete single-core Gemma latency by about 8–13% in the final equivalent-workload comparison. The checkpoint, FP32 activation representation and independent reference tolerances are unchanged. Production and the serial worker default were untouched.

## Host comparison

Baseline `fb5b3ff`; Linux amd64, Intel Core i7-12700, Go 1.27.1, CGO disabled, automatic AVX2/FMA. Both executables were pinned to logical CPU 0 with `GOMAXPROCS=1` and `Workers=1`. The fixed model, corpus, source content, prompts, token counts, warm-up and seven/five/three timed samples are identical. Timings include tokenisation and normalised 768-dimensional output, excluding model load, index search and worker IPC. Filesystem pages were warm; other host workloads were not suspended.

Final reverse-order comparison, candidate then baseline:

| Workload | Tokens | Baseline median | Candidate median | Reduction |
|---|---:|---:|---:|---:|
| Query | 15 | 58.883 ms | 52.946 ms | 10.1% |
| Paragraph | 69 | 257.341 ms | 236.824 ms | 8.0% |
| Longer chunk | 254 | 1,004.252 ms | 870.035 ms | 13.4% |

Ranges were 57.75–59.01 / 52.66–54.69 ms, 253.91–259.76 / 233.98–240.78 ms, and 963.13–1,006.87 / 858.40–874.55 ms. A later checked run measured 54.265/235.301/857.828 ms; its query had one 76.93 ms outlier. Process time was 5.77 s wall, 5.71 s user, 0.05 s system, consistent with one CPU. No parallel latency is used as single-core evidence.

The first fresh profiled baseline measured 65.80/293.99/1,084.56 ms, showing host timing drift during the experiment. Reported gains use the later adjacent pair rather than selecting that slower baseline. NAS, concurrent requests, cold disk and maximum-context latency were not measured.

## Measured cause and implementation

Fresh baseline CPU capture: 6.59 s samples; blocked matrix assembly 59.18% flat. The prior eight-row/eight-token layout broadcast each weight for only eight token lanes and required an 8×8 output transpose.

The new layout packs sixteen tokens contiguously per input column. A microkernel holds six weight rows × sixteen tokens in twelve AVX accumulators, reusing each broadcast across two vectors. A completed panel spills twelve vectors into a bounded 512-byte assembly frame, transposes six outputs per token, and writes exactly six floats using a 16-byte plus 8-byte store. Actual-token checks suppress padded stores, including entire later token blocks. The buffered partial-row path uses the same arithmetic and copies only actual rows.

Activations remain FP32; the weight panel is decoded only into existing bounded request scratch. Numerical reduction order per output remains column-ordered FMA. No model matrix is expanded. CPU capability selection still requires AVX2 and FMA; scalar/SSE2/non-amd64 paths retain their existing implementation. Serial and parallel projections switch packing and consuming kernels together. Worker row partitions can begin/end at multiples of eight; six-row loops explicitly stop at each assigned `end` and zero partial-panel tails.

Checked Go wrappers bound columns, token padding, row stride and all slice lengths before assembly calls. Packed request storage is unchanged: at most 64×2,048 floats. Existing eight-row decoded/sums scratch is sufficient for six-row panels. No new heap/cache/table or background worker is introduced.

Exploratory four-row/sixteen-token kernels showed only modest gains. Two-/four-way inner-loop unrolling did not provide a stable worthwhile increment; the retained six-row implementation uses a simple column loop. Rebalancing a tiny final token tile increased paragraph latency to 322 ms and was reverted. No rejected scheduling/unrolling variant remains active.

Candidate CPU capture: 5.56 s samples; new scatter kernel 51.08% flat, fast GELU 8.99% cumulative, contiguous attention 16.01% cumulative. Matrix work remains the largest concrete target. No speculative next optimisation is included in the measured benefit.

## Allocations and memory

Per-call allocations are unchanged: query 21,464 bytes / 217 objects, paragraph 140,312 / 1,466, longer chunk 474,584 / 5,224. Projection tests still require zero steady-state allocations. Scratch capacity and static GELU/half tables are unchanged; the kernel uses a 512-byte stack frame.

Reverse-order repeat peak RSS was identical at 172,228,608 bytes (~164.25 MiB). Loaded post-GC heap changed 55,427,744→55,428,080 bytes (+336 bytes). Workload heap differed by at most hundreds of bytes; final process peak was 167,936 KiB (~164 MiB). These runtime differences do not establish a memory saving. Maximum-context/concurrent/NAS resource use is not qualified.

CPU, alloc_space and alloc_objects captures were inspected. Sampled process totals were 72.02→93.51 MiB and 481,975→903,260 objects, dominated by tokenizer/parser startup and affected by sampling/GC publication boundaries. These totals do not establish a new per-call allocation regression; the equivalent direct allocation counters and capacity measurements are unchanged. No allocation-saving claim is made.

## Correctness and safety

All independent reference gates remain RMS `3e-7`, max absolute `1e-6`, cosine at least `1 - 1e-9`; fixtures were not regenerated. All 11 real-checkpoint cases pass, including the 1,028-token local attention window and truncated dimensions. Generated-vector hashes have no acceptance role.

Passing verification:

- Independent numerical, packing and scatter checks with exact final load/store guard pages, production widths and token counts around 8/16/32/64 boundaries. Output row gaps stay untouched.
- Guard-backed odd widths 1/7 and actual tokens 1–7 with full suppressed later blocks, compared against FP64 oracles. Malformed dimensions and short buffers fail before assembly.
- Synthetic interior worker spans of 8/16 rows that are not divisible by six, with sentinels outside the assigned span; final partial rows and zero padding covered.
- Focused race tests including parallel requests, cancellation/Close and all new kernels; explicit Go 1.26.6 real-checkpoint package/CLI/comparison suite; AVX/FMA-disabled reference/fallback suite; Linux ARM64/RISC-V test cross-compilation; vet; `CGO_ENABLED=0 go test ./... -count=1`. Non-amd64 binaries were not executed.
- Read-only review found no ABI/register/bounds/parallel-partition blocker. Suggested small-valid/odd-width guarded oracles and non-six-aligned worker spans were added and passed.

The fixed retrieval sanity set remains 16/16 correct top results at 768/512/256 dimensions and 15/16 at 128. This small authored corpus does not qualify production-corpus quality. An initial combined verification command timed out during fallback testing; it is not counted as passing, and the fallback suite was rerun separately to completion.

Raw before/intermediate/after profiles, matching binaries, generated source probes and disposable logs/JSON were removed after analysis. Source, pinned checkpoint, independent fixture and concise measurements remain.
