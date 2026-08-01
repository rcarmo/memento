# GTE-small model artefact

`gte-small.gtemodel` is an approximately 128 MB FP32 artefact generated from the upstream `thenlper/gte-small` model for the pure-Rust runtime.

The binary is deliberately not stored in Git. A clean checkout contains only [`../runtime-models.json`](../runtime-models.json). Prepare the pinned, SHA-256-verified runtime release asset with:

```bash
python3 tools/prepare_runtime_models.py
```

Expected SHA-256:

```text
06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171
```

The file remains ignored by Git after preparation. See [`rust/docs/golden-generation.md`](../../rust/docs/golden-generation.md) for parity-fixture regeneration.
