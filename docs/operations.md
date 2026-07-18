# Operations

Memento runs as a single authoritative writer. The daemon is the normal live interface. The local maintenance CLI is for offline or otherwise exclusive operator work.

This document covers Docker, Compose, systemd and reverse-proxy deployments. The examples have local checks; production results are not included.

## Operator decisions

* Start from [`examples/config.v1.json`](../examples/config.v1.json). It is the versioned baseline, and the safest place to diff local changes against.
* Set every principal token through its configured `token_env`. With the example config, `MEMENTO_TOKEN_SMITH` and `MEMENTO_TOKEN_FLINT` are mandatory for `serve` and any MCP-facing workflow.
* Set remote provider credentials only through environment variables named by each endpoint's `api_key_env` field. Do not place secrets in JSON.
* Semantic search path overrides are optional. Use `MEMENTO_FFI_LIBRARY`, `MEMENTO_SQLITE_VECTOR_EXTENSION` and `MEMENTO_GTE_MODEL` only when JSON does not already set those paths.
* Allow query fallback across trust boundaries only when a slot explicitly sets `allow_cross_trust_boundary: true`. Proposal and Dream fallback stay off by default for a reason.

## CLI

* `memento --config CONFIG serve`
* `memento --config CONFIG status [--format json|prometheus]`
* `memento --config CONFIG audit [--path /bundle/path.md]`
* `memento --config CONFIG rebuild-index`
* `memento --config CONFIG backup --output DIR`
* `memento --config CONFIG restore --input DIR [--no-rebuild-derived]`

## Live vs offline operator use

Every local CLI subcommand except `restore` builds a runtime first, and runtime startup acquires the writer lease under `repository.root_path/locks/writer.lock`. That means `status`, `audit`, `rebuild-index` and `backup` all require exclusive access to the repository state. If the daemon is already running, they should fail on lease contention rather than racing it.

Practical rule:

* use `serve` for the live service;
* use MCP `memory_status` or the `memory://status` resource for live health and readiness checks;
* use the local maintenance CLI only while the service is stopped or otherwise guaranteed exclusive.

## Logging

Commands emit structured JSON logs to stderr. JSON command results go to stdout. In Prometheus mode, `status --format prometheus` writes only metrics text to stdout and keeps structured logs on stderr, so a scraper or shell redirect gets clean exposition output.

Common secret-bearing keys such as `authorization`, `token`, `password`, `secret` and `api_key` are redacted, which is the bare minimum for anything that might end up in journald or a central log sink.

## Metrics

`status --format prometheus` emits dependency-free Prometheus text output with these metrics:

* `memento_service_up`
* `memento_control_db_open`
* `memento_index_stale`
* `memento_visible_concepts`
* `memento_proposal_backlog`
* `memento_repo_revision_info`

Because the CLI status path also needs the writer lease, use it for offline inspection or one-shot scrape jobs against a stopped instance. For live status, use MCP.

## Skill-pack storage

Accepted skill ZIPs are Git LFS objects inside the canonical bare repository. Install `git-lfs` anywhere Memento performs accepted skill writes or restores backups. The release container includes it.

Skill submission uses base64 inside MCP JSON. `mcp.max_request_bytes` defaults to 72 MiB, while decoded ZIP content remains capped at 50 MiB and is inspected before proposal storage. Reverse proxies must allow the same request size or skill submission will fail before reaching Memento.

Memento returns recalled ZIPs but does not install them. A client can use `memento.skill_import.import_skill_pack` to import into `.pi/skills/<name>/`; it fails if that destination exists, leaving merge decisions to the client or auditor.

## Backups

Backups contain the canonical bare repository, the control plane SQLite copy and, when present, a copy of `derived.sqlite`.

Operator rules:

* Write backups **outside** `repository.root_path`.
* Use timestamped directories and external retention, for example `BACKUP_ROOT/20260718T231500Z/`.
* Do not treat `derived.sqlite` as the crown jewels. The canonical backup value is `repo.git` plus `control.sqlite`.

Keeping backups outside `repository.root_path` matters for two reasons:

* the state root is what `restore` replaces;
* a backup stored under that root can be deleted by the very restore meant to recover it.

## Restore semantics

`restore` is intentionally destructive. After checksum verification and staging, it renames the existing `repository.root_path` aside, replaces it with the restored state, and removes the previous tree. Materialised `current/` is recreated from the archived bare repository, and `derived.sqlite` is rebuilt by default unless `--no-rebuild-derived` is explicitly requested.

Treat the command as replacing the entire state root, not as merging files into an existing installation.

## Shutdown behaviour

`serve` installs SIGINT and SIGTERM handlers, drains requests, closes the server when a compatible `shutdown`, `aclose` or `close` method exists, and releases both the SQLite control connection and the writer lease on every exit path. That recovery sequencing matters more than a fast stop.

## Worktree housekeeping

Detached worktrees are intentional isolation and recovery artefacts, not disposable copies of `current/`. Startup recovery classifies interrupted operations before removing their worktrees. Do not delete `worktrees/` while the service is running, and do not add cleanup scripts that bypass the operation journal.

The measured local add+remove cost remains below roughly 211 ms at 10,000 small concepts. See [ADR 0001](decisions/0001-keep-operation-worktrees.md) for the decision and alternatives.

## Compose reference

[`compose.example.yaml`](../compose.example.yaml) is a local packaging reference, not a production manifest. It starts the daemon correctly, exposes port 8000 and mounts the example config plus a writable state volume.

Minimal local setup:

```bash
cd /workspace/projects/memento
cp examples/memento.env.example .env
# Edit .env and replace both placeholder bearer tokens.
docker compose -f compose.example.yaml up --build
```

Notes:

* The image bakes in the vendored `gte-small.gtemodel` plus the Rust semantic libraries and exports the matching default environment variables.
* Semantic search still stays disabled unless the config enables it.
* The compose file does not mount a backup destination. If you want offline backups, mount a host path outside the state volume and run them only while the service is stopped.

## systemd reference

[`deploy/systemd/`](../deploy/systemd/) contains hardened reference units for an installed virtualenv layout.

Typical installation steps:

```bash
sudo install -d -m 0755 /opt/memento /etc/memento /var/lib/memento
sudo cp examples/config.v1.json /etc/memento/config.json
sudo cp deploy/systemd/memento.service /etc/systemd/system/
sudo cp deploy/systemd/memento-audit.service /etc/systemd/system/
sudo cp deploy/systemd/memento-audit.timer /etc/systemd/system/
sudo cp deploy/systemd/memento-backup.service /etc/systemd/system/
sudo cp deploy/systemd/memento-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now memento.service
```

Timer safety, as the files exist today:

* `memento-audit.service` and `memento-backup.service` invoke local maintenance commands, so they also require the writer lease.
* `memento-backup.service` writes to `/srv/memento-backups/latest`, outside the configured state root. Reusing `latest` replaces that backup set; use timestamped directories or copy it into retained external storage.
* The timer units are disabled-by-default examples and have no `WantedBy=timers.target` installation hook.
* Do not start either timer alongside the live daemon. Arrange a maintenance window that stops `memento.service`, runs the oneshot unit, then restarts the service.
* Use MCP for live status and reserve audit runs for maintenance windows.

## Deployment references

These artefacts exist and are useful for local packaging checks, but they are still pending production verification:

* Container: [`Dockerfile`](../Dockerfile)
* Compose: [`compose.example.yaml`](../compose.example.yaml)
* systemd: [`deploy/systemd/`](../deploy/systemd/)
* reverse proxy: [`deploy/nginx/memento.conf`](../deploy/nginx/memento.conf)
