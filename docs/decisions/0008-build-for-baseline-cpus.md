# ADR 0008: Build release images for baseline CPUs

**Status:** accepted  
**Date:** 2026-07-19
**Amended:** 2026-09-19

> `v1.0.0` keeps the baseline-CPU and native-architecture decision with Go `GOAMD64=v1`, runtime SSE2/AVX2/NEON selection, and a pinned distroless Debian 12 runtime. Rust/Python build details below describe the superseded image.

## Decision

The amd64 release image builds with `GOAMD64=v1`. SIMD kernels use runtime feature detection and select AVX2/FMA only when both features are present; older amd64 CPUs use SSE2, and ARM64 selects NEON. Scalar remains available everywhere as an explicit correctness fallback.

Release images use a pinned Go Bookworm builder and pinned distroless Debian 12 runtime. Before the multi-architecture manifest is published, the amd64 image runs under QEMU's Westmere CPU model, which provides SSE4.2 but no AVX2 or FMA. The gate loads GTE-small and the fine-tuned Needle router and performs real inference.

GitHub Actions publishes the tested image to GHCR but does not deploy it to DiskStation.

## Why

The first NAS target uses an Intel Celeron J3455. It supports SSE4.2 but not AVX, AVX2 or FMA. Building for a GitHub runner's native CPU could put unsupported instructions outside the guarded vector functions, causing an illegal-instruction crash before runtime dispatch has a chance to fall back.

Bookworm and the static distroless runtime provide a conservative userspace and avoid a host-language runtime dependency on the older DSM host.

## Consequences

* amd64 code outside guarded kernels uses the x86-64 baseline ISA.
* AVX2/FMA, SSE2, NEON and scalar code ship in the architecture images; automatic dispatch needs no operator selection, while `MEMENTO_SIMD` can force a supported tier.
* The release workflow blocks manifest publication if no-AVX GTE or Needle inference fails.
* Native CI checks Needle's peak process RSS against 220 MiB.
* The deployed DiskStation profile uses a 512 MiB container limit with Needle and semantic search enabled.
* GTE-small runs on the J3455 SSE2 path through short-lived progressive workers at low priority; its roughly 297 MiB historical peak remains bounded by worker exit.
* NAS deployment and rollback remain operator actions with a pinned image version.

## Alternatives considered

* **Build with `target-cpu=native`:** rejected because the resulting image would depend on the CI runner's CPU.
* **Publish separate scalar and AVX images:** rejected because runtime dispatch already selects the correct implementation and separate images complicate upgrades.
* **Disable optimized kernels globally:** rejected because newer hosts benefit from AVX2/FMA and NEON without reducing compatibility.
* **Deploy automatically from GitHub Actions:** rejected because NAS mounts, secrets and rollback require an operator-controlled step.
