# Semantic search

The choice to keep lexical search primary is recorded in [ADR 0006](decisions/0006-keep-lexical-search-primary.md).

Semantic search is optional and rebuildable. FTS5 remains the default because it is cheap to recover and always available.

## Pure-Go runtime

The `v1.0.0` image uses only:

* `go/gte` for GTE1 parsing, WordPiece tokenization and FP32 inference;
* `go/internal/simd` for AVX2/SSE2/NEON/scalar vector operations;
* `memento-embed-go` for the existing framed process-isolation protocol;
* pure-Go SQLite access and cosine scoring over little-endian float32 blobs.

There is no Python, Rust, CGo, C ABI, native SQLite extension or external model runtime in the image.

For low-memory NAS operation, semantic `worker_mode: "subprocess"` invokes the static Go worker at `/usr/local/bin/memento-embed` once per request, then releases model memory when the process exits. Needle uses the same lifecycle with `/usr/local/bin/memento-needle-go`, but maps a release-generated FP32 sidecar read-only so it does not repeat NDL1 parsing or BF16 expansion. Explicit `worker_mode: "in_process"` remains available for diagnostics and separately qualified hosts.

The old `ffi_library_path` and `sqlite_extension_path` fields remain accepted in schema-version-2 configuration so the production file can be mounted unchanged. Go does not load or require those paths. `MEMENTO_GTE_MODEL` remains a meaningful model-path override; `MEMENTO_SIMD` selects `auto`, `scalar`, `sse2`, `avx2` or `neon` where available.

## Configuration

```json
{
  "intelligent_tiers": {
    "semantic_search": {
      "enabled": true,
      "worker_mode": "subprocess",
      "worker_path": "/usr/local/bin/memento-embed",
      "ffi_library_path": null,
      "sqlite_extension_path": null,
      "model_path": "/usr/local/share/memento/models/gte-small.gtemodel",
      "model_id": "rust-gte",
      "dimensions": 384,
      "max_input_chars": 4096,
      "max_batch_size": 1,
      "max_candidates": 200,
      "default_search_mode": "lexical",
      "refresh_on_startup": false,
      "progressive_enabled": true,
      "progressive_startup_delay_seconds": 120,
      "progressive_interactive_idle_seconds": 15,
      "progressive_delay_seconds": 30,
      "progressive_cpu_busy_limit_percent": 75,
      "progressive_cpu_sample_seconds": 15,
      "progressive_nice": 15
    }
  }
}
```

The legacy FFI fields and `model_id` value are retained so the mounted production configuration and persisted embedding rows do not need migration. The Go runtime ignores the FFI paths. The vendored model SHA-256 is `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`; changing model identity or digest marks old embeddings stale.

## Search modes

* `lexical`: weighted FTS5 ranking; default and always available.
* `semantic`: query embedding plus cosine ranking over authorised, ready vectors.
* `hybrid`: deterministic reciprocal-rank fusion of lexical and semantic candidates.

Authorisation filters are applied before semantic scoring, so hidden concepts cannot influence visible scores or rank order. The optimized scorer reads vectors directly from SQLite blobs, uses their validated stored norms, and has an enforced zero-allocation kernel budget.

## Progressive generation

On shared or low-power hosts, progressive generation derives one missing or stale path at a time from `derived.sqlite`. It waits through startup grace, recent interactive activity, sampled CPU utilization and configured pacing. Manual selected/visible/full refresh requests enter the same queue and receive priority without bypassing those gates.

Ready embeddings persist in `/var/lib/memento/derived.sqlite`. Container replacement therefore resumes from existing progress. Derived rebuilds retain embeddings whose concept text hash and model metadata remain valid, delete removed rows and enqueue only changed, missing or model-stale concepts.

`/graph/api/v1/embeddings/status` reports worker liveness, activity, pending work, pause reason, current path, errors and completed jobs. A stopped worker rejects new work rather than appearing idle.

## Performance and release gates

Run:

```bash
python3 tools/prepare_runtime_models.py
make model-test
make performance
make go-container-contract MEMENTO_VERSION=1.0.0
```

The performance gate enforces zero-allocation semantic scoring, tokenizer lookups and SIMD dot products; real GTE is capped at 2 allocations and 2,400 bytes per operation. In-process real Needle generation is capped at 400 allocations and 1.8 MB after immutable constraint caching, request-local workspace reuse and typed BPE queues. Mapped one-shot routing adds about 5--6 ms of setup on the measured amd64 host and completes in roughly 79--93 ms. The container gate verifies the pure-Go subprocess path under the DiskStation read-only/512 MiB contract, authenticated semantic readiness, and old-image -> Go -> old-image state compatibility. Wall-clock figures are recorded as evidence but not used as cross-runner CI gates.
