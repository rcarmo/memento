# Attribution

## Visual memory debugger

The optional visual debugger vendors [Three.js](https://threejs.org/) 0.180.0 and [Preact](https://preactjs.com/) 10.27.2, including Preact Hooks, under their MIT licences. Exact source URLs and SHA-256 digests are recorded in `go/service/graph_static/vendor/manifest.json`; the combined licence text is embedded beside the browser modules in `LICENSES.md`.

## uMCP

Memento's standalone pure-Go uMCP module under `go/umcp` implements the MCP server, session-bound Streamable HTTP transport, request context and protocol helpers. Its observable behaviour is pinned and differentially verified against [`rcarmo/umcp`](https://github.com/rcarmo/umcp) `v0.2.2` / deployed commit `9c89a708d14ae804e32aa65de10af7c02922617d`. The Python package is a test oracle only and is not shipped.

## Go SIMD kernels

The opt-in Go SIMD dot/AXPY substrate under `go/internal/simd` is informed by the MIT-licensed [`rcarmo/go-gte`](https://github.com/rcarmo/go-gte) assembly design and retains scalar correctness fallbacks. Memento adds baseline amd64 SSE2, runtime-gated AVX2, and ARM64 NEON variants plus explicit runtime selection and differential tests. The upstream MIT licence is retained in the repository's existing attribution chain. Automatic model dispatch was enabled after scalar/corpus parity and x86-64/ARM64 architecture benchmarks passed; `MEMENTO_SIMD=scalar` retains the exact-order oracle.

## Deferred Vulkan dependency

The retained NAS compatibility recipe uses the ARM-proprietary/Mesa userspace delivered by Debian Bookworm packages and a pinned Debian base-image digest. The exact tested versions, device/group mapping and limitations are recorded in [`docs/evidence/vulkan-nas-bookworm-2026-09-19.md`](evidence/vulkan-nas-bookworm-2026-09-19.md). That report covers the separate Rust/wgpu experiment; no Vulkan code or binary is part of the accepted Go baseline.

## GTE-small model

The repository vendors the FP32 `gte-small.gtemodel` generated from [`thenlper/gte-small`](https://huggingface.co/thenlper/gte-small) through the `rcarmo/go-gte` conversion tooling. The file is `models/gte/gte-small.gtemodel`, is about 128 MB, and has SHA-256 `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`.

Runtime code is MIT licensed. The model artefact follows the upstream model card and repository licensing terms; release manifests must retain its source and digest.

## Needle study artefacts

The Needle feasibility and shallow-router study builds on [`cactus-compute/needle`](https://github.com/cactus-compute/needle), using upstream source commit `ffb1c51` and Hugging Face model revision `5f89b4307696d669c3df1d38ae057e6e1728b107`. The fine-tuned checkpoint, deterministic NDL1 conversion and pure-Go inference/SentencePiece/SIMD runtime are Memento additions; their evidence and corpora are described in [`docs/evidence/needle/README.md`](evidence/needle/README.md) and [`models/needle/README.md`](../models/needle/README.md).

Needle runtime files are release-hosted and verified through `models/runtime-models.json`. Prepare them before runtime checks with:

```bash
python3 tools/prepare_runtime_models.py
```

That command prepares only the three runtime files. The separately pinned `training-assets-v1` release contains the family-separated train/validation/test corpora for anyone reproducing the historical training study.

## Release records

`models/runtime-models.json` ties runtime model files to release-hosted archives and SHA-256 digests. Native image builds verify those files before publication, and GitHub releases record the immutable multi-architecture OCI digest, provenance attestation and attached SPDX JSON SBOM.
