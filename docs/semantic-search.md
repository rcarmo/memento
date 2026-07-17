# Semantic search

Status: implemented and locally verified; production model benchmarks remain pending.

Memento keeps lexical FTS5 search as the default. Semantic search is an optional rebuildable derived tier backed by a local Rust port of GTE-small.

## Components

- `memento-gte`: GTE1 FP32 model parser, tokenizer and inference.
- `memento-vector`: packed float32 validation and scalar/SIMD cosine kernels.
- `memento-ffi`: stable C ABI loaded from Python with `ctypes`.
- `memento-sqlite-vector`: loadable SQLite extension exposing `vector_cosine`, `vector_dimensions` and `vector_is_valid`.
- `memento-embed`: framed subprocess fallback for process isolation.

The model file is not shipped in the source tree or image. Mount it read-only under `/models` and record its source, licence and SHA-256 digest.

## Configuration

```json
{
  "intelligent_tiers": {
    "semantic_search": {
      "enabled": true,
      "ffi_library_path": "/usr/local/lib/memento/libmemento_ffi.so",
      "sqlite_extension_path": "/usr/local/lib/memento/libmemento_sqlite_vector.so",
      "model_path": "/models/gte-small.gtemodel",
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

The three paths may also come from `MEMENTO_FFI_LIBRARY`, `MEMENTO_SQLITE_VECTOR_EXTENSION`, and `MEMENTO_GTE_MODEL`. Explicit JSON values take precedence.

## Search modes

- `lexical`: weighted FTS5 ranking; default and always available.
- `semantic`: cosine ranking over authorized, ready embeddings.
- `hybrid`: deterministic reciprocal-rank fusion of lexical and semantic candidates.

Authorization path filters are applied before semantic scoring. Hidden concepts cannot influence visible scores or rank order.

Concept embeddings are packed little-endian float32 BLOBs in `derived.sqlite`. Rows carry model, dimension, content hash and repository revision. Model changes mark old rows stale. Changed or deleted concepts update incrementally; a full derived rebuild regenerates all vectors.

If model loading or embedding fails, Memento advances the lexical index, marks semantic readiness degraded and keeps canonical writes successful. Semantic and hybrid requests fall back to lexical with explicit warnings.

## Build and validation

```bash
make rust-check
make check
```

The Docker image builds the Rust FFI library, SQLite extension and subprocess worker in a separate stage. Python wheels do not currently bundle platform-specific Rust libraries; install or mount them separately and set the configured paths.
