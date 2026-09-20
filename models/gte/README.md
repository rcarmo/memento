# GTE-small model artefact

`gte-small.gtemodel` is an approximately 128 MB FP32 artefact generated from the upstream `thenlper/gte-small` model. The same GTE1 bytes used by the previous Rust runtime are loaded directly by the pure-Go `v1.0.0` embedder.

The binary is deliberately not stored in Git. A clean checkout contains only [`../runtime-models.json`](../runtime-models.json). Prepare the pinned, SHA-256-verified runtime release asset with:

```bash
python3 tools/prepare_runtime_models.py
```

Expected SHA-256:

```text
06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171
```

The file remains ignored by Git after preparation. Its accepted digest and source bundle are recorded in [`models/runtime-models.json`](../runtime-models.json); committed Go parity fixtures retain the historical reference outputs.
