# EmbeddingGemma packed matrix panels

An eight-row AVX2 matrix kernel and bounded RoPE angle cache reduced complete Gemma text-to-vector latency by another 30–40% on this host. The quantised checkpoint, FP32 activation representation and independent reference tolerances are unchanged. Production GTE, its worker, image and index were untouched.

## Equivalent workloads

Baseline `b79c887`; Linux amd64, Intel Core i7-12700, six visible logical CPUs, Go 1.27.1, CGO disabled, automatic AVX2/FMA, `GOMAXPROCS=1`, sequential calls. Original GGUF and `tools/embedding-compare/corpus.json` unchanged. The texts contain 35/406/1,516 source characters and 15/69/254 tokens after recommended prompts. Timings include tokenisation and normalised 768-dimensional output, reuse the workspace and exclude load/index/IPC overhead. Filesystem data was warm; other host workloads were not suspended.

Unprofiled repeat ran candidate then baseline, with identical warm-up and seven/five/three timed samples:

| Workload | Baseline median | Candidate median | Reduction | Speed-up |
|---|---:|---:|---:|---:|
| Query | 178.56 ms | 107.05 ms | 40.0% | 1.67× |
| Paragraph | 568.62 ms | 400.63 ms | 29.5% | 1.42× |
| Longer chunk | 2,146.52 ms | 1,499.54 ms | 30.1% | 1.43× |

Ranges were 177.69–180.00 / 106.60–107.52 ms, 558.62–578.54 / 395.35–401.92 ms, and 2,120.18–2,298.41 / 1,490.17–1,502.45 ms. Fresh profiled baseline medians were 176.52/556.64/2,130.26 ms; the final bounds-checked profiled candidate measured 107.85/404.96/1,512.07 ms. Full-request improvements are stable across profiling and reverse-order repeats.

## Profile and implementation

The fresh baseline CPU capture had 13.29 s of samples: `dotRowsAVX2` 41.38% flat, row decoding 9.86%, exponentiation 8.20%, four-token Q4 4.97%, `Sincos` 4.74%. Dot-row work repeatedly loaded each activation row, then horizontally reduced each output dot product.

The new AVX2/FMA kernel multiplies eight decoded weight rows by eight token columns at once. Activations are packed column-major once per projection tile, still bounded at 64 tokens. Each activation vector is reused across eight output rows. Each lane accumulates one token's dot product, avoiding horizontal reductions. Weights are decoded only into an eight-row request-owned panel; no full matrix or model is expanded. FMA changes rounding and reduction order but preserves the FP32 mathematical contract; the unchanged independent output tolerances validate the result. `simd.AVX2` already requires both AVX2 and FMA. Unsupported CPUs keep the former tiled/fused path.

A checked Go wrapper validates positive bounded dimensions, token padding and all slice lengths before passing pointers to assembly. The projection helper pads token counts to multiples of eight, clears the padding, handles incomplete eight-row panels, and copies only actual rows/tokens to the output. Cancellation is checked before packing and per panel; Close/workspace ownership remain unchanged.

The request-owned RoPE cache contains at most 64 positions of FP32 sine/cosine coefficients, with lazy local/global float64 frequency tables. Heads reuse angles rather than repeating `Pow` and `Sincos`. Cache identity includes position base, count and local/global mode. Tests compare against the original direct expression at zero, tile edges, 511/512 and 8,191, including norm checks. No context-length cache, cross-request shared mutable state or approximate trigonometry was added.

Exploratory four-row/eight-token panels measured 132/538/1,993 ms. Four-row/sixteen-token FMA panels measured 127/541/2,315 ms; the wider layout regressed longer inputs and was replaced. Eight-row/eight-token panels plus cached RoPE measured 114/436/1,632 ms without FMA; FMA then reached 110/412/1,527 ms before the final repeated results above. No rejected variant remains active.

The final checked candidate CPU capture had 9.38 s of samples: packed matrix assembly 36.99% flat, remaining row decode 13.75%; packed projection 66.84% cumulative. Packing/scatter, decoding and attention are the next concrete candidates. Remaining CPU cost is not projected into an unmeasured future speed-up.

## Memory and allocations

Text-to-vector allocation deltas are unchanged: query 21,464 bytes / 217 objects, paragraph 140,312 / 1,466, longer chunk 474,584 / 5,224. Projection tests still require zero steady-state allocations.

The new maximum activation packing panel reserves 512 KiB per used workspace. The decoded weight scratch grows from 2,048 to 16,384 floats (+56 KiB); sums grow by 1.75 KiB. RoPE adds 128 KiB coefficients plus about 3 KiB frequency arrays and metadata. The measured workload heap rose 722,272 bytes (~0.69 MiB) across all lengths. Workspace construction's embedded RoPE state raises model-load-run heap accounting by about 139 KiB before its first projection buffers are allocated; no model weights changed.

In the unprofiled repeat, peak RSS was 171,835,392→172,097,536 bytes (+0.25 MiB / 0.15%). Resident-memory samples vary with GC/page state, and the bound is scratch capacity rather than a whole-process guarantee. The secondary scratch increase is accepted for the verified 30–40% latency reduction. Maximum-context/concurrent/NAS resource use is not qualified. Warm load measured 185.71→191.77 ms; this is not a loader improvement claim.

CPU, alloc_space and alloc_objects captures were inspected. Sampled process totals were 84.19→82.16 MiB and 652,118→729,463 objects, dominated by tokenizer/parser startup; sampling and GC publication boundaries affect these totals. Per-workload TotalAlloc/Mallocs remained equal. No load-time allocation gain is attributed to the change.

## Validation

All 11 independent CPU fixtures passed without regeneration or threshold changes: RMS `3e-7`, max absolute `1e-6`, cosine at least `1 - 1e-9`. Observed truncated-vector max error was `2.49e-7`; long-window max was `2.26e-7`. Generated-vector hashes have no acceptance role.

Passing checks:

- All tensor-format tile tests, with counts around packing/fallback boundaries and independent FP64 oracles; zero steady-state allocations.
- Additional AVX2 tests with 8/9/15/16/17 output rows and 9/15/64 tokens exercise full panels, subsequent panels, padding and final-row tails.
- Invalid assembly-panel dimensions/lengths rejected by the checked wrapper. Guard-page tests for 1/7/256/512/2,048 columns and 8/16/64 tokens place weight/activation/output ends at protected pages.
- Exact direct-RoPE coefficient/formula comparisons and norm preservation; request reuse and mode changes tested.
- Focused race, cancellation/Close/workspace tests, AVX/FMA-disabled complete reference suite, explicit Go 1.26.6 real-checkpoint package/CLI suite, Linux ARM64/RISC-V cross-compilation, vet and `CGO_ENABLED=0 go test ./... -count=1`. Non-amd64 binaries were not executed.
- Read-only review found no production-path blocker; its suggestions for checked assembly preconditions and second-panel tests were implemented and passed. An earlier RoPE delegate timed out after writing its source/tests; the main task reviewed, integrated and tested them, so no passing delegate result is assumed.

The fixed 16-query diagnostic retrieval results are unchanged: 16/16 top results at 768/512/256 dimensions and 15/16 at 128. This small authored set does not establish production-corpus quality. Gemma is still slower than GTE-small; NAS and production integration need separate evaluation.

Raw before/intermediate/final captures, matching binaries, source probes and disposable logs/JSON were removed after analysis. Source, pinned checkpoint, independent fixture and concise measurements remain.
