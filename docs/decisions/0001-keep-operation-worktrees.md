# ADR 0001: Keep detached worktrees for canonical mutations

**Status:** accepted  
**Date:** 2026-07-17

## Decision

Memento keeps assembling each commit-capable mutation in a detached Git worktree. It does not mutate the reader-visible `current/` checkout directly, and it does not replace the filesystem mutation path with a temporary Git index or in-memory tree builder.

## Why

The mutation layer operates on a real directory tree. Create and patch operations parse and rewrite files. Rename operations unlink the old path, create the new path and scan Markdown files to update inbound links. A detached worktree gives those operations a complete filesystem snapshot without exposing intermediate state to readers.

The worktree also survives a process crash. Startup recovery can inspect its `HEAD`, compare it with canonical `main`, classify the operation and remove the worktree only after the outcome is known. A temporary `GIT_INDEX_FILE` would be faster for a handful of blob replacements, but it would also mean replacing the existing path-safety, rename, link-rewrite and recovery code with Git plumbing plus more durable metadata.

Writing directly into `current/` would remove checkout overhead at the cost of visible half-applied changes and a dirty reader-facing tree after a crash. That is not a sensible trade.

## Measured cost

A local Linux x86_64 benchmark created and removed 20 detached worktrees from repositories containing small Markdown files:

| Concepts | p50 add+remove | p95 add+remove | Maximum |
|---:|---:|---:|---:|
| 100 | 6.73 ms | 41.33 ms | 41.66 ms |
| 1,000 | 23.25 ms | 49.59 ms | 50.97 ms |
| 10,000 | 180.40 ms | 210.12 ms | 211.42 ms |

The documented initial ceiling is 10,000 concepts, and writes are serialized by design. Around 180 ms median isolation cost at that ceiling is acceptable beside parsing, validation, commit publication and index updates. This is a local engineering measurement, not a production service-level objective.

## Alternatives considered

* **Mutate `current/` under the writer lock:** rejected because the lock does not protect concurrent readers, and a crash leaves visible partial state.
* **Temporary Git index and `commit-tree`:** rejected for now because current mutations need whole-tree filesystem access and recovery would need a second durable state model.
* **In-memory tree builder:** rejected as a large rewrite with no demonstrated need.
* **Unregistered temporary checkout:** rejected because it keeps checkout cost while discarding Git-managed worktree state used by recovery.

## Separate follow-up

`materialize_current_checkout()` currently removes and repopulates `current/` after publishing a commit. Concurrent readers may therefore observe an empty or partially populated directory during refresh. This is independent of operation worktrees.

A future change should materialise immutable revision snapshots and atomically switch the reader-visible revision, or use another platform-safe indirection. That work needs to preserve read-your-writes behaviour and Windows/POSIX semantics. It does not require changing how mutation commits are assembled.
