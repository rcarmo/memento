# Attribution

## uMCP

Memento uses [`rcarmo/umcp`](https://github.com/rcarmo/umcp) for its MCP server, Streamable HTTP transport, request context, authentication and authorization hooks. The Python package pins commit `691af9f159757d45c180856ec0dfb89da7aa341c` through the `mcp` optional dependency.

## Rust workspace

The Rust implementation under `rust/` includes code derived from and validated against the MIT-licensed [`rcarmo/go-gte`](https://github.com/rcarmo/go-gte) reference implementation. That attribution applies to:

* `rust/crates/memento-gte`
* `rust/crates/memento-vector`
* `rust/crates/memento-embed`
* `rust/crates/memento-sqlite-vector`
* `rust/crates/memento-ffi`

`memento-ffi` exposes the same Rust embedding and vector functionality through a stable C ABI, and keeps the same attribution chain intact.

## GTE-small model

The repository vendors the FP32 `gte-small.gtemodel` generated from [`thenlper/gte-small`](https://huggingface.co/thenlper/gte-small) through the `rcarmo/go-gte` conversion tooling. The file is `models/gte/gte-small.gtemodel`, is about 128 MB, and has SHA-256 `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`.

Runtime code is MIT licensed. The model artefact follows the upstream model card and repository licensing terms; release manifests must retain its source and digest.

## Needle study artefacts

The Needle feasibility and shallow-router study builds on [`cactus-compute/needle`](https://github.com/cactus-compute/needle), using upstream source commit `ffb1c51` and Hugging Face model revision `5f89b4307696d669c3df1d38ae057e6e1728b107`. The fine-tuned checkpoint, deterministic NDL1 conversion, pure-Rust inference runtime, SIMD kernels and C ABI are Memento additions; their evidence and corpora are described in [`docs/evidence/needle/README.md`](evidence/needle/README.md) and [`models/needle/README.md`](../models/needle/README.md).

The vendored `models/needle/` files are tracked with Git LFS. Operators and reviewers need Git LFS installed before using them:

```bash
git lfs install
git lfs pull
```

That prerequisite applies both to the fine-tuned checkpoint and to the family-separated train/validation/test corpora.

## Pending evidence

Source provenance is documented here. Additional release-time evidence, such as shipped artefact manifests tying those crates and models to published binaries, remains pending.
