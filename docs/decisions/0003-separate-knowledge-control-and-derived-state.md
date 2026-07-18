# ADR 0003: Separate knowledge, control state and derived indexes

**Status:** accepted  
**Date:** 2026-07-18

## Decision

Memento keeps three kinds of state separate:

* Git stores concepts, accepted asset packs and their history.
* `control.sqlite` stores proposals, pending asset bytes, operations, idempotency records, leases and scheduler state.
* `derived.sqlite` stores FTS, graph data, embeddings and caches that can be rebuilt from Git.

A backup needs the bare Git repository and a consistent copy of `control.sqlite`. The readable checkout and derived database are recreated during restore.

## Why

The states have different recovery rules. Knowledge must be readable and versioned independently of the service. Proposal and operation records must survive retries and crashes but do not belong in concept Markdown. Search indexes are expensive enough to cache and cheap enough to rebuild.

Putting all three in Git would turn operational bookkeeping into knowledge commits. Putting concepts only in SQLite would make Memento the sole way to read or recover them. Treating indexes as canonical would complicate model upgrades and corruption recovery.

## Consequences

* Accepted writes are not reported as successful until Git, the readable checkout and derived revision agree.
* Deleting `derived.sqlite` loses performance, not knowledge.
* Git alone does not preserve pending proposals or idempotency records; backups include `control.sqlite`.
* Schema migrations for control state are explicit and reject unknown versions.
* Accepted ZIP assets use Git LFS but remain part of the canonical Git history.

## Alternatives considered

* **Store everything in Git:** rejected because leases, retries and proposal state would create noisy operational commits.
* **Store everything in SQLite:** rejected because concepts and history should remain accessible with ordinary Markdown and Git tools.
* **Treat the search database as canonical:** rejected because indexes depend on model and schema versions and must be replaceable.
