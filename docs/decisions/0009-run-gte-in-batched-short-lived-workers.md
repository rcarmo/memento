# ADR 0009: Run GTE in batched short-lived workers

**Status:** accepted

**Date:** 2026-07-19

**Amended:** 2026-07-31

## Decision

Memento runs GTE embedding inference in a separate `memento-embed` process by default. Path-loaded GTE1 model weights use a read-only memory mapping where the file layout, alignment and host byte order permit direct access; byte-loaded fixtures and incompatible layouts retain owned storage.

Concept embedding is asynchronous. Canonical Git writes and the lexical and graph indexes advance without waiting for GTE. The generic worker supports batches bounded by `semantic_search.max_batch_size`. The amended DiskStation profile derives pending paths from persisted `derived.sqlite` state and runs one concept per short-lived worker at `nice 15`, with one native thread, startup/interactive/pacing gates and sampled CPU-utilization load shedding.

Semantic revision state records any lag between the repository and its embeddings. Semantic and hybrid searches do not treat stale rows as current; they fall back to lexical results with an explicit warning until the worker catches up.

Query embeddings use the same subprocess boundary. Deployments may explicitly select the in-process FFI client where repeated low-latency semantic queries matter more than idle memory, but this is not the DiskStation default.

## Why

GTE-small expands its weights and allocates inference scratch space. Keeping its FFI handle in the service made that memory part of Memento's idle footprint, which was a poor fit for the initial 320 MiB DiskStation profile. The deployed profile now uses a 512 MiB limit but still relies on worker exit to reclaim model memory.

A read-only mapping avoids copying the complete model file into anonymous memory and allows the kernel to discard file-backed pages under pressure. It cannot guarantee that decoded tensors, allocator arenas and scratch buffers are returned after closing an in-process model handle. Worker exit is the reliable release boundary.

Batches amortise model startup and remain useful on hosts with spare CPU. Live DiskStation operation showed that minimizing contention matters more than throughput there, so its progressive profile intentionally accepts repeated model startup to process one concept at a time without monopolizing CPU.

## Consequences

* The main service does not keep GTE weights resident when subprocess mode is selected.
* One worker loads or maps the model once for each configured batch and exits after the response; the DiskStation batch size is one.
* Multiple writes may collapse into one pass over the newest repository revision rather than embedding intermediate revisions.
* Vector rows are published atomically per batch and remain rebuildable derived state in persistent `derived.sqlite`; compatible rows survive rebuilds and image upgrades.
* Status reports the indexed repository revision and embedding revision separately.
* Shutdown stops new batches and joins or cancels the active worker without weakening canonical Git durability.
* Cold query latency is higher in subprocess mode because a query may start a worker. Operators who need consistently low semantic-query latency can opt into the in-process FFI client and budget for resident GTE memory.
* The mmap path requires strict bounds and alignment checks. Unsupported layouts fall back to owned tensors rather than using an unsafe cast.
* Lexical search remains available while embeddings are pending, stale or unavailable, as required by [ADR 0006](0006-keep-lexical-search-primary.md).

## Alternatives considered

* **Keep GTE loaded in the service:** rejected as the default because its idle anonymous memory competes with Needle, SQLite and the Python service on the NAS.
* **Close and reopen an in-process FFI handle:** rejected as the memory-release mechanism because the allocator may retain decoded weights and scratch arenas.
* **Map the file but copy every tensor into vectors:** rejected for path-loaded production models because it preserves most of the anonymous-memory cost. It remains the safe fallback for incompatible input layouts.
* **Start one worker for every concept:** originally rejected as the universal default; adopted for the constrained DiskStation progressive profile after deployment evidence showed that predictable low contention mattered more than batch throughput.
* **Block each canonical write until embeddings finish:** rejected because a local inference failure must not hold up Git, FTS5 or graph updates.
* **Keep one subprocess alive indefinitely:** rejected as the DiskStation default because it recreates the idle-memory problem across a process boundary. A future bounded idle timeout may be useful for higher query rates, but it is not required for this deployment profile.
