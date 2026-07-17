# Semantic search

Semantic search is optional, rebuildable and deliberately second to lexical search. Memento keeps FTS5 as the default because it is deterministic, cheap to recover and always available. Semantic search is implemented and locally verified; production model benchmarks and live operating evidence are still pending.

## What operators decide

* Enable semantic search only when the local Rust stack and model artefacts are in place.
* Keep `lexical` as the default unless benchmark data says otherwise.
* Use the vendored model at `rust/tests/fixtures/gte-small.gtemodel` unless an explicitly reviewed replacement is configured. The container installs it read-only at `/usr/local/share/memento/models/gte-small.gtemodel`.

## Components

* `memento-gte`: GTE1 FP32 model parser, tokenizer and inference.
* `memento-vector`: packed float32 validation and scalar/SIMD cosine kernels.
* `memento-ffi`: stable C ABI loaded from Python with `ctypes`.
* `memento-sqlite-vector`: loadable SQLite extension exposing `vector_cosine`, `vector_dimensions` and `vector_is_valid`.
* `memento-embed`: framed subprocess fallback for process isolation.

## Configuration

```json
{
  "intelligent_tiers": {
    "semantic_search": {
      "enabled": true,
      "ffi_library_path": "/usr/local/lib/memento/libmemento_ffi.so",
      "sqlite_extension_path": "/usr/local/lib/memento/libmemento_sqlite_vector.so",
      "model_path": "/usr/local/share/memento/models/gte-small.gtemodel",
      "model_id": "gte-small-fp32",
      "dimensions": 384,
      "max_input_chars": 4096,
      "max_batch_size": 16,
      "max_candidates": 200,
      "default_search_mode": "lexical"
    }
  }
}
```

The three paths may also come from `MEMENTO_FFI_LIBRARY`, `MEMENTO_SQLITE_VECTOR_EXTENSION`, and `MEMENTO_GTE_MODEL`. Explicit JSON values take precedence. The vendored model SHA-256 is `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`; replacing it changes the embedding revision and forces re-indexing.

## Search modes

* `lexical`: weighted FTS5 ranking; default and always available.
* `semantic`: cosine ranking over authorised, ready embeddings.
* `hybrid`: deterministic reciprocal-rank fusion of lexical and semantic candidates.

Authorisation path filters are applied before semantic scoring, so hidden concepts do not influence visible scores or rank order.

## Derived-state rules

Concept embeddings are packed little-endian float32 BLOBs in `derived.sqlite`. Rows carry model, dimension, content hash and repository revision. Model changes mark old rows stale. Changed or deleted concepts update incrementally, and a full derived rebuild regenerates all vectors.

If model loading or embedding fails, Memento still advances the lexical index, marks semantic readiness degraded, and keeps canonical writes successful. Semantic and hybrid requests then fall back to lexical with explicit warnings.

## Build and validation

```bash
make rust-check
make check
```

The Docker image builds the Rust FFI library, SQLite extension and subprocess worker in a separate stage. Python wheels do not currently bundle platform-specific Rust libraries; install or mount them separately and set the configured paths.

## Pending verification

Production benchmark data, model operating envelopes and packaged deployment evidence remain pending.
