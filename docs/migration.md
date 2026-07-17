# Migration

Memento upgrades are conservative by design: preserve the canonical repository and control plane first, then rebuild anything derived. The procedure below is covered by local rebuild and restore tests, but still needs live verification in production.

## Upgrade procedure

* `memento --config CONFIG backup --output BACKUP_DIR`
* Stop the service.
* Install the new wheel or container image.
* Start the new service.
* Run `memento --config CONFIG status`.
* Run `memento --config CONFIG rebuild-index` if the derived index revision is stale or quarantined.

## What changes safely

* `control.sqlite` uses explicit schema version checks and rejects unknown versions. There is no implicit best-effort migration path, which is exactly what you want when state gates leases, idempotency and recovery.
* `derived.sqlite` is rebuildable from Git. Treat it as disposable when quarantine, restore or version drift leaves any doubt.

## Pending verification

Wheel upgrades, container-image swaps and post-upgrade service health have local test coverage. Production orchestration behaviour is still pending verification.
