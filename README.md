# Memento

Memento will provide several Piclaw instances with shared, durable knowledge over the Model Context Protocol (MCP). Git-backed Markdown will be authoritative for knowledge, SQLite will track operations, and rebuildable FTS5 and graph indexes will support retrieval.

The project is at **Milestone 0: contracts and repository bootstrap**. No service capability is implemented yet.

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

## Development

Memento uses a `src/` layout and supports Python 3.10–3.12.

```bash
make install-dev
make check
make coverage
```

Use Make targets as the stable local and CI interface.
