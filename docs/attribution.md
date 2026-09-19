# Attribution

## Visual memory debugger

The optional visual debugger vendors [Three.js](https://threejs.org/) 0.180.0 and [Preact](https://preactjs.com/) 10.27.2, including Preact Hooks, under their MIT licences. Exact source URLs and SHA-256 digests are recorded in `src/memento/graph_debug/static/vendor/manifest.json`; the combined licence text is shipped beside the browser modules in `LICENSES.md`. `bun tools/vendor_graph_libraries.ts --check` verifies the committed files without network access.

## uMCP

Memento uses [`rcarmo/umcp`](https://github.com/rcarmo/umcp) for its MCP server, session-bound Streamable HTTP transport, request context, authentication and authorisation hooks. The Python package pins the `v0.2.2` release commit, `9c89a708d14ae804e32aa65de10af7c02922617d`, through the `mcp` optional dependency.

## Rust workspace

The Rust implementation under `rust/` includes code derived from and validated against the MIT-licensed [`rcarmo/go-gte`](https://github.com/rcarmo/go-gte) reference implementation. That attribution applies to:

* `rust/crates/memento-gte`
* `rust/crates/memento-vector`
* `rust/crates/memento-embed`
* `rust/crates/memento-sqlite-vector`
* `rust/crates/memento-ffi`

`memento-ffi` exposes the same Rust embedding and vector functionality through a stable C ABI, and keeps the same attribution chain intact.

## Go SIMD kernels

The opt-in Go SIMD dot/AXPY substrate under `go/internal/simd` is informed by the MIT-licensed [`rcarmo/go-gte`](https://github.com/rcarmo/go-gte) assembly design and retains scalar correctness fallbacks. Memento adds baseline amd64 SSE2, runtime-gated AVX2, and ARM64 NEON variants plus explicit runtime selection and differential tests. The upstream MIT licence is retained in the repository's existing attribution chain. Automatic model dispatch was enabled after scalar/corpus parity and x86-64/ARM64 architecture benchmarks passed; `MEMENTO_SIMD=scalar` retains the exact-order oracle.

## GTE-small model

The repository vendors the FP32 `gte-small.gtemodel` generated from [`thenlper/gte-small`](https://huggingface.co/thenlper/gte-small) through the `rcarmo/go-gte` conversion tooling. The file is `models/gte/gte-small.gtemodel`, is about 128 MB, and has SHA-256 `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`.

Runtime code is MIT licensed. The model artefact follows the upstream model card and repository licensing terms; release manifests must retain its source and digest.

## Needle study artefacts

The Needle feasibility and shallow-router study builds on [`cactus-compute/needle`](https://github.com/cactus-compute/needle), using upstream source commit `ffb1c51` and Hugging Face model revision `5f89b4307696d669c3df1d38ae057e6e1728b107`. The fine-tuned checkpoint, deterministic NDL1 conversion, pure-Rust inference runtime, SIMD kernels and C ABI are Memento additions; their evidence and corpora are described in [`docs/evidence/needle/README.md`](evidence/needle/README.md) and [`models/needle/README.md`](../models/needle/README.md).

Needle runtime files are release-hosted and verified through `models/runtime-models.json`. Prepare them before runtime checks with:

```bash
python3 tools/prepare_runtime_models.py
```

That prerequisite applies both to the fine-tuned checkpoint and to the family-separated train/validation/test corpora.

## Release records

`models/runtime-models.json` ties runtime model files to release-hosted archives and SHA-256 digests. Native image builds verify those files before publication, and GitHub releases record the immutable multi-architecture OCI digest. An attached SBOM remains the provenance gap tracked in [`PLAN.md`](../PLAN.md).
