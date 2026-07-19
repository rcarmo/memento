# ADR 0009: Run GTE in batched short-lived workers

**Status:** accepted  
**Date:** 2026-07-19

## Decision

Memento runs GTE embedding inference in a separate `memento-embed` process by default. Path-loaded GTE1 model weights use a read-only memory mapping where the file layout, alignment and host byte order permit direct access; byte-loaded fixtures and incompatible layouts retain owned storage.

Concept embedding is asynchronous. Canonical Git writes and the lexical and graph indexes advance without waiting for GTE. A coalescing background queue scans the latest repository revision, deduplicates changed content and drains work in batches bounded by `semantic_search.max_batch_size`. Each batch starts one worker, submits one `embed_batch` request, commits the returned vectors in one SQLite transaction and lets the process exit.

Semantic revision state records any lag between the repository and its embeddings. Semantic and hybrid searches do not treat stale rows as current; they fall back to lexical results with an explicit warning until the worker catches up.

Query embeddings use the same subprocess boundary. Deployments may explicitly select the in-process FFI client where repeated low-latency semantic queries matter more than idle memory, but this is not the DiskStation default.

## Why

GTE-small expands its weights and allocates inference scratch space. Keeping its FFI handle in the service made that memory part of Memento's idle footprint, which is a poor fit for the initial DiskStation target and its 320 MiB container limit.

A read-only mapping avoids copying the complete model file into anonymous memory and allows the kernel to discard file-backed pages under pressure. It cannot guarantee that decoded tensors, allocator arenas and scratch buffers are returned after closing an in-process model handle. Worker exit is the reliable release boundary.

Embedding concepts one at a time would pay model startup repeatedly and waste the worker's batch API. Coalescing repository changes and embedding them in bounded batches amortises startup while keeping canonical writes independent of model availability.

## Consequences

* The main service does not keep GTE weights resident when subprocess mode is selected.
* One worker loads or maps the model once for each concept batch, embeds all items in that batch and exits after the response.
* Multiple writes may collapse into one pass over the newest repository revision rather than embedding intermediate revisions.
* Vector rows are published atomically per batch and remain rebuildable derived state in `derived.sqlite`.
* Status reports the indexed repository revision and embedding revision separately.
* Shutdown stops new batches and joins or cancels the active worker without weakening canonical Git durability.
* Cold query latency is higher in subprocess mode because a query may start a worker. Operators who need consistently low semantic-query latency can opt into the in-process FFI client and budget for resident GTE memory.
* The mmap path requires strict bounds and alignment checks. Unsupported layouts fall back to owned tensors rather than using an unsafe cast.
* Lexical search remains available while embeddings are pending, stale or unavailable, as required by [ADR 0006](0006-keep-lexical-search-primary.md).

## Alternatives considered

* **Keep GTE loaded in the service:** rejected as the default because its idle anonymous memory competes with Needle, SQLite and the Python service on the NAS.
* **Close and reopen an in-process FFI handle:** rejected as the memory-release mechanism because the allocator may retain decoded weights and scratch arenas.
* **Map the file but copy every tensor into vectors:** rejected for path-loaded production models because it preserves most of the anonymous-memory cost. It remains the safe fallback for incompatible input layouts.
* **Start one worker for every concept:** rejected because the existing protocol supports batches and model startup should be amortised.
* **Block each canonical write until embeddings finish:** rejected because a local inference failure must not hold up Git, FTS5 or graph updates.
* **Keep one subprocess alive indefinitely:** rejected as the DiskStation default because it recreates the idle-memory problem across a process boundary. A future bounded idle timeout may be useful for higher query rates, but it is not required for this deployment profile.
