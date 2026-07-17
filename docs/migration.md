# Migration

Status: documented and covered by local rebuild/restore tests; **not live verified**.

## Upgrade procedure

1. `memento --config CONFIG backup --output BACKUP_DIR`
2. Stop the service.
3. Install the new wheel or container image.
4. Start the new service.
5. Run `memento --config CONFIG status`.
6. Run `memento --config CONFIG rebuild-index` if the derived index revision is stale or quarantined.

## Control DB

The control database uses explicit schema version checks and rejects unknown versions rather than applying unsafe implicit migrations.

## Derived DB

The derived database is rebuildable from Git. Treat it as disposable if restore or quarantine is required.
