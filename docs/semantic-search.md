# Semantic search

The choice to keep lexical search primary is recorded in [ADR 0006](decisions/0006-keep-lexical-search-primary.md).

Semantic search is optional and rebuildable. FTS5 remains the default because it is cheap to recover and always available.

## Pure-Go runtime

The `v1.0.0` image uses only:

* `internal/gte` for GTE1 parsing, WordPiece tokenization and FP32 inference;
* `internal/simd` for AVX2/SSE2/NEON/scalar vector operations;
* `memento-embed-go` for the existing framed GTE process-isolation protocol;
* `memento-needle-go` and a release-generated `.nfp32` sidecar for mapped one-route Needle workers;
* pure-Go SQLite access and cosine scoring over little-endian float32 blobs.

There is no Python, Rust, CGo, C ABI, native SQLite extension or external model runtime in the image.

For low-memory NAS operation, semantic `worker_mode: "subprocess"` invokes the static Go worker at `/usr/local/bin/memento-embed` once per request, then releases model memory when the process exits. Needle uses the same lifecycle with `/usr/local/bin/memento-needle-go`, but maps a release-generated FP32 sidecar read-only so it does not repeat NDL1 parsing or BF16 expansion. Explicit `worker_mode: "in_process"` remains available for diagnostics and separately qualified hosts.

The old `ffi_library_path` and `sqlite_extension_path` fields remain accepted in schema-version-2 configuration so the production file can be mounted unchanged. Go does not load or require those paths. The Go runtime defaults semantic and Needle routing to `subprocess`, so shared rollback-compatible configuration files need not add the newer worker or FP32-sidecar keys that `0.5.9` cannot parse. `MEMENTO_GTE_MODEL` overrides the GTE asset. `MEMENTO_NEEDLE_MODEL`, `MEMENTO_NEEDLE_FP32_MODEL`, `MEMENTO_NEEDLE_TOKENIZER` and `MEMENTO_NEEDLE_WORKER` override the corresponding Needle paths. `MEMENTO_SIMD` selects `auto`, `scalar`, `sse2`, `avx2` or `neon` where available.

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

The legacy FFI fields and `model_id` value are retained so the mounted production configuration and persisted embedding rows do not need migration. The Go runtime ignores the FFI paths. The pinned GTE model SHA-256 is `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`; changing model identity or digest marks old embeddings stale.

## Search modes

* `lexical`: weighted FTS5 ranking; default and always available.
* `semantic`: query embedding plus cosine ranking over authorised, ready vectors.
* `hybrid`: deterministic reciprocal-rank fusion of lexical and semantic candidates.

Authorisation filters are applied before semantic scoring, so hidden concepts cannot influence visible scores or rank order. The optimized scorer reads vectors directly from SQLite blobs, uses their validated stored norms, and has an enforced zero-allocation kernel budget.

## Item-relative document chunks

Memento embeds the complete title, description and body in windows of at most 384 content tokens, measured with the packaged GTE WordPiece vocabulary. Each request also keeps the 4,096-Unicode-character guard. Ordinary windows overlap by up to 64 tokens, rounded to basic-token boundaries. Paragraph boundaries are preferred; headings start clean sections without overlap from the preceding section. The model has 512 positions including CLS/SEP.

Chunks are stored in the additive `concept_embedding_chunks` table. The first-chunk mirror in `concept_embeddings` exists solely for rollback readers; current refresh, search and graph paths never fall back to single vectors. Startup classifies existing single-vector rows as `legacy`, excluding them from semantic reads until item-by-item regeneration. Valid chunks are retained.

Freshness is item-relative: effective title/description/body fingerprint, model ID/digest/dimensions and chunking policy determine validity. Unrelated commits, metadata-only edits, stable-ID renames and rebuilds preserve vectors, timestamps and generation provenance. Actual input changes mark only that item stale. The aggregate `semantic_embedding_revision` keeps its historical summary format for clients, but repository-revision equality does not gate search or graph eligibility.

Search applies permission/model/policy filters, scores authorised chunks, and returns the best chunk cosine per concept. It does not discard later paths at the old candidate cap. Graph similarity uses one equal-weight aggregate of normalized chunks per item, avoiding quadratic chunk-pair comparisons. `semantic_graph_items` stores these aggregates and `semantic_graph_pairs` stores cosine scores. Publication recomputes only the changed item's pairs; startup backfills missing entries once. Web loads read stored scores without loading vectors or recalculating similarity. Permission and policy filtering precede neighbour selection. Valid compatible items can connect while others are missing, legacy, stale or queued.

Inference runs outside the index lock and SQLite transaction. Publication rechecks item content and atomically replaces a complete chunk set. Failed refresh retains ready chunks only when input, model and policy still match. Incremental multi-item index updates are atomic, including invalidation, link recomputation and orphan cleanup. Deleting a parent embedding also deletes its chunks, including when a rollback binary performs the deletion.

When a full refresh is requested, legacy single-vector rows and mismatched model/policy identities are eligible for chunk conversion. The policy identity includes model ID/digest/dimensions, chunk algorithm version, token budget, overlap and character guard. A terminal failure stops the current full run with an error; an explicit selected-path retry is required for an unchanged error row. Transient database failures back off and remain queued. Re-enqueued paths and full requests carry generations so older work cannot discard newer requests. `refresh_on_startup: false` still prevents automatic re-embedding at startup. The progressive delay applies once between entries. Idle and CPU admission checks still run between bounded chunk batches, and shutdown cancels in-flight subprocess inference. The subprocess applies `progressive_nice` to all Go runtime threads before loading the model.

Tests cover real GTE/subprocess execution, tail retrieval, Unicode/token budgets, SQL faults, cancellation, model/policy filtering, legacy migration, unchanged-item retention, atomic failed updates and concurrent enqueue generations. See [the item-relative audit](evidence/item-relative-embeddings-2026-09-22.md) for validation and deployment status.

## Progressive generation

On shared or low-power hosts, progressive generation derives one missing or stale path at a time from `derived.sqlite`. It waits through startup grace, recent interactive activity, sampled CPU utilisation and the configured delay between entries. Manual selected/visible/full refresh requests enter the same queue and receive priority without bypassing those gates.

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
