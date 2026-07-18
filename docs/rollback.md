# Rollback

Rollback is only safe if operators restore the canonical repository and control state together. Rebuild the derived index afterwards unless there is a very good reason not to. The flow below is covered by local restore tests, but still needs production verification.

## Safe rollback

* Stop the service.
* Select a known-good backup directory.
* Run `memento --config CONFIG restore --input BACKUP_DIR`.
* Start the service.
* Validate with `memento --config CONFIG audit` and `memento --config CONFIG status`.

## What `restore` does

`restore` verifies backup checksums, restores the bare Git repository, restores the control SQLite database with the SQLite backup API output, materialises `current/`, and rebuilds `derived.sqlite` by default. That sequencing is the point: canonical state first, derived state afterwards.

The command is destructive. It stages the recovered state, renames the existing `repository.root_path` aside, replaces it with the staged tree and removes the previous root. Do not keep backup sets under `repository.root_path` unless you are comfortable losing them during recovery.

## Operator notes

* `restore` itself does not reuse a running runtime, but the replacement it performs assumes the daemon is stopped.
* The post-restore `audit` and `status` commands require the writer lease, so keep the instance offline until those checks finish.
* If you pass `--no-rebuild-derived`, you are explicitly choosing to trust the archived `derived.sqlite`. The safer default is to let Memento rebuild it.

## Pending verification

The command path and recovery ordering are tested locally. Service-manager and production-storage behaviour remain pending live verification.
