# Needle shallow-router artefacts

This directory documents the shallow router described in [ADR 0002](../../docs/decisions/0002-needle-feasibility.md). Corpus generation, training, hard-negative continuation and checkpoint conversion are documented in [`docs/needle-fine-tuning.md`](../../docs/needle-fine-tuning.md).

The binaries and corpora are deliberately not stored in Git.

## Runtime files

A clean checkout contains only [`../runtime-models.json`](../runtime-models.json). Prepare the pinned, SHA-256-verified runtime release asset with:

```bash
python3 tools/prepare_runtime_models.py
```

This provides:

* `memento-router.ndl` — SHA-256 `fc9978c1d3817031a3f9ea00832cd8177290b25ff734b178cb9bcba0b894bb0b`;
* `needle.model` — SHA-256 `0823f5b9133c68a8140addc5d7a425fa9119c4c8cb4a550363b4bffa4ba1c8c7`.

## Training files

The separately pinned `training-assets-v1` GitHub release contains `needle-training-assets-v1.tar` with SHA-256 `7c723be0babdac2ff11a735d6feea2eef1d61622967a32777f2fd53dc826db4c`. It includes:

* `memento-router.pkl` — passing checkpoint, SHA-256 `969bf020dce5075e8043ec88386d2ffd192297d307f34bcddbd435156ba205a8`;
* `needle.vocab`;
* `train.jsonl`, `val.jsonl`, `test.jsonl`, and `train-hard.jsonl`.

Download that release asset only when reproducing training or evaluation. All extracted files are ignored by Git.

## Provenance and status

The deterministic splits come from `tools/experiments/needle/generate_router_v2.py`; the hard-negative continuation is recorded in ADR 0002 and `router-v2-training-summary.json`. The scalar and AVX2/FMA runtimes match the passing checkpoint on all 360 untouched AMD64 cases. The C ABI and Python wrapper include bounded output, lifecycle checks and cooperative cancellation. ARM64 correctness is covered by portable/NEON paths but still needs hardware performance evidence.
