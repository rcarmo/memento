# Semantic search

The choice to keep lexical search primary is recorded in [ADR 0006](decisions/0006-keep-lexical-search-primary.md).

Semantic search is optional and rebuildable. FTS5 stays the default because it is cheap to recover and always available. Local benchmark reports are included; production measurements are not.

## What operators decide

* Enable semantic search only when the local Rust stack and model artefacts are in place.
* Keep `lexical` as the default unless benchmark data says otherwise.
* Use the vendored model at `models/gte/gte-small.gtemodel` unless an explicitly reviewed replacement is configured. The container image copies that file to `/usr/local/share/memento/models/gte-small.gtemodel` and exports matching default environment variables.

## Progressive low-priority generation

On shared or low-power hosts, enable progressive generation instead of a full startup refresh:

```json
{
  "progressive_enabled": true,
  "progressive_startup_delay_seconds": 120,
  "progressive_interactive_idle_seconds": 15,
  "progressive_delay_seconds": 30,
  "progressive_cpu_busy_limit_percent": 75,
  "progressive_cpu_sample_seconds": 15,
  "progressive_nice": 15,
  "max_batch_size": 1,
  "refresh_on_startup": false
}
```

The worker derives one missing or stale path at a time from `derived.sqlite`; no separate queue needs recovery. It waits through startup grace, recent interactive traffic, sampled CPU utilization from `/proc/stat` and pacing, then launches one short-lived embedding subprocess at low CPU priority with native thread pools restricted to one thread. Manual selected/visible/full refresh requests enter the same worker and receive priority without bypassing the gates.

Ready embeddings persist in `/var/lib/memento/derived.sqlite`. Container replacement therefore resumes from existing progress. Derived rebuilds retain embeddings whose concept text hash and model metadata remain valid, delete rows for removed concepts and enqueue only changed, missing or model-stale records. A degraded row is not retried forever in the background; changing its content/model marks it stale, and an operator can explicitly prioritize it with manual refresh. `/graph/api/v1/embeddings/status` reports `pause_reason`, `current_path` and `completed` count.

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

The three paths may also come from `MEMENTO_FFI_LIBRARY`, `MEMENTO_SQLITE_VECTOR_EXTENSION` and `MEMENTO_GTE_MODEL`. Explicit JSON values take precedence. Those environment variables are optional path overrides, not mandatory global settings. The vendored model SHA-256 is `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`; replacing it changes the embedding revision and forces re-indexing.

## Search modes

* `lexical`: weighted FTS5 ranking; default and always available.
* `semantic`: cosine ranking over authorised, ready embeddings.
* `hybrid`: deterministic reciprocal-rank fusion of lexical and semantic candidates.

Authorisation path filters are applied before semantic scoring, so hidden concepts do not influence visible scores or rank order.

## Derived-state rules

Concept embeddings are packed little-endian float32 BLOBs in `derived.sqlite`. Rows carry model, dimension, content hash and repository revision. Model changes mark old rows stale. Changed or deleted concepts update incrementally. A full derived rebuild retains ready rows whose text hash and model metadata still match, marks changed rows stale, removes deleted-concept rows and progressively fills only the remaining gaps.

If model loading or embedding fails, Memento still advances the lexical index, records the row as degraded, and keeps canonical writes successful. Semantic and hybrid requests then fall back to lexical with explicit warnings.

## Build and validation

```bash
make rust-check
make check
```

The Docker image builds the Rust FFI library, SQLite extension and subprocess worker in a separate stage, then copies the vendored GTE-small model into the runtime image. Python wheels do not currently bundle platform-specific Rust libraries; install or mount them separately and set the configured paths when needed.

## Local evidence reproduction

The reviewed semantic load report under [`docs/evidence/load-semantic-local.json`](evidence/load-semantic-local.json) was produced with:

```bash
PYTHONPATH=src .venv/bin/python tools/load_test.py \
  --profile functional \
  --concepts 100 \
  --workers 8 \
  --requests 200 \
  --semantic-enabled \
  --include-semantic \
  --output docs/evidence/load-semantic-local.json
```

`--semantic-enabled` matters here. It tells the harness to build the local Rust artefacts if needed and enable semantic search in the temporary test config instead of merely asking for semantic queries against a lexical-only runtime.

## Pending verification

Production benchmark data, model operating envelopes and packaged deployment evidence remain pending.
