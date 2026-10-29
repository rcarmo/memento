# EmbeddingGemma projection reuse across tokens

Bounded token tiles reduced Gemma latency by another 29% for a short query and 52–54% for longer inputs on this host. The optimisation reuses decoded weight coefficients across tokens; it keeps the original quantised model, FP32 activations and existing reference tolerances. Production GTE, its worker, image and index were unchanged.

## Host comparison

Baseline `c780d8e`; Linux amd64, Intel Core i7-12700, six visible logical CPUs, Go 1.27.1, CGO disabled, automatic AVX2, `GOMAXPROCS=1`, sequential calls. The pinned UD-Q4_K_XL GGUF and `tools/embedding-compare/corpus.json` are unchanged. Timings include tokenisation, recommended prefixes and normalised 768-dimensional output, with reused request workspaces. Filesystem pages were warm; model loading and index/IPC overhead are excluded.

The first profiled baseline measured 251.65 ms / 1,165.90 ms / 4,519.70 ms. The final profiled candidate measured 178.75 ms / 557.24 ms / 2,102.43 ms. An unprofiled repeat ran candidate then baseline, preserving input, warm-up, sample counts and output checks:

| Workload | Tokens | Baseline median | Candidate median | Reduction | Speed-up |
|---|---:|---:|---:|---:|---:|
| Query, 35 source characters | 15 | 250.99 ms | 177.69 ms | 29.2% | 1.41× |
| Paragraph, 406 characters | 69 | 1,164.79 ms | 553.84 ms | 52.5% | 2.10× |
| Longer chunk, 1,516 characters | 254 | 4,506.78 ms | 2,084.68 ms | 53.7% | 2.16× |

Unprofiled ranges were 246.68–253.39 / 175.01–178.83 ms, 1,154.98–1,170.35 / 549.00–556.10 ms, and 4,506.69–4,528.18 / 2,080.81–2,103.27 ms. There were seven, five and three timed samples respectively. The host was not exclusively reserved or pinned to one core. NAS, parallel requests and maximum-context latency were not measured.

## Cause and implementation

The fresh baseline CPU profile captured 27.18 s. Whole-row Q4 assembly occupied 61.85% flat; Q5 projection 10.34% cumulative; Q8 assembly 5.00% flat. Every token independently decoded and traversed the same immutable weights.

The candidate processes up to 64 tokens per projection tile:

- For larger tiles, decode one weight row into the existing bounded scratch and apply `simd.Engine.DotRows` to the tile's FP32 activation rows. Decoding cost is shared across tokens. No weight matrix or complete model is expanded to FP32.
- Decode Q4 rows with AVX2. Scalar row decoding was too expensive in the initial candidate. Other tensor formats retain their reference row decoders; their cost is amortised by the tile.
- For fewer than 32 tokens, an AVX2 four-token Q4 kernel shares each coefficient vector across four independent activation streams without writing a decoded row. One-to-three trailing tokens use the existing row-dot kernel. Non-AVX2 backends use the bounded decoded-row path. Single-token projections retain the existing fused implementation.
- Compute the complete sequence's Q/K/V before bidirectional attention. Only after attention are per-token attention residuals, FFN and projection-only per-layer gates tiled. Token order, residuals, masks and pooling are unchanged.
- Reuse six tile arrays and a small sums array per workspace. Remove obsolete full-sequence normalisation and single-token FFN/PLE buffers. Cancellation remains checked between bounded projection row groups, during attention and before returning output. Close still waits for the full request read lock.

Exploratory candidates were measured and replaced: 16-token tiles with scalar row decode regressed to 295 ms / 1.43 s / 4.99 s. SIMD Q4 decode improved that to 262 ms / 1.06 s / 2.87 s. A 64-token tile reached 244 ms / 613 ms / 2.27 s but did not materially improve short queries. Adding the four-token fused path produced the accepted results above. No approximate activation quantisation, larger numerical tolerance, thread oversubscription or full-weight expansion was introduced.

