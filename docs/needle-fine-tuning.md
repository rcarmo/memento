# Fine-tuning the Needle router

Memento uses a fine-tuned 26M-parameter [`cactus-compute/needle`](https://github.com/cactus-compute/needle) model for shallow read routing. This document records how the shipped checkpoint was produced and which parts can be reproduced from this repository.

## Pinned inputs

* Needle source: commit `ffb1c51`
* Base model revision: `5f89b4307696d669c3df1d38ae057e6e1728b107`
* Tokenizer SHA-256: `0823f5b9133c68a8140addc5d7a425fa9119c4c8cb4a550363b4bffa4ba1c8c7`
* Training GPU: NVIDIA RTX 3060 12 GB
* Corpus seed: `20260718`

The base checkpoint and tokenizer came from the upstream Needle model revision. Network access was disabled during evaluation after those files were downloaded.

## Why there were two attempts

The first model generated complete `memory_execute` plans. It improved on Needle's random train/test split, but failed the unchanged and unseen-family tests: unsupported requests were often routed instead of rejected, and generated plans copied or invented paths badly enough to miss the release thresholds. That checkpoint is not shipped.

The second model only chooses one of six shallow actions:

```text
search_then_read
search_paths
status_field
search_then_graph
read_field
UNKNOWN
```

Memento fills in paths, IDs, search text and plans in ordinary code. This reduced the model's job to classification and made `UNKNOWN` useful as a hard stop.

## Generate the corpus

The shallow-router corpus is generated without a model API:

```bash
python tools/experiments/needle/generate_router_v2.py
```

By default this writes to `/tmp/needle-study/`:

```text
router-v2-train.jsonl
router-v2-val.jsonl
router-v2-test.jsonl
router-v2-manifest.json
```

The splits use different entity families and path shapes:

| Split | Examples | Examples per action | Entities |
|---|---:|---:|---|
| train | 1,440 | 240 | Alder through Lotus |
| validation | 360 | 60 | Maple, Nectar, Olive, Pine |
| test | 360 | 60 | Quartz, Reef, Spruce, Thyme |

The reviewed read-field order is `title`, `path`, `status`, `tags`, `body`, `type`; status fields are `repo_revision`, `index_revision`, `index_stale`, `semantic_search_ready`, `visible_concepts`, `proposal_backlog`. These values and their ordering are part of the corpus definition because field assignment depends on list position.

The generated hashes must match [`router-v2-manifest.json`](evidence/needle/router-v2-manifest.json). The reviewed files are stored through Git LFS under [`models/needle/`](../models/needle/README.md).

## Train and continue with hard negatives

The passing run used Needle's upstream training code at commit `ffb1c51`:

1. Load the pinned base checkpoint and `needle.model` tokenizer.
2. Train on `train.jsonl` for four epochs with batch size 16.
3. Evaluate against `val.jsonl` and the untouched `test.jsonl`.
4. Keep validation and test files unchanged.
5. Add 288 training-only direct-mutation hard negatives.
6. Continue for one epoch on the resulting 1,728-example `train-hard.jsonl`.
7. Evaluate the same 360 test rows again.

The exact upstream trainer command was not preserved in this repository. The settings and output hashes are recorded in [`router-v2-training-summary.json`](evidence/needle/router-v2-training-summary.json). Reproduction therefore requires the upstream Needle checkout and its training environment; the Memento repository supplies the corpus, split hashes, tokenizer, final checkpoint and evaluation records.

The earlier full-plan run used two epochs, batch size 32 and Needle's default optimiser settings on a 1,500-example mixed corpus. Its generator is `tools/experiments/needle/generate_corpus.py`; the run is documented in [`finetune-training-summary.json`](evidence/needle/finetune-training-summary.json).

## Acceptance checks

The final shallow-router checkpoint is tested on the untouched family-separated set:

| Check | Result |
|---|---:|
| Tool decision accuracy | 360/360 |
| Valid call shape | 100% |
| UNKNOWN recall | 100% |
| False action rate on UNKNOWN cases | 0% |
| Scalar/Rust SIMD decision parity | 360/360 |

Argument exact match is not a release criterion. The model's free-form arguments are treated as hints; Memento derives or validates the values it uses.

Per-case outputs are in [`router-v2-heldout-amd64.json`](evidence/needle/router-v2-heldout-amd64.json).

## Convert the checkpoint for the embedded runtime

Convert the reviewed pickle checkpoint to NDL1:

```bash
python tools/experiments/needle/convert_needle.py \
  models/needle/needle.model \
  --input models/needle/memento-router.pkl \
  --output models/needle/memento-router.ndl \
  --manifest docs/evidence/needle/memento-router-ndl1.json
```

The converter stores bf16-rounded tensors, model configuration, tokenizer metadata, section hashes and a tensor directory. Expected final hashes:

```text
memento-router.pkl  969bf020dce5075e8043ec88386d2ffd192297d307f34bcddbd435156ba205a8
memento-router.ndl  fc9978c1d3817031a3f9ea00832cd8177290b25ff734b178cb9bcba0b894bb0b
needle.model        0823f5b9133c68a8140addc5d7a425fa9119c4c8cb4a550363b4bffa4ba1c8c7
```

Run the Rust tests after conversion:

```bash
make rust-check
```

The embedded runtime, FFI and single-core benchmark are covered in [`docs/needle-performance.md`](needle-performance.md) and [ADR 0002](decisions/0002-needle-feasibility.md).
