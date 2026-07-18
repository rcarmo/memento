# Needle feasibility evidence

This directory records the 2026-07-18 local AMD64 baseline used by [ADR 0002](../../decisions/0002-needle-feasibility.md).

* `base-routing-amd64.json` contains all 21 held-out queries, two fixed-seed outputs per query, parsed tool/argument fields, correctness and determinism flags, aggregate accuracy, latency and peak RSS.
* `SHA256SUMS` records the pinned Needle checkpoint and SentencePiece artefacts downloaded from Hugging Face model revision `5f89b4307696d669c3df1d38ae057e6e1728b107`.

The model files are not committed here. The study ran from an isolated temporary directory and proved offline inference by removing proxy variables, setting Hugging Face/Transformers offline flags and replacing socket connections with a guard that raises on use.

The directory also contains the completed free GPU fine-tuning evidence:

* `finetune-corpus-manifest.json` records the deterministic 1,500-example corpus and SHA-256.
* `finetune-training-summary.json` records GPU settings, base/random-split metrics and the experimental checkpoint digest.
* `finetuned-original-holdout-amd64.json` reruns the unchanged baseline cases.
* `finetuned-unseen-holdout-amd64.json` covers unseen phrasing, entities, plans and safety requests.

The fine-tuned checkpoint is not committed because it does not pass the integration thresholds. The base and fine-tuned studies exist to size future work, not to justify enabling Needle in Memento.
