# Rollback

Status: documented and covered by local restore tests; **not live verified**.

## Safe rollback

1. Stop the service.
2. Select a known-good backup directory.
3. Run `memento --config CONFIG restore --input BACKUP_DIR`.
4. Start the service.
5. Validate with `memento --config CONFIG audit` and `memento --config CONFIG status`.

`restore` verifies backup checksums, restores the bare Git repository, restores the control SQLite database with the SQLite backup API output, materializes `current/`, and rebuilds `derived.sqlite` by default.
