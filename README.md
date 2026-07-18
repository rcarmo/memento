# Memento

[`piclaw`][piclaw] instances are deliberately independent. Smith can have one set of conversations, schedules and local Dream memories while Flint has another--which is exactly what you want until both need to remember the same durable fact.

Copying notes between workspaces does not scale. It creates several versions of the truth, makes updates hard to attribute and leaves every instance guessing whether its local copy is current. Sharing Piclaw's message database would be worse: conversations, credentials, reminders and runtime state have different ownership and retention rules.

Memento provides the missing middle ground. It is a standalone Model Context Protocol (MCP) service through which several Piclaw instances can search, read and curate one shared body of durable knowledge without sharing their private operational state.

```text
Smith ──┐
Flint ──┼── authenticated MCP ──> Memento ──> Git Markdown
Others ─┘                              ├──────> control.sqlite
                                      └──────> derived search indexes
```

## What Is Shared

Memento stores concepts: systems, services, projects, people, instances and other facts that should survive individual chats and remain useful to more than one assistant. Each concept is a Markdown file with strict frontmatter, a stable ID and normal links to related concepts.

The repository stays readable without Memento. You can inspect it with a text editor, review changes with Git, restore it on another machine and rebuild every search index from its contents.

Memento does not replace or merge:

* Piclaw conversations and SQLite message history;
* local `notes/daily/` and `notes/memory/` files;
* Dream or AutoDream state;
* reminders and scheduled tasks;
* credentials, keychains or instance configuration.

Those remain private to each Piclaw instance. Shared knowledge has a different job.

## Why Git, SQLite And MCP

Git is authoritative for knowledge. Every accepted mutation produces a commit with the authenticated principal, operation and base revision attached to it. Renames preserve concept IDs and update inbound links in the same transaction.

SQLite is authoritative for operational state: proposals, idempotency keys, write journals, leases and scheduler watermarks. A second rebuildable SQLite database holds FTS5, graph and optional vector indexes.

MCP provides the client boundary. Piclaw instances authenticate as separate principals and receive only the paths their policy permits. Search filtering happens before ranking, which prevents hidden concepts from affecting visible results.

The governing rule is simple:

> Git owns knowledge; SQLite owns operations; search indexes are disposable; models are advisory.

## Reading Shared Memory

The default MCP surface is deliberately small:

* `memory_help` describes goals and points to the on-demand operation catalog;
* `memory_status` reports repository, index and feature readiness;
* `memory_search` performs lexical, semantic or hybrid retrieval;
* `memory_read` returns one authorised concept or section;
* `memory_execute` runs a bounded declarative chain of existing operations;
* `memory_answer` is exposed only when the optional answer tier is enabled.

Detailed contracts live in MCP resources such as `memory://catalog`, `memory://catalog/{operation}` and `memory://workflow/{goal}`. This progressive disclosure keeps eighteen detailed tool schemas out of the model context during ordinary searches. A full compatibility surface remains available through configuration.

`memory_execute` is a constrained operation-plan interpreter rather than arbitrary code execution. It supports typed operations, bounded intermediate values and safe references such as `$matches.results.0.path`. It has no imports, loops, shell, filesystem access or network access, and it permits at most one commit-producing operation in a plan.

## Writing Shared Memory

Cross-instance writes are proposal-first:

```text
search -> read -> propose -> review -> apply -> Git commit -> index update
```

A proposer can describe a change and inspect its deterministic diff. A curator reviews and applies it against an expected repository revision. Stale writes conflict instead of overwriting newer knowledge, and repeating an idempotency key returns the recorded result rather than producing a second commit.

Direct create, patch and rename operations exist for curators and administrators. They use the same authorisation, validation, journalling and compare-and-swap publication path as reviewed proposals. There is no general client-facing hard delete.

## Search

Lexical search uses weighted FTS5 fields for titles, aliases, paths, descriptions, tags and bodies. Graph indexing adds links, backlinks, orphan detection and broken-link reporting.

Optional semantic search uses a local Rust port of GTE-small:

* 384-dimensional, L2-normalised concept embeddings;
* a stable C ABI loaded from Python with `ctypes`;
* packed float32 vectors in the rebuildable derived database;
* a SQLite extension that implements validated cosine ranking;
* scalar plus runtime-selected AMD64 AVX2/FMA and ARM64 NEON kernels;
* deterministic reciprocal-rank fusion for hybrid retrieval.

The reviewed FP32 model is vendored at `rust/tests/fixtures/gte-small.gtemodel` and copied into the container at `/usr/local/share/memento/models/gte-small.gtemodel`. Its SHA-256 digest is `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`; provenance and licensing are recorded in `docs/attribution.md`. If model loading or vector indexing fails, lexical search remains available and semantic readiness becomes degraded; canonical writes still complete.

