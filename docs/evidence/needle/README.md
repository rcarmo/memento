# Needle feasibility evidence

This directory records the 2026-07-18 local AMD64 baseline used by [ADR 0002](../../decisions/0002-needle-feasibility.md).

* `base-routing-amd64.json` contains all 21 held-out queries, two fixed-seed outputs per query, parsed tool/argument fields, correctness and determinism flags, aggregate accuracy, latency and peak RSS.
* `SHA256SUMS` records the pinned Needle checkpoint and SentencePiece artefacts downloaded from Hugging Face model revision `5f89b4307696d669c3df1d38ae057e6e1728b107`.

The model files are not committed here. The study ran from an isolated temporary directory and proved offline inference by removing proxy variables, setting Hugging Face/Transformers offline flags and replacing socket connections with a guard that raises on use.

The baseline does not pass the production thresholds. It exists to size a future Memento-specific fine-tuning experiment, not to justify enabling Needle in Memento.
