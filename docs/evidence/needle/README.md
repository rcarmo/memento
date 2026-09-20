# Needle feasibility reports

This directory records the 2026-07-18 local AMD64 baseline and retained training/corpus evidence used by [ADR 0002](../../decisions/0002-needle-feasibility.md).

* `base-routing-amd64.json` contains all 21 held-out queries, two fixed-seed outputs per query, parsed tool/argument fields, correctness and determinism flags, aggregate accuracy, latency and peak RSS.
* `SHA256SUMS` records the pinned Needle checkpoint and SentencePiece artefacts downloaded from Hugging Face model revision `5f89b4307696d669c3df1d38ae057e6e1728b107`.

The model files are not committed here. The study ran from an isolated temporary directory and proved offline inference by removing proxy variables, setting Hugging Face and Transformers offline flags, and replacing socket connections with a guard that raises on use.

The directory also contains records from the free GPU fine-tuning run:

* `finetune-corpus-manifest.json` records the deterministic 1,500-example corpus and SHA-256 produced by `tools/experiments/needle/generate_corpus.py`.
* `finetune-training-summary.json` records GPU settings, base/random-split metrics and the experimental checkpoint digest.
* `finetuned-original-holdout-amd64.json` reruns the unchanged baseline cases.
* `finetuned-unseen-holdout-amd64.json` covers unseen phrasing, entities, plans and safety requests.
* `router-v2-manifest.json` records the family-separated shallow-router corpus split produced by `tools/experiments/needle/generate_router_v2.py`.
* `router-v2-training-summary.json` records the shallow-router fine-tuning and hard-negative continuation settings.
* `router-v2-heldout-amd64.json` records the untouched family-separated AMD64 routing and abstention gate.
* `memento-router-ndl1.json` records the deterministic NDL1 conversion, section hashes and tensor inventory.
* `rust-router-amd64.json` records scalar/SIMD decision parity and the initial native runtime timing.
* `rust-router-single-core-i7-12700.json` records a release build pinned to one Intel Core i7-12700 logical CPU across all 360 untouched cases, including cold start, warm latency, throughput and peak RSS.

The first full-plan checkpoint is not committed because it does not pass the integration thresholds. The later shallow-router experiment is recorded in the `router-v2-*` files and passes the untouched family-separated AMD64 routing/abstention gate after a targeted hard-negative continuation.

The passing checkpoint and family-separated corpora are pinned in the `training-assets-v1` release. The current pure-Go runtime files are pinned in `models/runtime-models.json`. Prepare them with:

```bash
python3 tools/prepare_runtime_models.py
```

Needle is enabled with `intelligent_tiers.needle_router.enabled` and remains off by default. Release images convert the pinned NDL1 weights into a deterministic little-endian FP32 sidecar. The default `subprocess` mode starts `memento-needle-go`, maps that sidecar read-only, loads the pinned SentencePiece tokenizer, returns one bounded framed response and exits. Explicit `in_process` mode loads NDL1 directly for diagnostics and parity work. The historical Rust C ABI and Python wrapper remain reference material only.

Native amd64 and ARM64 model gates pass, including all 360 held-out outputs/errors under NEON. Current CPU, allocation and mapped-worker measurements are recorded in [`../go-real-model-simd-2026-09-19.json`](../go-real-model-simd-2026-09-19.json) and the [final Go validation report](../go-v1-validation-2026-09-19.md).
