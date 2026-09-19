# SIMD optimisation

The scalar GTE and Needle kernels remain the correctness oracle and default. `go/internal/simd` adds an opt-in shared substrate selected with `MEMENTO_SIMD`:

- `scalar` — universal exact-order fallback and default;
- `sse2` — baseline amd64 path suitable for older Intel systems such as the Celeron J3455;
- `avx2` — runtime-gated AVX2 tier (requires AVX2 and FMA capability);
- `neon` — ARM64 ASIMD/NEON path;
- `auto` — AVX2, then SSE2, then NEON, then scalar.

The DiskStation Celeron J3455 path is an explicit release gate: `GOAMD64=v1` builds run `memento-simd-check` with AVX/AVX2/FMA disabled and must report `backend=sse2`; release CI runs the same helper under QEMU's Westmere CPU model before the existing scalar model smoke. Backend selection is validated once through an `Engine`; hot calls do not repeat CPU-policy checks. Dot products permit a bounded reduction-order difference (`1e-4` relative in synthetic tests). AXPY currently matches scalar float32 bits. Scalar remains the model default because changed reduction order can change embeddings or generated tokens.

On the local Intel i7-12700, a 384-element persistent-dispatch dot benchmark measured scalar at roughly 63–71 ns/op, SSE2 at roughly 52–58 ns/op, and AVX2 at roughly 26–28 ns/op. SSE2 was about 1.2–1.35× faster and AVX2 about 2.3–2.7× faster. A 32×384 by 384×384 GTE projection measured roughly 3.45–3.80 ms scalar versus 0.28–0.29 ms AVX2, around 12–13× faster. Needle's full-vocabulary 8,192×384 argmax remained about 1.4–1.8 ms for both scalar and AVX2, indicating memory-bandwidth/dispatch overhead dominates that kernel. These are microbenchmarks, not end-to-end inference results.

ARM64 assembly cross-compiles in the normal static release gate. Real ARM64 hardware differential and performance measurements are required before model dispatch can select NEON. GTE linear projections and Needle attention/argmax now accept the optional Engine. Loaded service models read `MEMENTO_SIMD`; unset remains scalar, while invalid or unavailable values fail startup. Synthetic GTE projection and Needle attention/argmax tests compare against scalar with explicit tolerances. Gated real-model tests compare GTE output (`1e-5` max absolute error) and exact Needle generation, but the required assets are unavailable on this host and therefore remain a release gate. GTE and Needle integration must pass real-model/corpus parity and record embedding-space or generated-token effects before `auto` becomes a runtime default.
