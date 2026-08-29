# Operations

The state boundaries used by backup and recovery are recorded in [ADR 0003](decisions/0003-separate-knowledge-control-and-derived-state.md).

Memento runs as a single authoritative writer. The daemon is the normal live interface. The local maintenance CLI is for offline or otherwise exclusive operator work.

This document covers Docker, Compose, systemd and reverse-proxy deployments. The DiskStation container profile is live; the generic Compose, systemd and reverse-proxy files remain reference configurations.

## Operator decisions

* Start from [`examples/config.v1.json`](../examples/config.v1.json). It is the versioned baseline, and the safest place to diff local changes against.
* Set `MEMENTO_ADMIN_MASTER_KEY` and the configured bootstrap/recovery principal tokens before first managed-access startup. The example imports `MEMENTO_TOKEN_SANDBOX_BOOTSTRAP` and `MEMENTO_TOKEN_WORK_AGENT_BOOTSTRAP`; managed control-database principals become authoritative afterwards.
* Set remote provider credentials only through environment variables named by each endpoint's `api_key_env` field. Do not place secrets in JSON.
* Semantic search path overrides are optional. Use `MEMENTO_FFI_LIBRARY`, `MEMENTO_SQLITE_VECTOR_EXTENSION` and `MEMENTO_GTE_MODEL` only when JSON does not already set those paths.
* `memory_answer` is discoverable only when both `mcp.compact_answer_enabled` and `intelligent_tiers.deep_answers.enabled` are true. Enabling the compact tool without the deep-answer tier does not expose a half-configured answer path.
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

## Asset-pack storage

Accepted asset ZIPs are ordinary blobs inside the canonical bare repository. Existing repositories with hydrated legacy pointer-backed assets are migrated once at startup; keep the old hydrated checkout available until that migration commits.

Asset submissions should use `attach_asset_pack.zip_base64` when the complete JSON request fits the MCP ceiling. Piclaw agents use `memory_asset_stage_begin`, raw ZIP upload with the one-time `X-Memento-Upload-Ticket`, and `memory_asset_stage_status` only for larger packs or deliberate raw binary HTTP upload. Clients that already manage bearer authentication may use `POST /assets/staging` directly. `mcp.max_request_bytes` defaults to 72 MiB, while decoded ZIP content is capped at 50 MiB and inspected before storage. Reverse proxies must allow the staging body size and preserve `Authorization`, `Idempotency-Key`, `X-Memento-Asset-Kind`, `X-Memento-Asset-Version` and `X-Memento-Upload-Ticket` headers.

Memento returns recalled ZIPs but does not install them. For skill packs, `memento-skill-import` imports into `.pi/skills/<name>/` and fails if that destination exists.

## Backups

Backups contain the canonical bare repository, the control plane SQLite copy and, when present, a copy of `derived.sqlite`.

Operator rules:

* Write backups **outside** `repository.root_path`.
* Use timestamped directories and external retention, for example `BACKUP_ROOT/20260718T231500Z/`.
* `derived.sqlite` is not canonical, but preserving it avoids expensive semantic regeneration. Include it in routine backups when practical; `repo.git` plus `control.sqlite` remain the indispensable recovery set.

Keeping backups outside `repository.root_path` matters for two reasons:

* the state root is what `restore` replaces;
* a backup stored under that root can be deleted by the very restore meant to recover it.

## Restore semantics

`restore` is intentionally destructive. After checksum verification and staging, it renames the existing `repository.root_path` aside, replaces it with the restored state, and removes the previous tree. Materialised `current/` is recreated from the archived bare repository. `derived.sqlite` is rebuilt by default unless `--no-rebuild-derived` is explicitly requested; use that flag when restoring a compatible derived backup so progressive generation resumes from persisted vectors.

Treat the command as replacing the entire state root, not as merging files into an existing installation.

## Upgrades

1. Stop the service and create a backup outside `repository.root_path`.
2. Install the new wheel or container image.
3. Start Memento and check `memory_status`.
4. Run `rebuild-index` offline only if the lexical/graph index is stale or quarantined. Routine image upgrades preserve `derived.sqlite`; a rebuild reuses compatible embeddings and progressively regenerates only gaps.

Control database migrations reject unknown schema versions. Model changes mark incompatible embedding rows stale for progressive regeneration but do not change canonical Git history.

## Rollback

Stop the service, select a known-good backup, and run:

```bash
memento-serve --config CONFIG restore --input BACKUP_DIR
memento-serve --config CONFIG audit
memento-serve --config CONFIG status
```

Restore verifies checksums, restores the bare repository and control database together, materialises the checkout, and rebuilds `derived.sqlite` unless `--no-rebuild-derived` is given. Preserve a compatible derived backup when minimizing post-restore embedding work matters. Keep the service stopped until audit and status checks finish.

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

* [`Dockerfile`](../Dockerfile) publishes the tested non-root amd64/arm64 image. The operator-managed DiskStation deployment pins a release tag and persists `/var/lib/memento`.
* [`deploy/diskstation.compose.yaml`](../deploy/diskstation.compose.yaml) is the live trusted-LAN profile; [`docs/diskstation.md`](diskstation.md) records its J3455 limits and update process. The [`0.3.26` acceptance record](evidence/release-0.3.26.md) includes the current persistent-session and SSE checks, preserved derived state and unresolved production PIDs-limit discrepancy.
* [`compose.example.yaml`](../compose.example.yaml) is the local packaging reference.
* [`deploy/systemd/`](../deploy/systemd/) contains lease-aware reference units that still need an operator-run parity exercise.
* [`deploy/nginx/memento.conf`](../deploy/nginx/memento.conf) is a reverse-proxy reference. TLS and deployment-specific authentication remain operator responsibilities.

## Access operations

Set `MEMENTO_ADMIN_MASTER_KEY` as a strong container secret before first production startup. The example value `nenhuma` exists only to make an isolated trusted-LAN bootstrap obvious and must not survive setup. Open `/admin` with the preserved bootstrap credential; it authenticates as `sandbox`. Back up `control.sqlite` and the master key together. Rotate the key only with the explicit one-shot `rotate-master-key` container command documented in [Access Management](access-management.md); rotation is not available through HTTP or MCP.
