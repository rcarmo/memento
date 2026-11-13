# EmbeddingGemma 2 quantised CPU foundation

The first isolated engine slice validates and maps the real UD-Q4_K_XL GGUF and performs its mixed-quantisation matrix projections. Tokenization and transformer inference are not implemented yet; no complete embedding or retrieval-quality result is claimed.

## Workload and correctness

Host: Linux amd64, Intel Core i7-12700, Go1.27.1, CGO disabled except race instrumentation. The installed environment selected Go1.27.1; the existing repository default is Go1.26.6. A final explicitly pinned Go1.26.6 real-model test,96.8% coverage and vet rerun also passed; benchmark timings are from Go1.27.1. Source base: c92d3d9 plus this experimental package/inspection command. Existing modified AGENTS.md was not edited or included. No GPU or production operation was performed.

The pinned model is 175,673,856 bytes,413 tensors:194 F32,48 Q8_0,169 Q4_K,1 Q5_K,1 Q6_K. Metadata/directory validation and real projections passed for `blk.0.attn_q.weight` (1024×512 Q4_K), `per_layer_model_proj.weight` (12288×512 Q5_K) and `output.weight` (768×512 Q6_K); a quantised token-embedding row also decoded. No full weight expansion occurs.

Independent test reconstruction extracts logical quant bitplanes and uses float64 arithmetic; it does not call production half/scales/row decoders. Every FP16 bit pattern is checked against an independent exponent/mantissa formula, including signed zero/Inf/NaN. Row decode checks use1e-6 absolute/relative tolerance; projection sums use2e-5×max(1,abs(reference)), reflecting float32 reduction order. Real scalar/SIMD comparisons use3e-4×(1+abs(scalar)); these verify kernel consistency and do not replace independent full-model reference parity. No generated-vector hashes are used as correctness gates.

All supported portable row decoders, scalar/SSE2/AVX2 projections, zero-allocation checks, malformed headers, metadata scalar/array bounds, truncation, duplicate/overlap/alignment/type errors, cancellation, buffer aliasing and mapping lifecycle passed. Exact32-byte/128-byte guard-page tests passed for SSE2/AVX2 packed groups. Tests passed ten repetitions, race, AVX/FMA-disabled SSE2 fallback, and10,000 parser fuzz iterations. New package statement coverage is96.8%; cross-compilation passed for Linux ARM64 and RISC-V64, but those binaries were not executed. Vet passed for package and CLI. Inspector CLI passed with the real model.

A metadata-budget tightening initially rejected the real checkpoint because its parallel tokenizer metadata arrays exceed one million aggregate entries. Admission now bounds each array at one million and the aggregate at two million, still inside a64MiB input-header budget; the model passed again. This is format admission, not relaxed numerical acceptance.

## Measured tuning

Benchmark: one512×512 Q4_K projection, deterministic FP32 activation vector, quantised weights fixed outside the timer, caller-owned row workspace and output. Five250ms samples per backend, same host/toolchain/workload. No wall-clock gate was added.

| Backend | Initial median ns/op | Final median ns/op | Change |
|---|---:|---:|---:|
| Scalar decode + dot | 483,169 | 469,782 | −2.8%; host noise, no scalar change |
| SSE2 | 407,324 | 80,165 | −80.3%;5.08× faster |
| AVX2 | 383,776 | 52,261 | −86.4%;7.34× faster |

All rows report0 B/op and0 allocs/op. Baseline timing ranges were482,493–597,172ns scalar,401,253–511,111ns SSE2 and383,065–396,005ns AVX2. Initial post-tuning AVX2 repeats were51,740–52,325ns and SSE2 repeats79,149–80,719ns; a final same-source repeat produced the medians above.

Before tuning, a2-second AVX2 CPU profile captured2.37s samples: row dequantisation89.87% flat/94.94% cumulative, dot kernel5.06% flat. The fix fuses per-group nibble decode, scale/min application and FP32 dot into SSE2/AVX2 leaf kernels, avoiding a scratch-row write/read. It keeps original quantised values and FP32 activations, adds no activation quantisation and avoids FMA-dependent numerical approximation.

After tuning, the equivalent CPU profile captured2.43s samples: fusedQ4Dot97.12% cumulative,43.62% flat; assembly group kernel43.62% flat, scale extraction5.35%, half conversion4.53%. Helper-call/scale overhead is a concrete future candidate for block-wide kernels, but the verified increment already removes the material row-decode outlier. Do not extrapolate this7.34× kernel ratio to transformer inference.

Heap profiles at default524,288-byte sampling had3.75MiB cumulative allocated space in both before/after runs; objects18,358 before and40,983 after were dominated by profiling/runtime/testing setup. Hot-loop benchmark allocations stayed exactly zero. Total-process profile-object counts are not an application allocation regression or a saving claim. CPU, alloc_space and alloc_objects were inspected; captures/binaries/disposable logs were removed after analysis, keeping these concise results.

One separate warm-filesystem inspection measured0.06s elapsed and51,712KiB maximum process RSS. It includes header/token metadata parsing and directory JSON output, not transformer execution or a cold-model peak. A mapped175.7MB file does not establish full resident memory; repeated inference and container peak remain unmeasured.

## Limits and next step

No end-to-end tokenizer, attention, pooling or embedding output; no independent runtime golden output, ARM execution, NAS benchmark, model-wide RSS qualification, Memento integration, full repository audit, release or production change. Initial compiler attempts failed on incorrect assumed SIMD API names, then the actual existing Engine API was used and all final gates above passed.

Next implement a bounded tokenizer/model contract and stage-level reference tests, resolving GGUF-versus-Transformers sliding-window semantics before attention acceptance. The reusable projection foundation is ready for that work. See [engine design and commands](../embeddinggemma-native.md).
