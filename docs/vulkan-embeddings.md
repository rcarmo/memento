# Vulkan embedding pre-test

The optional Vulkan backend runs Memento's existing FP32 GTE1 model. It shares the CPU tokenizer, token limits, input embedding construction, attention masks, mean pooling and L2 normalisation. No GGUF conversion, replacement model or quantisation is used. CPU remains the default. Cold-request parity and timings have been checked on one Intel Iris Xe; warm reuse and production deployment need separate qualification.

[Documentation index](README.md) · [Semantic search](semantic-search.md)

## Implementation and limits

Build feature `memento-embed/vulkan` enables a Vulkan-only wgpu compute path. WGSL shaders implement FP32 linear layers, attention scores/softmax/value reduction, residual addition, layer normalisation and GELU. The transformer buffers stay on the GPU; layers submit separately to bound uploaded weights. Pooling and normalisation of the final 384-dimensional vector use the existing CPU functions.

The first implementation targets the packaged GTE-small geometry: 12 layers, 384 hidden dimensions, 12 heads, 1,536 intermediate dimensions, at most 512 tokens and at most four GPU batch items. Larger GPU batches return an error in explicit mode or run wholly on CPU in auto mode. The existing worker protocol accepts up to 16 items and bounds incoming frames to 4 MiB.

The kernels use 64-thread workgroups and ordinary FP32 storage. They do not require FP16, subgroup-size assumptions, integer dot products or cooperative matrices. wgpu still checks its own Vulkan/driver baseline and device limits. GPU selection excludes CPU and virtual adapters; Lavapipe/llvmpipe cannot pass as hardware acceleration.

The local llama.cpp checkout was consulted for device filtering, conservative feature checks, shared-memory matrix tiling and Intel-specific hazards. No llama.cpp inference code or weights are included. Its Sigma reports identify an i5-1340P / Iris Xe and show workload-dependent Vulkan performance, including correct but slower generation. Those results do not measure GTE embeddings.

## Build and test locally

Use a current Rust toolchain, a working Vulkan loader/ICD and access to a hardware GPU. On Debian/Ubuntu, `vulkan-tools` provides `vulkaninfo`; the appropriate vendor driver must already work. Shader compilation is through the pinned wgpu/Naga Rust dependencies; an external GLSL compiler is unnecessary.

```bash
vulkaninfo --summary
make install-dev
make check
make vulkan-check
make vulkan-pretest VULKAN_DEVICE=Intel
```

`VULKAN_DEVICE` is a case-insensitive device-name substring; omit it to select the first enumerated physical integrated/discrete GPU. Use a specific selector for repeatable multi-GPU testing. `make vulkan-pretest` builds the release worker and writes `build/vulkan-pretest.json`. It fails if explicit Vulkan did not run, vectors are nonfinite or non-unit, cosine falls below 0.99999, or maximum absolute error exceeds 0.001. It also tests absent-device auto fallback. Inputs include short text, a mixed-length Unicode/empty batch, a medium document and token-limit truncation, using three fresh processes per case/backend.

The hardware integration test is explicit and fails if its model/device is missing:

```bash
cd rust
cargo test --release -p memento-embed --features vulkan \
  hardware_same_model_parity_and_diagnostics -- --ignored --nocapture
```

CPU-only checks do not need Vulkan libraries or GPU access. CI additionally compiles and tests the optional feature without claiming hardware coverage.

## Backend controls

The worker accepts:

```text
memento-embed MODEL [cpu|vulkan|auto] [DEVICE_SUBSTRING]
```

Its existing framed stdin/stdout protocol remains intact. Response headers add backend diagnostics: requested/selected backend, device name and fallback reason. Never write informational text to stdout ahead of a protocol frame.

* `cpu` is the default and does not initialise Vulkan.
* `vulkan` requires the build feature, a matching hardware adapter and a successful same-model self-test. Failures are explicit; it does not silently substitute CPU.
* `auto` probes hardware and performs a CPU/GPU self-test. A failure falls back to the original CPU implementation, discarding partial results. It selects availability, not the fastest backend for a particular input.

