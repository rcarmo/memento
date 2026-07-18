# Migration

Memento upgrades are conservative by design: preserve the canonical repository and control plane first, then rebuild anything derived. The procedure below is covered by local rebuild and restore tests, but still needs live verification in production.

## Upgrade procedure

* `memento --config CONFIG backup --output BACKUP_DIR`
* Stop the service.
* Install the new wheel or container image.
* Start the new service.
* Run `memento --config CONFIG status`.
* Run `memento --config CONFIG rebuild-index` if the derived index revision is stale or quarantined.

## Operator notes

* `backup`, `status` and `rebuild-index` all require the writer lease. Run them only while the service is stopped or otherwise guaranteed exclusive.
* For a live pre-upgrade readiness check, use MCP `memory_status` or the `memory://status` resource before taking the daemon offline.
* Keep `BACKUP_DIR` outside `repository.root_path`, and prefer a timestamped destination so retention is explicit and restores do not wipe your backup set.
* `restore` replaces the full state root. If an upgrade fails badly enough to need rollback, assume the restore path is destructive and plan storage layout accordingly.

## What changes safely

* `control.sqlite` uses explicit schema version checks and rejects unknown versions. There is no implicit best-effort migration path, which is exactly what you want when state gates leases, idempotency and recovery.
* `derived.sqlite` is rebuildable from Git. Treat it as disposable when quarantine, restore or version drift leaves any doubt.
* The vendored semantic model and Rust libraries are deployment artefacts, not canonical state. Changing them can force semantic re-indexing, but it must not put repository recovery at risk.

## Pending verification

Wheel upgrades, container-image swaps and post-upgrade service health have local test coverage. Production orchestration behaviour is still pending verification.
