# EmbeddingGemma batched long-sequence attention

Batching attention dot products and value accumulation reduced 254-token request latency by 3–10% in opposite-order, same-core comparisons. Inputs below 128 tokens keep the previous path. The checkpoint, numerical reference limits and production GTE are unchanged.

## Measurements

Baseline `67b58cb`; Linux amd64, Intel Core i7-12700, Go 1.27.1, CGO disabled, automatic AVX2, `GOMAXPROCS=1`. The host had competing workloads and timing drift during the experiment. Final paired measurements pin both executables to logical CPU 0 and use the identical longer input from `tools/embedding-compare/corpus.json`, including its document prompt, 254 tokens and normalised 768-dimensional output. Each process warms up and runs five samples; model load, worker IPC and index lookup are excluded.

| Order | Baseline median | Candidate median | Reduction |
|---|---:|---:|---:|
| Baseline then candidate | 1,195.79 ms | 1,160.02 ms | 3.0% |
| Candidate then baseline | 1,214.26 ms | 1,091.82 ms | 10.1% |

Ranges were 1,190.35–1,222.24 / 1,124.01–1,163.13 ms and 1,187.71–1,245.38 / 1,074.57–1,131.44 ms. The improvement direction was consistent, but the magnitude varies with host conditions; no single larger gain is selected as definitive. A final profiled full-workload candidate measured 68.46/284.31/1,067.43 ms for 15/69/254 tokens. Short query and paragraph execution paths are unchanged, so their timing differences are not attributed to this change.

The fresh unpinned baseline measured 67.97/284.20/1,204.46 ms; later unpinned/pinned comparisons showed host variation larger than a modest paragraph improvement. The initial candidate enabled batching from 32 tokens. Final dispatch uses 128 tokens so only longer sequences receive the measured increment. No GPU, concurrent request scheduling, model migration or production operation was performed. NAS and maximum-context latency remain unmeasured.

## Profile and implementation

Fresh baseline CPU samples: 7.11 s. Packed matrix kernels remained dominant; attention was 20.82% cumulative, with many per-token `Dot` and `AXPY` calls. The change:

- Transpose token-major K/V heads to contiguous head-major rows using existing attention-output scratch. Copy back before reusing scratch; no new array or model expansion is allocated.
- Compute score rows with `Engine.DotRows`, then accumulate values with `Engine.AXPYRows`, in groups of at most 256 tokens. This reuses input/output vectors and avoids a Go/assembly transition for every score/value.
- Preserve GQA mapping, inclusive local radius 512, bidirectional/global attention, max subtraction, FP32 exponentials/denominator, token-order value accumulation and mean pooling. Check cancellation between bounded score/value groups and before returning output.
- Retain the old attention path for fewer than 128 tokens and keep all CPU-backend fallbacks through the existing SIMD engine.

The final retained CPU capture contained 6.55 s of samples; contiguous attention was 11.30% cumulative. Matrix arithmetic is still the largest remaining cost. Different process profiles include different transient host timing, so these percentages diagnose reduced repeated work rather than establish a standalone speed-up.

Exploratory changes were rejected: a wider matrix layout did not improve full-request timing; GELU tail branching regressed all three probe workloads; four-way matrix unrolling failed the existing guard-page test on odd-column kernel widths. All three were reverted. No failed probe is counted as passing verification and no speculative future gain is recorded.

## Memory and allocations

No new scratch or cache capacity is added. Equivalent 254-token calls retain 474,584 bytes / 5,224 allocations including tokenisation and the owned vector. Workload heap differed by only −176/−496 bytes in the paired rounds, consistent with runtime noise. The transpose uses existing disjoint request buffers.

Peak process RSS varied: round one 178,126,848→178,352,128 bytes (~+0.21 MiB); round two 164,503,552→178,388,992 bytes (~+13.24 MiB). The second transient difference is recorded as a measured secondary resource regression, without attributing it to new storage. Direct heap/capacity/per-call allocation measures do not increase. The bounded change is accepted for the verified longer-input latency improvement; maximum-context/concurrent/NAS memory is not qualified.

CPU, alloc_space and alloc_objects captures were analysed. Sampled whole-process alloc_space was 89.49→80.02 MiB and objects 881,075→711,599, dominated by tokenizer/parser startup and sampling/GC publication effects. These totals are not an allocation-saving claim; direct per-workload counters are unchanged.

## Correctness and verification

All 11 independent CPU fixtures passed without regeneration or tolerance changes: RMS `3e-7`, max absolute `1e-6`, cosine at least `1 - 1e-9`. Generated-vector hashes have no acceptance role. The 1,028-token reference case uses the retained path; smaller reference inputs use the original path.

Checks passed:

- Direct old/new attention comparisons across available scalar/SSE2/AVX2/NEON backends, 1/7/31/32/69/127/128/129/255/256/257/513 tokens and one/two KV heads, both local/global masks. Assertions explicitly reject nonfinite results.
- Independent FP64 attention oracle at the ±512 mask boundary and at production head/KV configurations `(256,2)` / `(512,1)`. Production tests use attention output as transpose scratch, matching model buffer reuse.
- Deterministic cancellation after one completed score group, pre-cancelled calls, model concurrency, busy-workspace rejection, cancellation/Close and focused race checks.
- Full real-checkpoint package/CLI suite on explicitly pinned Go 1.26.6; AVX/FMA-disabled full reference suite; Linux ARM64/RISC-V test cross-compilation; vet; `CGO_ENABLED=0 go test ./... -count=1`. Non-amd64 binaries were not executed.
- Read-only review found no blocker in head mapping, transpose ownership, softmax order or cancellation. Suggested production-head, scratch reuse, dispatch-boundary, finiteness and mid-group-cancellation tests were added and passed.

The fixed 16-query diagnostic retrieval set is unchanged: 16/16 top results at 768/512/256 dimensions, 15/16 at 128. Its short inputs do not assess long-document retrieval quality or the production corpus. Production integration requires a separate decision.

Raw profiles, matching binaries, rejected source/kernel probes and disposable logs/JSON were removed after analysis. Source, pinned checkpoint, independent fixture and these conclusions remain.