See [`docs/semantic-search.md`](docs/semantic-search.md) for configuration and packaging details.

## Optional Model Features

Memento remains fully useful without an LLM. Optional, independently gated tiers add:

* a fine-tuned local Needle shallow-router checkpoint and deterministic action expander, currently disabled pending embedded AMD64/ARM64 runtime validation;
* exact answer caching scoped by repository revision and authorisation visibility;
* a small hot working set over recent concepts and accepted answers;
* bounded read-only traversal with validated citations;
* model-assisted proposal drafting, which cannot review or apply its own work;
* Dream graph-health scans in report-only or proposal mode;
* task-specific provider slots with explicit data classifications and model-level fallback.

Models never authenticate clients, choose canonical paths, publish Git refs, approve mutations or claim that persistence succeeded. Deterministic code owns those decisions.

## Safety And Recovery

Only one Memento process may hold the repository writer lease. Each mutation runs in a temporary Git worktree, validates exact changed paths and publishes `main` with compare-and-swap. The materialised checkout and derived indexes advance before the operation is marked successful, giving the submitting client read-your-writes behaviour.

Startup recovery reconciles interrupted journal rows with Git history. The derived database can be deleted and rebuilt. Backups contain the bare Git repository and a consistent SQLite backup with checksums; temporary worktrees, materialised checkouts and search indexes do not need to be preserved.

Tool arguments, Markdown, links, retrieved text and model output are all untrusted. Memento rejects traversal, symlinks, special files, reserved-file writes, oversized changes, stale revisions, namespace violations and likely secrets in model-authored proposals.

## Running It

Memento supports Python 3.12-3.14 and uses a Makefile as the stable development and CI interface:

```bash
make install-dev
make check
make coverage
make build-wheel
```

`make check` validates Python formatting, linting, strict types and tests, then runs Rust formatting, Clippy and the complete Rust workspace tests.

The service CLI provides:

```text
memento-serve --config /etc/memento/config.json serve
memento-serve --config /etc/memento/config.json status
memento-serve --config /etc/memento/config.json audit
memento-serve --config /etc/memento/config.json rebuild-index
memento-serve --config /etc/memento/config.json backup --output /path/to/backup
memento-serve --config /etc/memento/config.json restore --input /path/to/backup
memento-serve --config /etc/memento/config.json dream --mode report_only
```

Docker, Compose/Portainer, nginx and hardened systemd examples are included. The container runs as a non-root user with a read-only root filesystem, writable state under `/var/lib/memento`; the default GTE-small model is already installed read-only in the image.

Start with [`examples/config.v1.json`](examples/config.v1.json), then read [`docs/operations.md`](docs/operations.md) before enabling writes.

## Project State

The deterministic repository, transaction journal, FTS/graph indexes, authenticated MCP service, proposal workflow, backup/restore tooling, compact tool catalog, bounded executor, optional model tiers and Rust semantic-search runtime are implemented and covered by local tests.

Local validation includes Python 3.12-3.14, wheel installation, container builds, authenticated Streamable HTTP calls through Piclaw's bundled MCP SDK, crash-boundary recovery tests and Rust FFI/SQLite-extension parity tests.

Published SBOM/provenance, production image digests, live Docker/systemd parity and a clean-host production restore drill still require deployment evidence. They are tracked in [`PLAN.md`](PLAN.md) and are not presented as completed work. Repository-owned local load testing is documented in [`docs/load-testing.md`](docs/load-testing.md); its thresholds are local checks, not universal service SLOs.

## Documentation

* [`PLAN.md`](PLAN.md) tracks implementation and acceptance evidence.
* [`docs/implementation.md`](docs/implementation.md) contains the complete architecture and roadmap.
* [`docs/decisions/`](docs/decisions/0001-keep-operation-worktrees.md) records consequential design decisions and measurements, including the [Needle local-model feasibility study](docs/decisions/0002-needle-feasibility.md).
* [`docs/contracts.md`](docs/contracts.md) defines schemas, envelopes and MCP operations.
* [`docs/threat-model.md`](docs/threat-model.md) records trust boundaries and abuse cases.
* [`docs/semantic-search.md`](docs/semantic-search.md) covers the Rust GTE and SQLite vector tier.
* [`docs/operations.md`](docs/operations.md) covers deployment, health, backup and recovery.
* [`docs/load-testing.md`](docs/load-testing.md) covers the repository-owned load harness and local thresholds.
* [`docs/evidence/`](docs/evidence/README.md) contains reviewed local operational, HTTP and semantic reports.
* [`AGENTS.md`](AGENTS.md) defines contribution and validation rules.

Memento is MIT licensed. Third-party runtime and model attribution is recorded in [`docs/attribution.md`](docs/attribution.md).

[piclaw]: https://github.com/rcarmo/piclaw
