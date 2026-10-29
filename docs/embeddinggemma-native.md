# Native EmbeddingGemma 2 engine

`internal/embeddinggemma` generates text embeddings on CPU from the pinned text-only GGUF, using Go and small Go-assembler SIMD kernels. It implements the tokenizer, all 24 transformer layers, mean pooling, output projection and L2 normalisation. The offline CLI is separate from production GTE, its worker protocol, Docker image and existing index.

## Checkpoint

`models/embeddinggemma/manifest.json` pins `unsloth/embeddinggemma-2-GGUF` revision `031f0d4b35536f69ab3509d4893c923264fcf253`, file `embeddinggemma-2-UD-Q4_K_XL.gguf`: 175,673,856 bytes; SHA-256 `ea905fd08e8061db77a0031cbb5096d459d7cfd2bdd0ad2ba66e483f16719493`; Apache-2.0. The git-ignored weights are a durable input. No multimodal projector is required.

The file contains 194 F32, 48 Q8_0, 169 Q4_K, one Q5_K and one Q6_K tensor. The engine keeps quantised weights mapped read-only. Activations remain FP32; it never expands the complete model. LiteRT INT4 QAT uses different quantisation and requires a separate quality comparison.

## Commands

```sh
make gemma-test                 # synthetic and real-checkpoint/reference-fixture checks
make gemma-embed                # builds the offline CLI; prints its build path
make gemma-inspect              # metadata and tensor directory
make gemma-bench                # projection and warm full-encoder benchmarks
```

Set `EMBEDDINGGEMMA_MODEL=/absolute/path/model.gguf` to override the default checkpoint. Direct commands must first source `tools/project-env.sh`; it resolves portable project-owned scratch/cache/build paths. In a shell:

```sh
. tools/project-env.sh
go run ./cmd/memento-gemma-embed \
  -model models/embeddinggemma/embeddinggemma-2-UD-Q4_K_XL.gguf \
  -role query -text 'Where is the project documentation?' -dimensions 768
```

The CLI writes JSON `{tokens, embedding}` and supports `-backend auto|scalar|sse2|avx2|neon`, `-role raw|query|document`, `-title`, and dimensions 768/512/256/128. Unsupported CPU backends fail. Query prompts are `task: search result | query: `; document prompts are `title: <title> | text: `, with `none` as the default title. Raw mode adds no prompt. Smaller vectors use the requested prefix followed by renormalisation.

## Inference contract

- `Load(path, backend)` owns its mapping and compact tokenizer; `Close` waits for active inference. `NewModel(file, backend)` takes ownership on success. Its caller must relinquish the file and must not mutate it concurrently during construction.
- `Tokenize` performs Gemma 4 ranked BPE from GGUF vocabulary/merges. Only ASCII spaces become `▁`; newline runs are distinct pieces. Added/control tokens bypass ordinary BPE. UTF-8 is preserved without NFC/NFKC conversion; unsupported bytes use explicit `<0xXX>` pieces. Invalid UTF-8, malformed metadata, missing fallback pieces and oversize inputs return errors.
- BOS 2 and EOS 1 are included. The published embedding context is 8,192 tokens, including prompt and special tokens. The GGUF positional capacity of 262,144 does not widen that API limit. Overflow is rejected without truncation. Text admission is bounded at 1 MiB before normalisation.
- `Embed` allocates tokenisation/request output; `EmbedTokens` accepts explicit IDs and includes all of them in pooling. There is no padding or batch mask: each call handles one unpadded sequence. Callers supplying explicit IDs own BOS/EOS policy.
- Four attention heads use head dimension 256 and two KV heads in local layers; every sixth layer uses dimension 512 and one KV head. Q/K have learned RMS weights, V uses unweighted RMS; RoPE pairs the two halves. Attention is bidirectional, with scale 1.0 and inclusive local radius 512. The converter doubles the Transformers radius into GGUF's symmetric span of 1,024; both masks admit positions `abs(i-j) <= 512`.
- Attention and MLP post-normalisation/residuals, projection-only per-layer inputs, GELU gates and learned layer scalars follow the reference graph. Per-layer projection row views recompute only the current layer's input, avoiding a retained 24-layer activation cube.
- Final RMS-normalised token states are mean-pooled, then linearly projected to 768 dimensions. Projection commutes with mean pooling. The output is normalised after optional truncation. Zero/nonfinite output norms fail.
- `Workspace` is reusable and grows to a bounded high-water mark. Use separate workspaces per concurrent request; a busy one returns an error. Release it to free reserved activations. Cancellation is checked during tokenisation, projections, attention rows and before returning a vector. Errors never publish partial embeddings.
- No cross-request KV cache, global FP32 weights, runtime CGo/native library, Python, GPU, shell subprocess or network access is required. Treat the mapped file as immutable while open. External truncation is outside the loader's ownership contract.

