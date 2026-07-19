# ADR 0010: Use tiled matrix kernels for local inference

**Status:** accepted  
**Date:** 2026-07-19

## Decision

Memento uses shared, cache-aware FP32 matrix kernels for GTE and Needle projections. The kernels process up to four input rows together, retain output tiles in registers and traverse each model's native weight layout without repacking the model at load time.

Two layouts are supported:

* GTE weights are `[out, in]`. Its kernel reuses each contiguous output-weight row across several token or batch rows.
* Needle weights are `[in, out]`. Its kernel loads contiguous output lanes and accumulates several input rows before advancing through the weights.

The x86-64 implementation selects AVX2/FMA at runtime. ARM64 selects NEON at runtime. Both retain a scalar implementation for baseline CPUs and other architectures. Release binaries continue to target baseline x86-64 rather than the build host's CPU, as required by [ADR 0008](0008-build-for-baseline-cpus.md).

Needle uses tiled matrix multiplication for encoder, prompt and other multi-row projections. Single-token decoder projections retain the specialised matrix-vector path because there is no second row over which to amortise a matrix tile.

GTE batches inference rather than merely batching its worker protocol. Inputs are tokenised together, padded to the longest sequence in the bounded batch and projected as `[batch * sequence, hidden]` matrices. Attention remains isolated per input and ignores padded keys. Single-item embedding uses the same implementation with a batch size of one.

## Why

The previous projection code was mathematically correct but left reuse to the cache by accident. GTE computed one output dot product at a time. Needle projected each row independently with repeated AXPY calls. Both approaches repeatedly traversed weights without retaining enough output state in registers.

The kernel structure follows the practical lesson in Justine Tunney's [CPU matrix multiplication work](https://justine.lol/matmul/): use matrix-matrix kernels when prompt, sequence or batch rows are available, keep small output tiles in registers and reserve matrix-vector kernels for genuinely single-row work.

Keeping each model's existing weight layout avoids a second copy of the 128 MB GTE model or the roughly 53 MB Needle model. This matters on the DiskStation target, where mmap-backed GTE loading and short-lived workers are intended to reduce idle memory rather than trade it for repacked weights.

## Validation

The native AMD64 implementation passed:

* the independent Go GTE embedding fixture within `1e-4` per component;
* batched-versus-individual GTE parity for varied sequence lengths, empty input and batch tails;
* all 360 untouched Needle routing decisions with 360 valid outputs;
* 182 Python tests and the complete Rust workspace test suite;
* Ruff, mypy, Rustfmt and Clippy with warnings denied.

The end-to-end GTE measurements include Python framing, worker startup, read-only mmap model loading, inference, vector transfer and process exit. Five runs were made for each batch size on the same host and payloads:

| Batch | Previous p50 | Tiled/batched p50 | Latency reduction | Throughput gain |
|---:|---:|---:|---:|---:|
| 1 | 187.1 ms | 88.8 ms | 52.6% | 2.11x |
| 8 | 878.2 ms | 598.6 ms | 31.8% | 1.47x |
| 16 | 1,497.6 ms | 1,036.8 ms | 30.8% | 1.44x |

For Needle, the release evaluator was pinned to one logical CPU and ran the unchanged 360-case corpus serially:

| Metric | Previous | Tiled | Change |
|---|---:|---:|---:|
| warm p50 | 510.8 ms | 445.4 ms | 12.8% lower |
| warm p95 | 554.6 ms | 496.9 ms | 10.4% lower |
| throughput | 1.95 requests/s | 2.24 requests/s | 14.9% higher |
| tool decisions | 360/360 | 360/360 | unchanged |

These are local measurements, not DiskStation estimates. The release pipeline must still run real GTE and Needle inference under QEMU's Westmere CPU model before publishing an amd64 image. This host does not have QEMU installed, so the no-AVX image gate was not reproduced during this local run.

## Consequences

* GTE concept indexing benefits from genuine bounded batch inference rather than model-load amortisation alone.
* GTE attention cost and padding limit batch scaling, so batch size remains configurable and capped at 16 for the initial DiskStation profile.
* Needle retains a distinct matrix-vector path for autoregressive single-token decoding.
* Floating-point accumulation order differs between scalar and SIMD implementations. Release gates compare final embeddings and routing decisions rather than requiring bit-identical intermediate tensors.
* The kernels use model-native layouts and do not allocate transposed or packed copies at model load.
* New projection layouts belong in `memento-vector`; model crates should not grow private SIMD implementations.

## Alternatives considered

* **Use the existing scalar loops:** rejected because real-model measurements show material latency and throughput losses.
* **Call a system BLAS library:** rejected for now because it adds a platform dependency, complicates the baseline-CPU image and provides less control over the small, mixed matrix shapes used during decoding.
* **Transpose or pre-pack every model at load:** rejected because the additional resident memory conflicts with the NAS memory budget and short-lived GTE worker design.
* **Use the tiled kernel for every Needle projection:** rejected because single-token decoding is a matrix-vector operation and the existing AXPY path is the better fit.
* **Batch GTE only at the subprocess protocol:** rejected because sequential `embed_to()` calls do not reuse weights across inputs and produced lower measured throughput.
