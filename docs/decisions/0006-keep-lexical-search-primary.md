# ADR 0006: Keep lexical search primary

**Status:** accepted  
**Date:** 2026-07-18

## Decision

Memento always builds lexical and graph indexes from Markdown. Local GTE-small embeddings may add semantic or hybrid ranking, but they do not replace lexical search or become canonical state.

The semantic runtime uses the vendored FP32 model under `models/gte/`, a Rust inference implementation, a C ABI and a SQLite vector extension. If any semantic component is unavailable, queries use lexical search and accepted writes continue.

## Why

FTS5 is fast, predictable and easy to rebuild. Semantic ranking helps when a query and concept use different words, but it adds a model, platform-specific libraries and versioned embeddings. Those are useful additions, not prerequisites for reading memory.

Keeping semantic data derived also makes model replacement straightforward: change the model ID, mark old embeddings stale and rebuild them from concepts.

## Consequences

* Lexical search is the default mode.
* Embeddings include model, dimension, content hash and repository revision.
* Namespace filtering happens before semantic ranking so hidden concepts do not affect scores.
* Hybrid search combines authorised lexical and semantic candidates with reciprocal-rank fusion.
* The release image includes the GTE model and Rust libraries; semantic search stays off until `intelligent_tiers.semantic_search.enabled` is set.
* ARM64 and AMD64 use the same model format with platform-specific SIMD paths.

## Alternatives considered

* **Semantic-only search:** rejected because model failure would make memory hard to query and exact terms would lose predictable ranking.
* **Remote embedding API:** rejected as the default because shared memory may contain private operational facts and local operation is practical at this model size.
* **Store vectors in concept Markdown:** rejected because vectors are derived, large and tied to a model revision.
