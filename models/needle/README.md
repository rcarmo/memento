# Needle shallow-router artefacts

This directory holds the Git LFS artefacts for the passing shallow-router follow-up described in [ADR 0002](../../docs/decisions/0002-needle-feasibility.md).

## Git LFS prerequisite

These files are tracked with Git LFS. Install LFS and fetch the real objects before you try to inspect, hash or use them:

```bash
git lfs install
git lfs pull
```

## Files

* `memento-router.pkl` -- the passing fine-tuned shallow-router checkpoint. ADR 0002 records SHA-256 `969bf020dce5075e8043ec88386d2ffd192297d307f34bcddbd435156ba205a8`.
* `memento-router.ndl` -- deterministic `NDL1` conversion with bf16-rounded tensors, tokenizer metadata, section hashes and tensor directory for the pure Rust runtime. SHA-256: `fc9978c1d3817031a3f9ea00832cd8177290b25ff734b178cb9bcba0b894bb0b`.
* `needle.model` and `needle.vocab` -- pinned SentencePiece artefacts used by the Rust tokenizer. The model SHA-256 is `0823f5b9133c68a8140addc5d7a425fa9119c4c8cb4a550363b4bffa4ba1c8c7`.
* `train.jsonl` -- family-separated training corpus for the shallow-router study.
* `val.jsonl` -- family-separated validation corpus.
* `test.jsonl` -- untouched family-separated held-out corpus used for the routing and abstention gate.
* `train-hard.jsonl` -- additional training-only hard negatives used for the one-epoch continuation after the first shallow-router run still emitted false actions for direct-mutation prompts.

## Provenance

`train.jsonl`, `val.jsonl` and `test.jsonl` come from `tools/experiments/needle/generate_router_v2.py`, which writes the deterministic `router-v2-*.jsonl` splits under `/tmp/needle-study/` before they are reviewed and vendored here.

`train-hard.jsonl` is the targeted hard-negative continuation set referenced by ADR 0002 and the `router-v2-training-summary.json` evidence.

The earlier 1,500-example mixed routing/plan/UNKNOWN corpus is a different experiment. Its generator is `tools/experiments/needle/generate_corpus.py`, and its manifest and training evidence live under [`docs/evidence/needle/`](../../docs/evidence/needle/README.md).

## Status

These artefacts support the embedded pure-Rust router. The scalar reference and AVX2/FMA runtime produce the same decisions as the passing checkpoint on all 360 untouched AMD64 cases. The dedicated C ABI and Python wrapper include bounded output, lifecycle checks and cooperative cancellation. ARM64 correctness is covered by the portable/NEON code paths but still needs hardware performance evidence.
