# Native EmbeddingGemma 2 engine

The first implementation slice loads the selected real text-only quantised checkpoint and runs validated quantised matrix projections in pure Go with SSE2/AVX2 leaf kernels. It does **not yet tokenize text or produce complete embeddings**. Production GTE, its worker protocol, Docker runtime and index are unchanged.

## Selected checkpoint

Start with `unsloth/embeddinggemma-2-GGUF` revision `031f0d4b35536f69ab3509d4893c923264fcf253`, `embeddinggemma-2-UD-Q4_K_XL.gguf`: 175,673,856 bytes, SHA-256 `ea905fd08e8061db77a0031cbb5096d459d7cfd2bdd0ad2ba66e483f16719493`. The model is Apache-2.0. The durable local weight asset is git-ignored; `models/embeddinggemma/manifest.json` pins its provenance. No multimodal projector is needed.

This selects a practical native-format candidate. LiteRT's ~165MB INT4 QAT model uses a different container/quantisation/runtime; UD-Q4_K_XL is not numerically equivalent or automatically equal in retrieval quality. The real GGUF mixes 194 F32, 48 Q8_0, 169 Q4_K, one Q5_K and one Q6_K tensor. The Q5_K per-layer projection and Q6_K output projection must not be silently treated as Q4_K.

The initial download probe mistook the CDN ETag for a SHA-256 and failed verification. The file was subsequently verified against the pinned Hugging Face LFS object's actual SHA-256 and exact length before being installed. The CDN ETag is not asset integrity evidence.

## Scope and interfaces

- `internal/embeddinggemma.Open`: bounded GGUF v2/v3 metadata/directory parsing, fixed architecture, supported type admission, shape/product/offset/overlap/alignment checks and read-only mapping.
- `File.Matrix`: model-owned quantised storage, no global FP32 expansion. Tensor metadata returns copied shapes; tokenizer arrays remain private.
- `Matrix.RowTo`: caller-owned dequantised row for token lookup and reference inspection.
- `Matrix.ProjectTo`: bounded caller-owned scratch, no steady-state heap allocations, context checks every sixteen output rows and after completion. Output may be partial on cancellation/error and must not be published then.
- F32/F16/BF16/Q8_0/Q4_K/Q5_K/Q6_K decoding. No inference-time activation quantisation has been added; projections consume FP32 activations, avoiding an extra unqualified approximation.
- Fused Q4_K dequantisation/dot for SSE2/AVX2. Other types use one reusable decoded row and the existing architecture-selected dot kernel. ARM64 uses the existing NEON dot after scalar row decode; other CPUs use scalar. No native library, CGo or external runtime.
- `cmd/memento-gemma-inspect`: explicit inspection-only command; not part of production/static release archives.

`Close` waits for projections and prevents later access. Concurrent projections need separate caller buffers. Inputs/output/scratch cannot overlap. Treat the underlying weight file as immutable while mapped; external truncation/replacement is outside this API's ownership contract. Non-Unix mapping platforms return unsupported instead of allocating an undisclosed whole-file copy.

## Reuse and isolation

GGUF block semantics and the immutable-matrix/caller-workspace approach follow `go-pherence/loader/gguf/quant_matrix.go` at `2c283159`. See `internal/embeddinggemma/THIRD_PARTY.md`. The local go-pherence checkout has unrelated dirty edits; none were changed or imported as a live dependency. Existing Memento SIMD capability detection supplies AVX2/FMA, SSE2, NEON and scalar selection. The new packed Q4 leaf functions are separate from existing GTE/Needle code.

All scratch and profiling work uses the existing portable `memento-go` project environment. Weights are a durable input under `models/embeddinggemma`; temporary profiles/binaries/logs are removed after the current analysis, leaving concise measurements only.

## Development commands

```sh
make gemma-test
make gemma-bench
make gemma-inspect
```

`EMBEDDINGGEMMA_MODEL=/absolute/path/model.gguf` overrides the test/inspection path. The default names the pinned local artifact. Ordinary tests without the variable skip real-model checks; offline synthetic tests still run. Do not interpret that skip as real-model qualification. These commands do not call production or a GPU.

## Remaining engine work

1. Implement Gemma4 tokenizer behaviour with the actual checkpoint vocabulary/scores, special tokens, BOS/EOS, Unicode and task/document prefixes; compare token IDs with a pinned independent reference.
2. Implement bidirectional sliding/full attention, per-layer input projection/gates, Q/K/V normalisation, RoPE, residual scalars, MLP and final 768d projection in a bounded request workspace. Validate tensor shapes before running.
3. Resolve reference discrepancies explicitly: this GGUF advertises sliding window1024 and positional capacity262144; the inspected Transformers config used sliding window512 and the model card promises an8192-token embedding context. Determine conversion/runtime mask semantics from pinned implementations before accepting attention parity. Do not guess or silently widen input limits.
4. Add mask-aware pooling, query/document role policy, L2 normalisation and post-truncation renormalisation (768/512/256/128d). Validate numerical error distributions against independent outputs; hashes identify model inputs only.
5. Measure full-model cold/warm latency, allocations, retained/peak memory and cancellation with real text. Compare quantised versus full-precision reference retrieval on a held-out corpus before considering a Memento backend or worker integration.

The initial quantised kernels do not establish end-to-end embedding quality, full-model RAM under512MiB, old-NAS performance or a production replacement. No production refresh or index migration belongs to this implementation slice.