The Python subprocess client bounds the whole call with its existing timeout. A recoverable auto worker error can retry once on CPU within the remaining deadline; a spent deadline returns an error. The client remembers a failed GPU attempt and uses CPU on subsequent requests during that process lifetime. Explicit Vulkan errors never trigger that fallback. Driver hangs/crashes are contained by the subprocess boundary, not recoverable by an in-process Rust timer.

Startup and each fresh worker include adapter/pipeline setup and a short same-model self-test. Cold processes remain the default. Optional idle-capped reuse retains that initialisation across calls; it does not add a resident GPU service or automatic driver/container reconfiguration.

## Optional warm worker

Set `intelligent_tiers.semantic_search.worker_idle_seconds` above zero to reuse one subprocess per client. The default is `0` (start and reclaim a worker per call); valid values are finite seconds from 0 to 3600. Positive values require subprocess mode and a POSIX runtime. The setting works with CPU, Vulkan or auto; it does not change the model, vectors or backend revision.

Reuse retains the model mapping, device/queue, compiled pipelines and completed startup self-test. It does **not** retain every layer's GPU weights or eliminate per-layer uploads/temporary buffers. No dummy inference is sent to keep the worker alive. One worker serves requests serially, including concurrent foreground and background callers.

The request deadline covers lock wait, pipe writes/reads and a possible whole-batch CPU fallback. Cancellation is checked during I/O and queue wait. Responses are bounded to a 64 KiB header allowance plus the expected vector bytes, matched to unique request IDs, and validated for shape, backend and finite values. Stderr is drained during requests with at most 8 KiB retained internally; it is never returned as a warm-worker diagnostic. Failure discards the process before retry; explicit Vulkan never silently falls back.

Idle expiry, cancellation, protocol failure, timeout and `close()` kill the owned process group and reap the child. Cleanup has a separate bounded one-second reap allowance after the request deadline; a process stuck in an uninterruptible kernel operation can still fail that cleanup. Calls after warm-client close are rejected. A child is recycled before its 257th request even under continuous traffic. An expiry callback is tied to both the process generation and idle token, so an old callback cannot kill a reused/replacement worker. Runtime shutdown already stops the refresh scheduler before closing this client.

The Python client's `worker_status` snapshot reports the owned PID (not a live health probe), generation, starts, completed calls, active state, monotonic idle deadline and closed state. This is not a new public MCP/status API. OS process creation itself cannot be interrupted by the pipe deadline, and cancellation callbacks must return promptly.

A 120-second idle lifetime is a candidate experiment, not a recommended production default. It spans the existing 30-second progressive pacing, but actual request gaps and idle memory must be measured. The CPU-only lifecycle tests use disposable protocol fakes; they establish reuse, framing and cleanup behaviour, not GPU memory reclamation or warm performance.

There is no automatic GPU reservation or inference-priority integration. On a shared device, do not enable reuse without an agreed allocation/window; blocking new requests alone does not free an idle worker's allocations. Never evict another service's weights to make room. The current local primary server must remain resident, and hardware measurements require Rui's explicit isolated maintenance-window approval.

## Service configuration for a disposable instance

Use a separate test repository/index and the newly built worker. Current release containers are unchanged and do not include Vulkan dependencies or GPU device mappings.

```json
{
  "intelligent_tiers": {
    "semantic_search": {
      "enabled": true,
      "worker_mode": "subprocess",
      "worker_path": "/absolute/path/to/memento-embed",
      "model_path": "/absolute/path/to/gte-small.gtemodel",
      "backend": "vulkan",
      "vulkan_device": "Intel",
      "max_batch_size": 1,
      "worker_timeout_seconds": 120,
      "worker_idle_seconds": 0
    }
  }
}
```

Use `backend: "auto"` only when fallback is wanted. Non-CPU settings are rejected for in-process FFI mode. Runtime status includes `configured_backend` and the last subprocess backend diagnostics; MCP readiness reports the configured mode.

