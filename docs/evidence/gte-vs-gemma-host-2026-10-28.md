# GTE-small versus EmbeddingGemma 2 on this host

GTE-small was 12–35 times faster on three representative texts. Gemma retrieved the correct document for all 16 diagnostic queries at 768, 512 and 256 dimensions; GTE retrieved 14, missing two cross-language queries. The set is small and authored for this experiment, so its scores cannot establish production retrieval quality.

## Method

Source: `a53fc39` plus `tools/embedding-compare`. Host: Linux amd64, Intel Core i7-12700, six visible logical CPUs (affinity 0–5), Go 1.27.1, pure-Go builds, automatic AVX2 dispatch. `GOMAXPROCS=1`; each process handles one embedding at a time. No GPU, external inference runtime, production request or deployment change was involved. Other host workloads were not suspended and the process was not pinned to one core.

Models:

| | GTE-small | EmbeddingGemma 2 UD-Q4_K_XL |
|---|---:|---:|
| Checkpoint bytes | 133,702,058 | 175,673,856 |
| Layers | 12 | 24 |
| Hidden width | 384 | 512 |
| Output dimensions | 384 | 768; also 512/256/128 |
| Input limit incl. special tokens | 512 | 8,192 |
| Weight storage | FP32 copies in Go heap | read-only mixed-quantisation GGUF mapping |

GTE checkpoint SHA-256: `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`. Gemma: `ea905fd08e8061db77a0031cbb5096d459d7cfd2bdd0ad2ba66e483f16719493`.

Inputs, relevance labels and repeat counts were written before the first run in `tools/embedding-compare/corpus.json`, SHA-256 `26e2d182f62db007abe6d7d2d9e5b4cfc7d34dfd8d46fb2300b5e8adefe98094`. No documents, labels or queries were revised after inspecting results. Performance inputs contain 35, 406 and 1,516 Unicode characters. GTE receives plain text; Gemma receives its recommended query/document prefix. Retrieval documents include the same title/content information in each model's input. Both return owned normalised vectors and reuse their native request workspaces. Timing includes tokenisation, prompting and forward inference, excludes model loading and omits worker IPC/index lookup.

First pass: GTE then Gemma, with CPU and allocation profiles. Repeat: Gemma then GTE, without profiles. A separate Gemma retrieval-only run checked its supported 512-dimensional output prefix. Checkpoint digests were read before timing, warming filesystem pages. Load/first-call measurements are new-process measurements with warm filesystem data; they are not cold-disk measurements.

## Warm latency

Unprofiled repeat medians, after a warm-up for each length:

| Source text | GTE / Gemma tokens | GTE median | Gemma median | Gemma / GTE | Samples each |
|---|---:|---:|---:|---:|---:|
| Short query | 8 / 15 | 15.57 ms | 540.56 ms | 34.7× | 7 |
| Paragraph | 69 / 69 | 145.32 ms | 2,521.06 ms | 17.3× | 5 |
| Longer chunk | 262 / 254 | 818.20 ms | 9,621.04 ms | 11.8× | 3 |

Profiled first-pass medians were 15.78/539.08 ms, 145.01/2,504.98 ms and 840.86/9,630.04 ms respectively. The reverse-order repeat preserved the gap. None of these texts reached GTE's input cap; the comparison does not hide a truncation/length mismatch. Gemma's prefix adds proportionally more work to short queries.

Generating all 18 document and 16 query embeddings in the profiled run took 2.57 s for GTE and 49.48 s for Gemma, a 19.3× difference. These calls were sequential, without production worker overhead. Truncating Gemma output vectors saves index storage but retains all transformer work; no shorter-dimension latency benefit was measured.

## Load and memory

Unprofiled fresh processes, with warm filesystem data:

| Metric | GTE | Gemma |
|---|---:|---:|
| Model load | 101.45 ms | 209.94 ms |
| First short embedding after load | 15.76 ms | 540.14 ms |
| Retained Go heap after load + forced GC | 129.5 MiB | 52.8 MiB |
| RSS after longer workload | 147.6 MiB | 157.1 MiB |
| Peak process RSS | 262.6 MiB | 163.6 MiB |

`/usr/bin/time` independently reported peaks of 268,928 KiB and 167,552 KiB. The profiled full run peaked at 269,440 and 174,920 KiB. Gemma's smaller heap excludes mapped weight pages; RSS includes touched mapped pages. GTE's higher peak comes from reading the file into a byte slice and copying FP32 weights into model arrays before the temporary file buffer is collected. GTE's steady resident memory was lower on this workload despite its higher startup peak. Maximum-context memory, concurrent requests, NAS memory and cold-storage I/O were not measured.

