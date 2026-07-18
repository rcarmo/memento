# ADR 0002: Needle is an evaluation candidate, not a production model

**Status:** accepted  
**Date:** 2026-07-18

## Question

Can [`cactus-compute/needle`](https://github.com/cactus-compute/needle) replace remote or general-purpose LLM processing for Memento while remaining fully local and embedded?

## Decision

Do not add Needle to Memento's production model slots yet. Keep the deterministic core, GTE-small retrieval and existing optional model-provider boundary unchanged.

Needle is worth a separate Memento-specific fine-tuning experiment for compact tool routing and bounded `memory_execute` plan drafting. It must clear the acceptance thresholds below before integration is reconsidered. Proposal and Dream drafting remain out of scope until routing, abstention and strict-schema results are strong enough.

## What was tested

The study pinned:

* Needle source commit `ffb1c51`.
* Hugging Face model revision `5f89b4307696d669c3df1d38ae057e6e1728b107`.
* `needle.pkl`, SHA-256 `40a32e91d1d4197bf15ba559b74f6727c342dc8746918742fc7d8e2c1f18df40`, 52,633,098 bytes.
* `needle.model`, SHA-256 `0823f5b9133c68a8140addc5d7a425fa9119c4c8cb4a550363b4bffa4ba1c8c7`, 124,960 bytes.
* `needle.vocab`, SHA-256 `7cf61fdf69759a8b6128da7724c3e6541a7671658de34a92b969c97aae700e75`.

Inference ran on Linux x86_64 with six visible CPUs on an Intel Core i7-12700. The process loaded local artefacts with Hugging Face and Transformers offline flags set, proxy variables removed and socket connections replaced by a guard that raises immediately. A constrained `memory_status` call completed successfully, proving that inference needs no API call once all artefacts are present.

The evaluation exposed the five compact Memento tools plus an explicit `UNKNOWN` tool:

* `memory_help`
* `memory_status`
* `memory_search`
* `memory_read`
* `memory_execute`
* `UNKNOWN`

Twenty-one held-out queries covered help, readiness, search, read, compound plans and unsupported or unsafe requests. Every case ran twice with seed zero.

## Baseline results

| Measure | Result | Feasibility threshold |
|---|---:|---:|
| Tool accuracy | 38.10% | >=97% |
| Valid JSON/tool-call shape | 85.71% | >=99% |
| Repeat determinism | 100% | 100% |
| UNKNOWN precision | 0% | >=95% |
| Median warm latency | 1.058 s | to be set after runtime choice |
| Maximum observed latency | 2.943 s | to be set after runtime choice |
| Peak RSS | 1,207.9 MB | substantially below current JAX result |
| First two-tool call after load | 2.006 s | informational |
| Subsequent two-tool call | 0.660 s | informational |
| Checkpoint/tokenizer load | 0.205 s | informational |

The base model handled direct search and read requests reasonably, but confused help/readiness, reduced compound requests to a single search and never selected `UNKNOWN`. Some constrained outputs were still not valid JSON.

The complete per-case output is stored in [`docs/evidence/needle/base-routing-amd64.json`](../evidence/needle/base-routing-amd64.json).

## Embeddability findings

Needle is small at the model level--26.3 million parameters and a 53 MB checkpoint--but the checked-in Python/JAX inference path is not an embedded runtime:

* The isolated environment occupied 1.1 GB and contained 61 packages.
* Importing generation pulled in `tqdm` and Hugging Face `datasets`, even though inference did not need training data.
* The default UI uses `hf_hub_download(..., force_download=True)` for checkpoints and tokenizer files.
* JAX compilation dominated the first call and peak RSS reached 1.2 GB during repeated evaluation.
* The repository does not provide a stable C ABI, Cactus binding or embedded artefact manifest.

A production candidate would need a split inference-only package or a verified Cactus library path with pinned local assets, no telemetry/download code, bounded cancellation and explicit AMD64/ARM64 evidence.

## Constrained-output limits

Needle's constrained decoder restricts tool names and top-level argument keys. It deliberately leaves argument values unconstrained, including nested objects and arrays. This is useful for ordinary function calling, but it cannot guarantee a valid nested `memory_execute` plan, proposal draft or Dream draft.

Memento would still validate every result, but the base model's 85.71% valid-call rate and zero abstention make that validation path too noisy for production use.

## Fine-tuning experiment

A free local fine-tune was completed on an NVIDIA RTX 3060 12 GB using a deterministic 1,500-example corpus: 180 help, 180 status, 240 search, 240 read, 360 execute and 300 UNKNOWN cases. No model API generated the data. Training used the pinned base checkpoint, two epochs, batch size 32, BF16 and Needle's default optimiser settings.

Needle's built-in random per-tool split used 1,380 train, 60 validation and 60 test examples. It improved from 21.67% to 91.67% exact match, 42.28% to 98.33% name F1 and reached 100% parse rate. Those figures are useful training evidence but are not acceptance evidence because paraphrase families and slot patterns crossed the random split.

Two leakage-resistant checks remained below threshold:

| Held-out set | Routing | JSON validity | UNKNOWN | Execute plans |
|---|---:|---:|---:|---:|
| Original unchanged 21 cases | 85.71% | 100% | 80% precision | 2/3 routed; routed plans valid |
| New unseen-family 28 cases | 75.00% | 96.43% | 62.50% recall; 37.50% false actions | 50% routed; 0% valid across all expected plans |

Peak RSS on the CUDA/JAX evaluation path was about 2.29 GB and median unseen-holdout latency was 0.626 s. The fine-tuned checkpoint is not shipped because it fails the safety and plan-validity gates.

Evidence is under [`docs/evidence/needle/`](../evidence/needle/). The deterministic corpus generator is `tools/experiments/needle/generate_corpus.py`.

A later follow-up may repeat fine-tuning with strict family-separated train/validation files, stronger hard negatives and grammar-constrained nested plans. The minimum held-out set should cover:

* 700 compact routing examples split across help, status, search, read, execute, answer and UNKNOWN.
* 600 bounded `memory_execute` plans, including safe references, return projections and one-commit enforcement.
* 300 unsupported, ambiguous, forbidden and prompt-injected requests for abstention calibration.

Proposal and Dream examples should only be added after those gates pass.

The go/no-go thresholds are:

* routing accuracy >=97%;
* routing macro-F1 >=95%;
* strict call/schema validity >=99%;
* executable `memory_execute` success >=95%;
* UNKNOWN precision >=95% and recall >=90%;
* false action rate on UNKNOWN cases <=2%;
* 100% compliance with one-commit, forbidden-path and no-direct-write rules.

Results must be repeated on AMD64 and ARM64. Claims about Cactus throughput require running the exact fine-tuned checkpoint through a pinned Cactus runtime; the Needle repository's published Cactus figures do not prove Memento workload performance.

## Shallow-router follow-up

A second experiment stopped asking Needle to generate nested plans or copy authoritative slots. It classified six shallow actions: `search_then_read`, `search_paths`, `status_field`, `search_then_graph`, `read_field` and `UNKNOWN`. Memento expands those actions deterministically; the model never supplies references or commit operations.

The corpus used explicit family-separated files with disjoint entities and path shapes: 1,440 training, 360 validation and 360 untouched test examples, balanced across all six actions. Four epochs at batch 16 reached 99.3% tool-name F1, but five test cases from one direct-mutation family still truncated instead of abstaining. A one-epoch continuation added 288 training-only direct-mutation hard negatives without changing validation or test data.

The unchanged 360-case test then produced:

| Measure | Result | Gate |
|---|---:|---:|
| Routing accuracy | 100% | >=97% |
| Valid call shape | 100% | >=99% |
| Non-UNKNOWN routing | 100% | >=97% |
| UNKNOWN recall | 100% | >=90% |
| False action rate | 0% | <=2% |
| Median latency | 0.442 s | informational |
| p95 latency | 0.579 s | informational |

Argument exact match remained 54.17%, confirming that Needle should classify intent and fixed enums only. Memento must derive search text from the original request, parse exact paths/IDs and expand fixed plans in deterministic code. `src/memento/router.py` freezes and tests that boundary without adding a JAX dependency.

The passing checkpoint and family-separated corpora are vendored through Git LFS under `models/needle/`. The checkpoint SHA-256 is `969bf020dce5075e8043ec88386d2ffd192297d307f34bcddbd435156ba205a8`.

## Updated decision

The shallow router passes the AMD64 held-out routing and abstention gates. It remains disabled in production because the available JAX environment is 1.1 GB and peaked above 2 GB RSS. Integration now depends on proving the exact checkpoint through a pinned embedded/Cactus runtime on AMD64 and ARM64, with cancellation, offline artefacts and equivalent outputs.

## Consequences

Memento keeps working exactly as before. GTE-small remains the local retrieval model. Existing completion-model slots remain optional, and no remote API is required by the deterministic service.

Needle remains a promising specialist for local orchestration after fine-tuning, not a general answer model and not a source of authority. Even if adopted later, deterministic Memento code will continue to own authorisation, path validation, proposal review, Git publication, indexing and completion claims.