Experimental Vulkan/auto configurations append `:gte1-fp32-vulkan-v1` to the model revision. This separates their vectors from the established CPU index even though they use identical model bytes and passed local parity. Switching modes on an existing index can mark embeddings stale; do not point the Sigma test at production state. CPU fallback within experimental auto mode retains this experimental identity. Broader corpus/retrieval validation is required before treating existing CPU and experimental vectors as interchangeable everywhere.

## Sigma pre-test

The review bundle contains a source snapshot, the exact GTE1 model, the local x86-64 worker/parity binaries, the standalone Python runner, checksums and local results. Verify `SHA256SUMS` after transfer. The local binary requires x86-64 Linux and glibc 2.34 or newer. Rebuild on Sigma if its libc/driver requirements do not match; source and `Cargo.lock` are included.

```bash
sha256sum -c SHA256SUMS
vulkaninfo --summary > sigma-vulkan.txt
python3 tools/vulkan_pretest.py --worker bin/memento-embed \
  --model models/gte-small.gtemodel --device Intel --output sigma-results.json
```

The runner uses only the supplied model and non-sensitive sample strings. It does not connect to Memento or read/write any index. Each subprocess has a 120-second limit; test repetitions are capped at five. Preserve stderr and a failure report if the driver rejects the workload. Before testing, check other GPU users, available system/GPU memory and render-node permissions. Do not run a privileged container or install a different host driver just to pass the test.

If rebuilding from the bundled source:

```bash
tar -xzf memento-source.tar.gz
cd memento-source
make vulkan-build
python3 tools/vulkan_pretest.py --worker rust/target/release/memento-embed \
  --model ../models/gte-small.gtemodel --device Intel --output ../sigma-results.json
```

Use a production-like memory cap only in a disposable process/container. The NAS has a 512 MiB limit; this GPU path has not been qualified under that cap. Model mappings, uploaded weights, attention buffers and driver allocations all contribute, and device memory accounting can differ from ordinary RSS. A single readback follows the transformer; each layer still uploads weights and synchronises, leaving room for later performance work if Intel measurements justify it.

## Measured locally

The development host has an NVIDIA RTX 3060 (12 GiB), driver 580.173.02, Linux 6.8 and a hardware Vulkan device. The virtual GPU and llvmpipe were not used. Model SHA-256:

```text
06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171
```

Final three-run cold-process medians, including model load, Vulkan initialisation and self-test:

| Input | CPU | Vulkan |
| --- | ---: | ---: |
| Short single input | 0.113 s | 0.386 s |
| Unicode/empty mixed batch of three | 0.280 s | 0.396 s |
| Medium input | 0.844 s | 0.417 s |
| Token-limit input | 4.341 s | 0.516 s |

The separate warm-engine comparison reached maximum absolute error below 1.6e-7. Raw cold-process parity/timing samples are in [the local report](evidence/vulkan-rtx3060-pretest.json). Results are specific to this host/build; they are not Intel or DiskStation measurements. Short requests lose to GPU startup overhead, while longer inputs benefit. Existing progressive pacing remains unchanged.

Sigma completed one authorised cold benchmark campaign on Intel Iris Xe (RPL-P), Mesa 26.1.5: 12 explicit Vulkan samples passed, maximum error `1.4901161193847656e-7`; median CPU/Vulkan ratios were 1.630 for medium and 5.074 for token-limit inputs. Short and mixed-batch inputs were slower. The original archive and all 38 internal checksums were checked on receipt; the monitor summary has a documented off-by-one (101 claimed samples, 100 retained records). Vector payloads were not retained for independent parity recomputation. Details are linked from [PR #36](https://github.com/rcarmo/memento/pull/36#issuecomment-5736430906).

Gemma was resident but idle during those observations. Concurrent inference, warm-idle retention and NAS 512 MiB operation remain unqualified. No further GPU test or deployment accompanies the worker-lifecycle changes in [#37](https://github.com/rcarmo/memento/issues/37). The older production DiskStation still needs separately authorised device/driver compatibility qualification.
