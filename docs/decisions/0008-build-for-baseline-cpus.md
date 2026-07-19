# ADR 0008: Build release images for baseline CPUs

**Status:** accepted  
**Date:** 2026-07-19

## Decision

The amd64 release image compiles Rust with `target-cpu=x86-64`. SIMD kernels use runtime feature detection and select AVX2/FMA only when both features are present; older CPUs use scalar kernels automatically. ARM64 builds use a generic target and select NEON at runtime.

Release images use pinned Debian Bookworm builder and Python 3.12 runtime bases. Before the multi-architecture manifest is published, the amd64 image runs under QEMU's Westmere CPU model, which provides SSE4.2 but no AVX2 or FMA. The gate loads GTE-small and the fine-tuned Needle router and performs real inference.

GitHub Actions publishes the tested image to GHCR but does not deploy it to DiskStation.

## Why

The first NAS target uses an Intel Celeron J3455. It supports SSE4.2 but not AVX, AVX2 or FMA. Building for a GitHub runner's native CPU could put unsupported instructions outside the guarded vector functions, causing an illegal-instruction crash before runtime dispatch has a chance to fall back.

Bookworm and Python 3.12 provide a conservative userspace and longer support window for an older DSM host.

## Consequences

* amd64 code outside guarded kernels uses the x86-64 baseline ISA.
* AVX2/FMA and scalar code ship in one image; users do not select a mode.
* The release workflow blocks manifest publication if no-AVX GTE or Needle inference fails.
* Native CI checks Needle's peak process RSS against 220 MiB.
* The DiskStation profile starts with a 320 MiB container limit, Needle enabled and semantic search disabled.
* GTE-small is supported on the J3455 scalar path, but its roughly 297 MiB native peak requires a larger container limit than the initial profile.
* NAS deployment and rollback remain operator actions with a pinned image version.

## Alternatives considered

* **Build with `target-cpu=native`:** rejected because the resulting image would depend on the CI runner's CPU.
* **Publish separate scalar and AVX images:** rejected because runtime dispatch already selects the correct implementation and separate images complicate upgrades.
* **Disable optimized kernels globally:** rejected because newer hosts benefit from AVX2/FMA and NEON without reducing compatibility.
* **Deploy automatically from GitHub Actions:** rejected because NAS mounts, secrets and rollback require an operator-controlled step.
