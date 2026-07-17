# Operations

Status: implemented in code and tests for local workflows; **not live verified** on Docker, Compose, systemd or reverse proxies.

## Config

Use [`examples/config.v1.json`](../examples/config.v1.json) as the versioned starting point.

Provider endpoints are configured per slot under `intelligent_tiers.model_provider_slots`. Supply any remote credentials only through environment variables named by each endpoint's `api_key_env` field. Query fallback may cross trust boundaries only when the slot explicitly sets `allow_cross_trust_boundary: true`; proposal and Dream fallback remain disabled by default.

## CLI

- `memento --config CONFIG serve`
- `memento --config CONFIG status [--format json|prometheus]`
- `memento --config CONFIG audit [--path /bundle/path.md]`
- `memento --config CONFIG rebuild-index`
- `memento --config CONFIG backup --output DIR`
- `memento --config CONFIG restore --input DIR [--no-rebuild-derived]`

## Logging

The CLI emits structured JSON logs to stdout. Common secret-bearing keys such as `authorization`, `token`, `password` and `secret` are redacted.

## Metrics

`status --format prometheus` renders dependency-free Prometheus text metrics:

- `memento_service_up`
- `memento_control_db_open`
- `memento_index_stale`
- `memento_visible_concepts`
- `memento_proposal_backlog`
- `memento_repo_revision_info`

## Graceful shutdown

`serve` installs SIGINT/SIGTERM handlers, requests drain, closes the server when a compatible `shutdown`/`aclose`/`close` method exists, and closes the SQLite control connection and writer lease in all command paths.

## Deployment examples

- Container: [`Dockerfile`](../Dockerfile)
- Compose: [`compose.example.yaml`](../compose.example.yaml)
- systemd: [`deploy/systemd/`](../deploy/systemd/)
- reverse proxy: [`deploy/nginx/memento.conf`](../deploy/nginx/memento.conf)