The final CPU profile contained 13.25 s of samples. FP32 `dotRowsAVX2` became the largest cost at 41.36% flat, row decoding 10.94%, exponentiation 7.09%, four-token Q4 assembly 5.06%, GELU 5.06% cumulative. Tiled projection was 66.57% cumulative. The previous repeated quantised-decode outlier was reduced; remaining row-dot arithmetic and attention are concrete follow-up candidates. No speculative future gain is included in these measurements.

## Allocations and memory

Text-to-vector allocation totals were identical before and after: query 21,464 bytes / 217 objects, paragraph 140,312 / 1,466, longer chunk 474,584 / 5,224. Tile storage is allocated on first workspace use and reused. All projection-tile tests require zero steady-state projection allocations.

The six fixed arrays hold 1.5 MiB of FP32 data, plus a 256-byte sums array. Removed buffers offset part of that cost: measured workload heap rose by 1,520,064 bytes for the query, 1,372,608 for the paragraph and 881,088 for the longer chunk. Removing full-sequence normalisation saves more memory as input length grows; maximum-context savings are an allocation-size calculation, not a measured resource qualification.

In the reverse-order repeat, post-GC model heap was unchanged (55,381,264→55,381,456 bytes). Longer-workload RSS rose 165,056,512→166,199,296 bytes (+1.09 MiB / 0.7%); peak RSS rose 171,966,464→172,228,608 bytes (+0.25 MiB / 0.15%). These measured secondary memory increases are accepted for the verified latency reduction. Transient process memory varies with GC/filesystem/runtime state; this is not a full-context or concurrent-service guarantee. Warm-filesystem load was 206.90→187.13 ms; no loader optimisation was made and the difference is not attributed to this work.

CPU, alloc_space and alloc_objects profiles were inspected. Sampled whole-process alloc_space was 81.46 MiB before / 87.04 MiB after; objects 673,833 / 537,630. Startup parser/tokenizer allocations dominate those profiles, and sampling/GC publication boundaries differ. They do not replace the equivalent per-call TotalAlloc/Mallocs measurements, which remained equal. The fixed workspace increase is explicitly counted above.

## Correctness and safety

All 11 independent CPU reference fixtures passed without regeneration or threshold changes: RMS limit `3e-7`, max absolute `1e-6`, cosine at least `1 - 1e-9`. The long-window case remained below `2.28e-7` maximum absolute component error. Numerical acceptance uses distributions/tolerances, not vector hashes.

Verification passed:

- Projection-tile oracle tests for F32/F16/BF16/Q4_K/Q5_K/Q6_K/Q8_0 and available scalar/SSE2/AVX2 backends; tile lengths 1/2/3/4/5/15/16/31/32/63/64 cover dispatch and remainder boundaries.
- Independent FP64 row references, zero projection allocations, invalid shape rejection and cancellation.
- Linux guard-page tests for the new Q4 decode and four-token kernels, placing final packed weights, activations, decoded row and four results at protected-page boundaries.
- Full real-checkpoint package/CLI suite under explicitly pinned Go 1.26.6; performance measurements used installed Go 1.27.1.
- Focused race tests for kernels, tiled projection, model concurrency, busy-workspace rejection, cancellation/Close and CLI; AVX/FMA-disabled full reference suite exercising SSE2 fallback.
- Linux ARM64/RISC-V test cross-compilation, package/CLI vet and `CGO_ENABLED=0 go test ./... -count=1`. Non-amd64 binaries were not executed.
- Read-only review of assembly bounds/ABI, tile aliasing, bidirectional attention sequencing and cancellation found no substantive blocker. The private projection helper relies on caller-owned disjoint buffers; all model call sites and tests satisfy that contract.

Initial new test-helper names and a final assembly trailing blank line caused failed probe/check runs and were corrected; they are not passing evidence. The 16-query diagnostic retrieval set was rerun: 16/16 correct top results at 768, 512 and 256 dimensions, 15/16 at 128, unchanged. Production-corpus quality is still unmeasured.

Raw before/intermediate/final captures, matching binaries, source probes and disposable logs/JSON were removed after analysis. Source, pinned checkpoint, independent fixture and these conclusions remain. Production integration requires a separate decision.