Text-to-vector allocation totals in the unprofiled repeat:

| Source text | GTE bytes / call | Gemma bytes / call | GTE objects / call | Gemma objects / call |
|---|---:|---:|---:|---:|
| Query | ~1,569 | 21,464 | ~3 | 217 |
| Paragraph | 1,592 | 140,312 | 7 | 1,466 |
| Longer chunk | 1,640 | 474,584 | 13 | 5,224 |

These include tokenisation and the owned output vector. Gemma's previously measured one-allocation forward pass used already-tokenised IDs and a reused workspace; that narrower measurement excludes BPE symbols/heap merges. Small runtime-accounting fractions in sample averages are not literal fractional objects per call.

## Retrieval sanity set

18 English documents cover backup/restore, snapshot/retention, vector dimensions/input windows, CPU/GPU, credentials/TLS/network, semantic/lexical indexes, profiling and customer-delivery tasks. There are 12 English queries and four Portuguese/Spanish/French/Japanese queries, each with one labelled relevant document. Similarity is cosine. Every model builds its own document index; vectors from different models are never compared directly.

| Model / output dimensions | Recall@1 | Recall@3 | MRR | English top-1 | Cross-language top-1 |
|---|---:|---:|---:|---:|---:|
| GTE / 384 | 14/16 (87.5%) | 15/16 (93.75%) | 0.9125 | 12/12 | 2/4 |
| Gemma / 768 | 16/16 | 16/16 | 1.0000 | 12/12 | 4/4 |
| Gemma / 512 | 16/16 | 16/16 | 1.0000 | 12/12 | 4/4 |
| Gemma / 256 | 16/16 | 16/16 | 1.0000 | 12/12 | 4/4 |
| Gemma / 128 | 15/16 (93.75%) | 16/16 | 0.96875 | 11/12 | 4/4 |

GTE ranked the Portuguese database-recovery query's correct document tenth, choosing meeting notes first. The Japanese CPU-only question ranked its correct document second behind GPU training. Gemma at 128 dimensions ranked the missing-semantic-vectors document second behind lexical search. All other labelled top results were correct. A 384-dimensional Gemma prefix was initially explored but is excluded from the supported-output table; the follow-up used 512 dimensions.

A 256-dimensional FP32 Gemma vector occupies 1,024 bytes versus GTE's 1,536 bytes, before index overhead: one-third less vector storage. This result is promising for multilingual testing, but 16 manually authored queries provide neither statistical confidence nor coverage of the real corpus. No production relevance labels, long-document retrieval, reranker, approximate-nearest-neighbour index or full-precision Gemma checkpoint was tested.

## Diagnostic profile findings

GTE's CPU profile captured 6.94 s: AVX2 row-dot kernel 44.81% flat; transformer loop 28.10%; exponentiation 10.66%. Gemma captured 107.31 s: Q4 AVX2 group kernel 28.74% flat; fused Q4 dispatch/scale work 27.76%; remaining row decoding 19.13%. Gemma's gap is dominated by transformer/quantised projection computation, not JSON, tokenisation or output dimensions. Both inference paths were CPU-bound.

Sampled allocation profiles recorded 262.11 MiB for GTE, dominated by the file byte slice plus FP32 weight copies; Gemma recorded 73.58 MiB, dominated by GGUF/tokenizer maps and metadata parsing. Object profiles showed Gemma parser strings and BPE queue operations; GTE's objects were mainly initial vocabulary/model state. Profiles were analysed as diagnostics of the fixed comparison baseline. No model/kernel tuning or production changes were made during this comparison.

## Reproduction and cleanup

```sh
. tools/project-env.sh
run="$PROJECT_TMP_ROOT/runs/embedding-compare"
mkdir -p "$run"
CGO_ENABLED=0 go build -o "$run/compare" ./tools/embedding-compare
"$run/compare" -engine gte -model models/gte/gte-small.gtemodel
"$run/compare" -engine gemma -model models/embeddinggemma/embeddinggemma-2-UD-Q4_K_XL.gguf
# Optional diagnostic captures, removed immediately after analysis:
# add -profiles "$run/profiles"
# -mode performance skips retrieval; -mode retrieval skips timed workloads.
```

The corpus/metric unit tests, race tests and vet passed. CPU, alloc_space and alloc_objects captures were reviewed. Raw profiles, matching binaries and disposable JSON/timing outputs were removed after analysis; the comparison code, labelled corpus and this report remain. Production continues using GTE. Next evaluate multilingual retrieval on an independently labelled representative corpus and benchmark the NAS before deciding on an alternate worker or migration.
