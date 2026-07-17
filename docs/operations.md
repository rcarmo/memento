# Operations

Memento runs local operator workflows today. Docker, Compose, systemd and reverse-proxy deployments are documented here, but still need production verification.

## Operator decisions

* Start from [`examples/config.v1.json`](../examples/config.v1.json). It is the versioned baseline, and the safest place to diff local changes against.
* Set remote provider credentials only through environment variables named by each endpoint's `api_key_env` field. Do not place secrets in JSON.
* Allow query fallback across trust boundaries only when a slot explicitly sets `allow_cross_trust_boundary: true`. Proposal and Dream fallback stay off by default for a reason.

## CLI

* `memento --config CONFIG serve`
* `memento --config CONFIG status [--format json|prometheus]`
* `memento --config CONFIG audit [--path /bundle/path.md]`
* `memento --config CONFIG rebuild-index`
* `memento --config CONFIG backup --output DIR`
* `memento --config CONFIG restore --input DIR [--no-rebuild-derived]`

## Logging

`serve` and the other CLI commands emit structured JSON logs to stdout. Common secret-bearing keys such as `authorization`, `token`, `password` and `secret` are redacted, which is the bare minimum for anything that might end up in journald or a central log sink.

## Metrics

`status --format prometheus` emits dependency-free Prometheus text output with these metrics:

* `memento_service_up`
* `memento_control_db_open`
* `memento_index_stale`
* `memento_visible_concepts`
* `memento_proposal_backlog`
* `memento_repo_revision_info`

## Shutdown behaviour

`serve` installs SIGINT and SIGTERM handlers, drains requests, closes the server when a compatible `shutdown`, `aclose` or `close` method exists, and releases both the SQLite control connection and the writer lease on every exit path. That recovery sequencing matters more than a fast stop.

## Worktree housekeeping

Detached worktrees are intentional isolation and recovery artefacts, not disposable copies of `current/`. Startup recovery classifies interrupted operations before removing their worktrees. Do not delete `worktrees/` while the service is running, and do not add cleanup scripts that bypass the operation journal.

The measured local add+remove cost remains below roughly 211 ms at 10,000 small concepts. See [ADR 0001](decisions/0001-keep-operation-worktrees.md) for the decision and alternatives.

## Deployment references

These artefacts exist and are useful for local packaging checks, but they are still pending production verification:

* Container: [`Dockerfile`](../Dockerfile)
* Compose: [`compose.example.yaml`](../compose.example.yaml)
* systemd: [`deploy/systemd/`](../deploy/systemd/)
* reverse proxy: [`deploy/nginx/memento.conf`](../deploy/nginx/memento.conf)
