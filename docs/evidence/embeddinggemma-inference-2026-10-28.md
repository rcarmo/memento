# EmbeddingGemma 2 native text inference

The CPU-only engine now generates embeddings from the pinned UD-Q4_K_XL GGUF. It includes Gemma 4 BPE, 24 transformer layers, bidirectional local/global attention, per-layer gates/projections, mean pooling and normalised 768/512/256/128-dimensional output. Production GTE, its worker, image and index were unchanged.

## Correctness

Base revision `ef9cb52`, branch `feat/embeddinggemma-native`; Linux amd64, Intel i7-12700. Tests and measurements used Go 1.27.1; focused real-model, lifecycle, scalar/SIMD and CLI tests also passed with explicitly pinned Go 1.26.6. Runtime builds use `CGO_ENABLED=0`; race instrumentation uses CGo only for tests.

Independent reference: llama.cpp `c3df4b7e22a1ac38da663552d19e649dd97ca936`, CPU four threads, same quantised model asset. The reference dequantises every 2D matrix to FP32, disables FP16 GELU lookup, and runs with flash attention off. It independently computes token IDs and forward outputs; it shares no Go kernel implementation. The retained fixture records source changes and asset provenance. A separate reference-decoder check sampled first/middle/last rows from every matrix: maximum absolute decode difference was zero.

The converter doubles the Transformers sliding radius 512 into GGUF's symmetric span 1,024. Both implementations include positions at distance 512. Global attention occurs every sixth layer. A 1,028-token case and a separate attention oracle test both exercise the boundary. The API admits at most 8,192 tokens, including prefixes and BOS/EOS; it rejects overflow without truncation.

Eleven reference cases passed: empty, punctuation, accented/decomposed Unicode/CJK/emoji, whitespace/newlines, literal special token, query/document prompts, three Matryoshka dimensions, and the long window case. Exact token IDs matched. Final RMS component error was `6.12e-9`–`6.19e-8`; maximum absolute error was `2.39e-7`; cosine exceeded `1 - 1e-9`. Gates are RMS `3e-7`, max absolute `1e-6`, cosine `1 - 1e-9`, calibrated above observed FP32 rounding with a several-fold margin. Truncated reference prefixes are renormalised before comparison. No generated-output hash gate is used.

### Failed checks and reference corrections

The first apparently FP32 reference produced roughly `1e-4` RMS error; special-token and truncated-vector cases failed initial tolerances. Removing FP16 GELU and disabling flash attention did **not** resolve those failures. An early progress update incorrectly attributed a pass to GELU; the subsequent actual rerun disproved that explanation.

Inspection found the converter excluded `per_layer_model_proj.weight` from F32 conversion. Its Q5_K path still quantised activations. Removing that exclusion for the disposable reference conversion reduced errors to `1e-8` without changing native numerical behaviour or widening tolerances. Final gates were tightened. Dequantised weights are derived from the exact selected GGUF; comparison with the original unquantised model's retrieval quality is a separate evaluation.

## Profiling and tuning

Equivalent workload before/after: 15-token prompted query `task: search result | query: Where is the project documentation?`, 768-dimensional output, fixed model, automatic AVX2 backend, one reusable workspace, warm-up before timing, five iterations per sample, three samples. CPU and heap profiles include test startup/model loading, so total-process allocation figures are separate from steady-state per-call allocations.

| Metric | First complete encoder | Tuned encoder | Change |
|---|---:|---:|---:|
| Warm median | 1,009,452,759 ns | 593,902,317 ns | −41.2%, 1.70× faster |
| Warm range | 1.0093–1.0121 s | 0.5731–0.6231 s | host noise included |
| Bytes / call | 3,072 | 3,072 | unchanged; owned returned vector |
| Allocations / call | 1 | 1 | unchanged |
| Total-process alloc_space | 746.67 MiB | 456.40 MiB | −38.9% |
| Total-process alloc_objects | 11,497,583 | 3,954,756 | −65.6% |

Initial CPU profile: 25.87 s samples; row decoding 55.04% flat, quantised projection 91.23% cumulative. Q5_K per-layer projection decoding was material. Added fused SSE2/AVX2 Q5_K high-bit/nibble decode-and-dot kernels, keeping FP32 activations and original weights. Independent high-plane tests and guard pages validate exact packed/high/activation load boundaries.

The tuned CPU profile had 15.67 s samples: Q4 fused dispatch/scale work 26.55% flat, Q4 AVX2 group kernel 25.91%, remaining row decoding 17.17%, Q5 AVX2 group kernel 2.55%; projection cumulative 85.32%. The large Q5 outlier was removed. Residual Q8 decode and Q4 block-helper overhead are bounded kernel work; further block-wide Q4/Q8 fusion is a non-blocking optimisation, with current complete-request improvement measured above.

Startup allocation profiles ranked tokenizer maps and parser scalar/string work. Compact merge keys now use token-ID pairs; scalar reads reuse parser-owned storage, and buffered strings avoid a redundant byte copy. `Load` releases tokenizer metadata arrays after constructing owned lookup tables. Tokenizer flat allocated space fell from 351.06 to 182.78 MiB across benchmark invocations; parser text from 112.50 to 51 MiB. Remaining allocation work occurs mainly at model load, outside the zero-scratch-allocation forward loop.

A separate executable run measured 0.75 s elapsed and 166,912 KiB maximum RSS (~163 MiB) for the same query. It includes load and one forward pass with warm filesystem data. NAS timing, full-context peak memory and a cold-disk measurement were not performed. Activations use linear storage; attention compute remains quadratic in global layers. An 8,192-token request can be slow on CPU and requires separate operational limits before service integration.

## Verification

- `make gemma-test`: real-checkpoint package/CLI tests and all 11 reference vectors passed.
- `bun tools/gemma-reference.mjs` with built native CLI and live independent CPU servers: all 11 cases passed final tightened gates.
- `CGO_ENABLED=0 go test ./... -count=1`: repository suite passed. Model-dependent suites retain their usual environment requirements; Gemma's explicit real-checkpoint run is listed separately.
- Focused race checks passed for concurrent requests, independent workspaces, busy-workspace rejection, cancellation, Close during an active request, tokenizer and Q4/Q5 kernels/guard pages.
- AVX/FMA-disabled SSE2 and scalar consistency checks passed. Linux ARM64/RISC-V test binaries cross-compiled; those CPUs were not executed.
- 10,000 GGUF fuzz iterations, package/CLI vet, repository compile and whitespace checks passed.
- A coverage run before the last added CLI/admission tests measured 91.5% for the expanded encoder package. Full zero-uncovered coverage was not a gate for this isolated experiment.
- The independent automated code-review delegate timed out without returning a review; it is not counted as passed review evidence.

Raw before/intermediate/final CPU and heap captures, matching binaries, disposable logs, temporary all-F32 weights and reference builds are removed after the completed analysis. Source, pinned original checkpoint, compact reference fixture and these measurements remain. See [API, reference recipe and commands](../embeddinggemma-native.md).
