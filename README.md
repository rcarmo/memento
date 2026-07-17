# Memento

Memento will provide several Piclaw instances with shared, durable knowledge over the Model Context Protocol (MCP). Git-backed Markdown will be authoritative for knowledge, SQLite will track operations, and rebuildable FTS5 and graph indexes will support retrieval.

The project is at **Milestone 6: production operations (implementation complete, deployment examples not live verified)**, with deferred intelligent tiers 1–4 now implemented behind independent feature flags. Deterministic repository primitives, Git publication, derived indexing, read/write service flows and local production-operations tooling are implemented.

## Core rules

- The service is the sole canonical repository writer.
- Deterministic code owns identity, authorization, validation, concurrency, persistence and audit.
- Clients propose changes before curators apply them.
- Models are optional and advisory; they never write canonical knowledge directly.
- Piclaw conversations, local memory, schedules and secrets remain outside Memento.

See:

- [PLAN.md](PLAN.md) for the executable delivery plan;
- [docs/implementation.md](docs/implementation.md) for the full architecture and roadmap;
- [AGENTS.md](AGENTS.md) for repository contribution rules.

## Implemented now

- strict concept schema v1 with Pydantic v2 validation
- versioned JSON config loading and composition root runtime assembly
- standard MCP-style success and error envelopes
- safe repository path containment for writes and reads
- frontmatter parsing and deterministic Markdown serialization
- Markdown structural link extraction and safe rename rewriting
- deterministic directory index and root log generation
- bundle scan, concept read and repository audit
- SQLite WAL control-plane schema v1 for operations, proposals, scheduler runs and service state
- principal-scoped durable idempotency records
- POSIX writer lease, bare Git bootstrap, temporary worktrees and compare-and-swap publication of `main`
- materialized `current/` checkout, startup recovery classification and deterministic transaction checkpoints
- rebuildable derived FTS/graph index with parity check and quarantine handling
- MCP service read tools, `memory_answer`, proposal workflow and curator write pipeline
- operational CLI commands: `serve`, `audit`, `rebuild-index`, `backup`, `restore`, `status`
- structured JSON logs with redaction and dependency-free Prometheus text metrics output
- backup/restore support for bare Git and SQLite with manifest checksums
- Docker, Compose, systemd and reverse-proxy examples
- CI workflow for Python 3.10-3.12, wheel build/install and container build validation
- feature-gated deep read-only answers, exact answer cache and hot working memory with bounded trace persistence
- feature-gated model-assisted proposal drafting through `memory_propose_freeform` and `memory_propose_update`, with deterministic search context, strict citations, diff validation and secret blocking
- sample bundle and contract/threat/operations documentation

## Development

Memento uses a `src/` layout and supports Python 3.10–3.12.

```bash
make install-dev
make check
make coverage
make build-wheel
make install-wheel
make diff-check
```

## Operations

- example config: [`examples/config.v1.json`](examples/config.v1.json)
- MCP/data contracts: [`docs/contracts.md`](docs/contracts.md)
- operations guide: [`docs/operations.md`](docs/operations.md)
- migration guide: [`docs/migration.md`](docs/migration.md)
- rollback guide: [`docs/rollback.md`](docs/rollback.md)
- release guide: [`docs/release.md`](docs/release.md)

Deployment examples are provided for local review and CI build validation, but are **not claimed as live verified**.

## Sample bundle

A minimal audited sample bundle lives under [`sample-bundle/`](sample-bundle/).

Use Make targets as the stable local and CI interface.
