# Needle feasibility evidence

This directory records the 2026-07-18 local AMD64 baseline used by [ADR 0002](../../decisions/0002-needle-feasibility.md).

* `base-routing-amd64.json` contains all 21 held-out queries, two fixed-seed outputs per query, parsed tool/argument fields, correctness and determinism flags, aggregate accuracy, latency and peak RSS.
* `SHA256SUMS` records the pinned Needle checkpoint and SentencePiece artefacts downloaded from Hugging Face model revision `5f89b4307696d669c3df1d38ae057e6e1728b107`.

The model files are not committed here. The study ran from an isolated temporary directory and proved offline inference by removing proxy variables, setting Hugging Face and Transformers offline flags, and replacing socket connections with a guard that raises on use.

The directory also contains the completed free GPU fine-tuning evidence:

* `finetune-corpus-manifest.json` records the deterministic 1,500-example corpus and SHA-256 produced by `tools/experiments/needle/generate_corpus.py`.
* `finetune-training-summary.json` records GPU settings, base/random-split metrics and the experimental checkpoint digest.
* `finetuned-original-holdout-amd64.json` reruns the unchanged baseline cases.
* `finetuned-unseen-holdout-amd64.json` covers unseen phrasing, entities, plans and safety requests.
* `router-v2-manifest.json` records the family-separated shallow-router corpus split produced by `tools/experiments/needle/generate_router_v2.py`.
* `router-v2-training-summary.json` records the shallow-router fine-tuning and hard-negative continuation settings.
* `router-v2-heldout-amd64.json` records the untouched family-separated AMD64 routing and abstention gate.

The first full-plan checkpoint is not committed because it does not pass the integration thresholds. The later shallow-router experiment is recorded in the `router-v2-*` files and passes the untouched family-separated AMD64 routing/abstention gate after a targeted hard-negative continuation.

The passing checkpoint and family-separated train/validation/test corpora are vendored through Git LFS under [`models/needle/`](../../../models/needle/README.md). Install and fetch LFS objects before using them:

```bash
git lfs install
git lfs pull
```

Passing the model-quality gate does not enable Needle in Memento. Runtime integration remains blocked on an embedded/Cactus implementation with offline artefacts, cancellation and AMD64/ARM64 parity.
