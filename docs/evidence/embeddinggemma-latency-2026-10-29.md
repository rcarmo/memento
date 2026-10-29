# EmbeddingGemma CPU latency reduction

Whole-row Q4_K and Q8_0 SIMD projection reduced complete text-embedding latency by 54–55% on this host, without changing the model, FP32 activation contract or reference tolerances. Production GTE, its image, worker and index were unchanged.

## Equivalent-workload comparison

Baseline: `f5d93d3`. Candidate: the same revision plus the Q4/Q8 changes. Linux amd64, Intel Core i7-12700, six visible logical CPUs, Go 1.27.1, CGO disabled, automatic AVX2 backend, `GOMAXPROCS=1`, one request at a time. The comparison uses the unchanged `tools/embedding-compare/corpus.json` and pinned UD-Q4_K_XL GGUF. Timing includes text tokenisation, recommended prompts and 768-dimensional output, with reused workspaces; model loading and index/IPC overhead are excluded.

First pass ran baseline then candidate with CPU/allocation captures. The independent unprofiled repeat ran candidate then baseline. Warm-up, input strings, token counts, sample counts and output checks were identical. Other host workloads were not suspended. These measurements apply to this host, not the NAS.

| Workload | Tokens | Baseline median | Candidate median | Reduction | Speed-up |
|---|---:|---:|---:|---:|---:|
| Query, 35 source characters | 15 | 536.90 ms | 247.27 ms | 53.9% | 2.17× |
| Paragraph, 406 characters | 69 | 2,588.33 ms | 1,152.72 ms | 55.5% | 2.25× |
| Longer chunk, 1,516 characters | 254 | 9,692.77 ms | 4,451.12 ms | 54.1% | 2.18× |

Unprofiled ranges were 535.36–546.30 / 241.71–256.47 ms for the query, 2,527.02–2,651.21 / 1,146.39–1,203.59 ms for the paragraph, and 9,584.05–9,868.89 / 4,433.65–4,485.85 ms for the longer chunk. There were seven, five and three timed samples respectively. The profiled pass measured 540.48→246.17 ms, 2,496.48→1,141.47 ms and 9,855.20→4,571.05 ms, consistent with the unprofiled repeat.

## Measured hotspots and changes

The fresh baseline CPU capture contained 58.31 s of samples. Q4 projection consumed 62.37% cumulative, split between the Go block/group dispatcher (27.34% flat) and assembly group kernels (28.38% flat). Row decoding consumed 19.52% flat. Calling a 32-value Q4 group kernel eight times per block repeatedly extracted scales, crossed the Go/assembly boundary and cleared AVX state.

The candidate:

- Calls one AVX2 Q4 kernel per complete row. It extracts packed scale/minimum values in assembly, processes low/high nibble planes together and accumulates across blocks before reducing. Exact float32 coefficient arithmetic is preserved; only reduction order changes.
- Uses whole-row SSE2/AVX2 Q8 kernels. Signed-byte decoding, scale multiplication and dot products stay in registers, avoiding a decoded scratch-row write/read.
- Uses a shared 65,536-entry binary16 lookup table (256 KiB) for scale conversion. It preserves all half bit patterns, including signed zeros, infinities and NaNs.
- Leaves scalar/NEON/non-amd64 fallback decoding intact; SSE2 Q4 still uses the existing group implementation. CPU capability checks continue through `simd.Engine`.

The candidate CPU capture contained 27.03 s of samples. Q4 row assembly became the largest remaining cost at 62.75% flat; Q8 assembly was 5.18% flat. Projection cumulative time was 82.54%. The remaining outlier is useful arithmetic inside the Q4 kernel; the measured redundant group-call and scratch-row work was removed. Further block/row scheduling, Q4 SSE2 fusion or multi-token matrix reuse are separate candidates, not assumed future gains.

An additional Q4 inner-loop unrolling experiment produced a 26.48 µs median versus 26.69 µs for a 512×512 projection, with overlapping 26.24–27.19 µs / 26.63–26.73 µs ranges. This ~0.8% result did not establish a stable gain, so the extra code was reverted. The retained row kernel reports zero projection allocations.