## Independent reference

`internal/embeddinggemma/testdata/reference.json` retains 11 independent CPU vectors and token IDs, including empty input, multilingual/decomposed Unicode, whitespace, a special token, retrieval prompts, three truncated dimensions and a 1,028-token local-window case. Provenance pins llama.cpp commit `c3df4b7e22a1ac38da663552d19e649dd97ca936` and the model digest. Ordinary real-model tests read this fixture without launching another runtime.

The reference deliberately removes two llama.cpp approximations: FP16 GELU lookup and activation quantisation. Build its disposable source under `$PROJECT_TMP_ROOT/cache/reference` with Release, `GGML_NATIVE=OFF`, `GGML_CUDA=OFF`, `GGML_VULKAN=OFF`, and CPU-only execution. Apply these two changes to the pinned source:

1. In `ggml/src/ggml-cpu/vec.h`, remove `#define GGML_GELU_FP16` to use its existing direct FP32 GELU implementation.
2. In `src/llama-quant.cpp`, remove `quantize &= name.find("per_layer_model_proj") == std::string::npos;`. Otherwise conversion to F32 silently leaves that Q5_K matrix quantised and the reference still quantises its activations.

Build `llama-server` and `llama-quantize`. Run the latter with `--allow-requantize <pinned.gguf> <temporary-f32.gguf> F32 4`. Confirm **every** 2D tensor converts to F32, including `per_layer_model_proj.weight`. Optional `TestReferenceDequantizedWeights` samples independent decoded rows from every matrix when `EMBEDDINGGEMMA_F32_REFERENCE` names this file; its final maximum difference was zero.

Start two localhost servers: port 18091 with original GGUF for token IDs, and port 18092 with the dequantised F32 file. Both use `--embedding --pooling mean --ctx-size 8192 --batch-size 8192 --ubatch-size 8192 --threads 4 --n-gpu-layers 0 --parallel 1 --flash-attn off`. Flash attention can cast K/V to F16 even with F32 weights. Then:

```sh
make gemma-reference
# Deliberate regeneration only, after independently verifying the reference setup:
# bun tools/gemma-reference.mjs --write-fixture
```

The Bun checker verifies the pinned asset digest, exact token IDs, fixture drift and numerical vector errors. Maximum observed component error was `2.39e-7`; RMS was at most `6.19e-8`. Gates are RMS `3e-7`, max absolute `1e-6`, cosine at least `1 - 1e-9`, allowing several times measured FP32 reduction/rounding noise. A changed vector hash has no acceptance role.

Stop reference servers before deleting their disposable source, binaries, logs and expanded GGUF. Retain the compact test fixture and [measured conclusions](evidence/embeddinggemma-inference-2026-10-28.md). The reference uses the same quantised weights; full-precision model quality and held-out retrieval quality require separate evaluation.

## Measurements and scope

On this i7-12700 host, whole-row AVX2 Q4 and SSE2/AVX2 Q8 projection [reduced complete text-to-vector latency by about 2.2×](evidence/embeddinggemma-latency-2026-10-29.md): a 15-token query takes about 0.25 s; 69/254-token chunks take about 1.15/4.45 s. The query allocates 21,464 bytes across 217 objects including BPE tokenisation. A previous already-tokenised forward-only measurement allocated 3,072 bytes once for its returned vector; that excludes text tokenisation. Longer-workload resident memory was about 158 MiB, with about 164 MiB peak process RSS in the latest run. A shared 256 KiB half-conversion table avoids per-block conversion branches; no whole-model FP32 expansion is used. These are host observations; NAS performance, full 8,192-token resource use, ARM execution and production retrieval quality are unmeasured. Production integration and index migration require a separate decision.

Quantised layouts adapt go-pherence's immutable-matrix/caller-workspace approach; see `internal/embeddinggemma/THIRD_PARTY.md`. The unrelated go-pherence checkout and pre-existing `AGENTS.md` edit were preserved.
