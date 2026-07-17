# Memento

Memento will provide several Piclaw instances with shared, durable knowledge over the Model Context Protocol (MCP). Git-backed Markdown will be authoritative for knowledge, SQLite will track operations, and rebuildable FTS5 and graph indexes will support retrieval.

The project is at **Milestone 1: deterministic repository core**. Contracts and the first deterministic repository primitives are implemented.

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
- strict service, authorization and principal configuration models
- standard MCP-style success and error envelopes
- safe repository path containment for writes and reads
- frontmatter parsing and deterministic Markdown serialization
- Markdown structural link extraction and safe rename rewriting
- deterministic directory index and root log generation
- bundle scan, concept read and repository audit
- sample bundle and contract/threat documentation

## Development

Memento uses a `src/` layout and supports Python 3.10–3.12.

```bash
make install-dev
make check
make coverage
```

## Sample bundle

A minimal audited sample bundle lives under [`sample-bundle/`](sample-bundle/).

Use Make targets as the stable local and CI interface.
