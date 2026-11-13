# EmbeddingGemma SIMD decoding and transposes

SIMD Q5_K/Q8_0 decoding, vectorised activation packing and direct token-major output stores reduced Gemma text-to-vector latency by another 22–37% on this host. The original checkpoint, FP32 activation representation and independent reference tolerances are unchanged. Production GTE, its worker, image and index were untouched.

## Equivalent workloads

Baseline `473f07c`; Linux amd64, Intel Core i7-12700, six visible logical CPUs, Go 1.27.1, CGO disabled, automatic AVX2/FMA, `GOMAXPROCS=1`, sequential calls. Model and `tools/embedding-compare/corpus.json` unchanged. Timings include tokenisation, recommended prompts and normalised 768-dimensional output, with reusable request scratch; load/index/IPC overhead is excluded. Filesystem pages were warm and the host was not exclusively reserved.

Unprofiled reverse-order repeat, candidate then baseline, with identical warm-up and seven/five/three samples:

| Workload | Tokens | Baseline median | Candidate median | Reduction | Speed-up |
|---|---:|---:|---:|---:|---:|
| Query | 15 | 107.79 ms | 68.06 ms | 36.9% | 1.58× |
| Paragraph | 69 | 407.59 ms | 280.57 ms | 31.2% | 1.45× |
| Longer chunk | 254 | 1,538.52 ms | 1,196.28 ms | 22.2% | 1.29× |

Ranges were 106.73–108.46 / 66.87–68.19 ms, 401.88–413.66 / 279.59–289.61 ms and 1,531.96–1,540.63 / 1,194.70–1,205.12 ms. Final checked profiled medians were 67.73/278.72/1,209.69 ms, consistent with the repeat.

The first fresh baseline measured 125.92/470.36/1,841.46 ms. One early longer-input candidate probe regressed to 2.72 s; subsequent profiled and unprofiled runs recovered. Host timing changed during the experiment, so the result uses the fresh paired repeat rather than the earlier slower baseline. No speed-up is attributed to frequency/load variation. NAS, parallel requests and maximum-context latency remain unmeasured.

## Measured cause and changes

The baseline CPU capture had 11.19 s of samples: packed matrix assembly 38.16% flat, row decoding 14.30%, packed projection's own Go work 10.55%, attention dot/AXPY kernels about 13.5% combined. Remaining decode and packing/scatter loops repeatedly moved individual values.

Changes:

- Whole-row AVX2 Q5_K and Q8_0 decoders use the existing half-conversion table. Q5 nibble/high-plane reconstruction, scale multiplication and minimum subtraction retain separate FP32 operations; Q8 signed-byte expansion retains scale multiplication. No decoder FMA or activation quantisation is added. Scalar/SSE2/non-amd64 fallbacks are unchanged.
- An 8×8 AVX2 transpose packs eight contiguous token rows into column vectors. Full token groups use vector loads/shuffles/stores; final token tails retain the bounded scalar loop. Token padding remains zero.
- Full eight-row/eight-token result panels transpose directly into token-major destinations. This removes the intermediate result-copy loop. Padded-token and partial-row panels keep the previous safe temporary-buffer path. Scatter writes only its eight selected rows, preserving neighbouring row gaps.
- Checked Go wrappers bound dimensions and raw input/output extents before assembly. Existing Q4 SIMD decode now also validates empty/malformed slices before calling assembly, matching the new decoder boundary discipline.

The final CPU capture contained 7.11 s of samples: direct-scatter matrix kernel 37.41% flat, buffered matrix kernel 12.10%; packed projection 62.45% cumulative. SIMD decoding removed the measured scalar-row outlier. Remaining matrix arithmetic and attention are concrete next targets; no speculative gain is included.

Standalone decoder benchmarks measured roughly 15× faster Q5/Q8 decoding with zero allocations. Those are kernel measurements; the accepted benefit is the equivalent full-request table above.

## Memory and allocations

No new scratch array or weight expansion is added. Per-call bytes/objects are unchanged: query 21,464/217, paragraph 140,312/1,466, longer chunk 474,584/5,224. All existing tile capacity limits remain unchanged.

Reverse-order repeat peak RSS was exactly 172,097,536 bytes (~164 MiB) for both processes. Loaded post-GC heap differed by −5,192 bytes, and each workload's heap likewise differed by −5,192 bytes; these are runtime noise, not an allocation-saving claim. Warm load measured 210.98→187.91 ms without a loader change; this difference is not attributed to the optimisation. Maximum-context/concurrent/NAS memory is not qualified.

CPU, alloc_space and alloc_objects were inspected. Sampled process totals were 95.96→86.01 MiB and 865,275→723,159 objects, dominated by tokenizer/parser startup and affected by sampling/GC publication boundaries. These totals do not substitute for the equivalent per-call allocation counters, which remained equal.

## Correctness and safety

All 11 independent reference fixtures passed without regeneration or tolerance changes: RMS `3e-7`, max absolute `1e-6`, cosine at least `1 - 1e-9`. Model output acceptance uses numerical tolerances, never generated-vector hashes.

Passing checks:

- Q5/Q8 decoder oracles for multiple blocks and widths through 2,048, Q5 high-bit patterns, signed zero/nonfinite values and zero allocations. Per-coefficient FP32 and independent FP64 references agree within their existing contracts.
- Guard pages for exact decoder raw/destination boundaries and packing/scatter input/output ends. Packing lengths cover 8/9/15/16/63/64 tokens and 8/256/512/2,048 columns. Scatter tests verify token-major placement and untouched output gaps.
- Multi-panel integration at 8/9/15/16/32/64 tokens, output-row tails, plus Q5/Q8 at 512/2,048 columns. Invalid packing/scatter dimensions and short strided destinations are rejected. Packed-path cancellation and empty/malformed Q4 decoding are tested.
- Focused race, cancellation/Close/workspace tests; AVX/FMA-disabled complete reference suite; explicit Go 1.26.6 real-checkpoint package/CLI suite; Linux ARM64/RISC-V cross-compilation; vet; three repeated synthetic package runs; `CGO_ENABLED=0 go test ./... -count=1`. Non-amd64 binaries were not executed.
- Read-only review found no blocker in the new kernels. Suggested decoder/scatter integration, validation and packed-cancellation coverage was added and passed. The reviewer did not independently inspect the Git diff; baseline attribution comes from the main task's repository checks.

The fixed 16-query diagnostic retrieval scores are unchanged: 16/16 top results at 768/512/256 dimensions, 15/16 at 128. This small authored set does not establish production-corpus quality. Gemma remains slower than GTE-small; production integration requires a separate decision.

Raw before/intermediate/final profiles, matching binaries, source probes and disposable logs/JSON were removed after analysis. Source, pinned checkpoint, independent fixture and these concise measurements remain.