## Allocations and memory

Unprofiled repeat allocations were unchanged: query 21,464 bytes / 217 objects, paragraph 140,312 / 1,466, longer chunk 474,584 / 5,224. These text-to-vector figures include BPE tokenisation and the owned result. The kernels add no per-call allocation or full-weight expansion.

Post-GC loaded heap was 55,381,248 bytes before and 55,381,392 after; the 144-byte difference is runtime noise. The lookup table is static shared data outside that heap. Longer-workload resident memory changed from 164,880,384 to 165,195,776 bytes, an increase of 315,392 bytes (~0.3 MiB), consistent with touching the table and minor runtime variation. Peak RSS changed from 164,880,384 to 171,966,464 bytes (+6.8 MiB / 4.3%). That larger transient peak is recorded as a secondary resource regression; it is acceptable for the measured 2.2× latency improvement, with unchanged steady-state allocations and only ~0.3 MiB higher final RSS. Load time was unchanged at about 190 ms with warm filesystem data. Maximum-context/concurrent/NAS memory was not measured.

Both alloc_space and alloc_objects profiles were inspected. Sampled whole-process allocated space was 76.92 MiB before / 94.26 MiB after, with 389,665 / 920,542 objects, dominated by tokenizer/parser startup. The profile snapshots are influenced by GC publication boundaries, sampling and runtime map allocation; they are not equivalent complete allocation counters and do not establish an inference allocation regression. Direct per-workload TotalAlloc/Mallocs deltas remained equal. No load-time allocation saving is attributed to these kernel changes.

## Correctness and safety

All existing numerical gates stayed unchanged: independent-vector RMS `3e-7`, max absolute `1e-6`, cosine at least `1 - 1e-9`. All 11 real-checkpoint fixtures passed, including special tokens, multilingual text, Matryoshka prefixes and the 1,028-token local-attention boundary. The candidate long-window error was RMS `5.36e-8`, max `2.20e-7`. The reference fixture was not regenerated.

Verification passed:

- Independent multi-block Q4 and Q8 dequantisation/projection oracles, including high scale/min bits, signed Q8 bytes, zero/subnormal/exceptional scales and exhaustive half-table conversion checks.
- Linux guard pages for exact final Q4/Q8 weight and activation boundaries. No quantised-row overread is allowed.
- Busy-workspace, concurrent request, cancellation and Close lifecycle tests; focused race instrumentation.
- Full reference suite with AVX/FMA disabled, exercising SSE2 Q8 and existing SSE2 Q4, plus scalar consistency.
- Full real-checkpoint package/CLI suite under explicitly pinned Go 1.26.6; performance measurements used installed Go 1.27.1.
- `CGO_ENABLED=0 go test ./... -count=1`, package/CLI vet, five repeated synthetic package runs and diff checks.
- Linux ARM64 and RISC-V test cross-compilation; those CPUs were not executed.
- Read-only review of the new assembly found no substantive dispatch, bounds, packing or ABI issue. Its two test-coverage suggestions (full SSE2 Q4 rows and exceptional Q8 scales) were added and passed.

An initial Q8 assembly frame-size declaration failed vet and was corrected from 40 to the ABI-required 36 bytes before acceptance. Initial new test helper names were corrected before the final test runs. These failed probes are excluded from passing evidence.

The authored 16-query diagnostic retrieval set was rerun: 16/16 correct top results at 768, 512 and 256 dimensions; 15/16 at 128, unchanged from baseline. This preserves the small-set behaviour but does not establish quality on the production corpus. Gemma is still substantially slower than GTE-small; compare the fixed GTE results in [the host comparison](gte-vs-gemma-host-2026-10-28.md).

Raw captures, matching binaries, unrolling snapshots, disposable JSON and logs were removed after analysis. Source, pinned checkpoint, independent fixture and concise measurements remain. Reproduce using `tools/embedding-compare` with the same model/corpus and optional project-owned `-profiles` directory; profile disposal follows `AGENTS.md`.
