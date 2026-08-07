# Memento

![Memento](docs/icon_256.png)

I wrote Memento to let several [`piclaw`][piclaw] instances share facts without sharing their chats, reminders, credentials or machine-specific notes. One looks after personal work, another deals with servers, and project agents come and go; they all need to know things like where a service runs, why one system replaced another and which machine a project depends on.

Memento gives them an authenticated MCP service over a repository of Markdown concepts. Agents can search, follow links, read a concept and propose changes. Curators can review those proposals and publish them to Git.

```text
Personal agent --\
Server agent -----+-- authenticated MCP --> Memento --> Markdown in Git
Project agent ---/                          |          operation journal
                                             `-------- search and graph indexes
```

Concepts have stable IDs, structured metadata and ordinary Markdown links. Read them with a text editor, inspect their history with Git or rebuild the indexes from the checkout. `control.sqlite` keeps operations, proposals, dynamic principals, credential verifiers and access activity; `derived.sqlite` holds FTS5, backlinks, graph metrics and optional embeddings.

## What Belongs Here

Shared memory is for facts that should outlive a conversation and be useful to more than one agent:

* where a service runs and who owns it;
* why one system replaced another;
* relationships between people, projects, machines and services;
* aliases, tags and links that make the same fact easier to find later;
* reviewed operating knowledge that several agents should follow.

Chat transcripts, daily notes, reminders, schedules, passwords and tokens stay with the agent or machine that owns them.

## Using It

The common read path is short:

```text
search -> read -> follow links if needed
```

The compact MCP surface exposes those frequent operations and keeps less common schemas in `memory://catalog` and `memory://workflow/{goal}`. `memory_execute` can chain known operations using saved results, such as searching for a project and reading the first match.

Writes normally go through review:

```text
search -> read -> propose -> review -> apply -> Git commit -> index update
```

Memento checks the caller's namespace, the expected repository revision and the request's idempotency key. A retry returns the recorded result instead of creating another commit. Curators may review proposals they authored when they have write access to every affected path; `author_principal` and `reviewed_by` preserve that fact in the audit trail. Curators can also create, patch and rename concepts directly when those tools are exposed. Memento has no client-facing hard delete.

Administrators manage principals through the preset-driven [`/admin`](docs/access-management.md) UI or role-filtered `access_*` tools on the same `/mcp` endpoint. Ordinary principals cannot discover or invoke those tools. New and rotated credentials are shown once; only verifiers are retained.

The complete tool contracts, roles, limits and response envelopes are in [`docs/contracts.md`](docs/contracts.md).

## Search, Links And Local Models

FTS5 handles exact and lexical search. Markdown links supply backlinks and graph neighbourhoods. Neither needs a model.

GTE-small can add semantic ranking when different wording describes the same subject. Embeddings live in persistent `derived.sqlite`: they remain rebuildable from Markdown, but routine container updates and derived rebuilds preserve reusable vectors. On memory-constrained hosts, progressive mode processes one missing or stale concept at a time in a short-lived, low-priority, single-threaded worker, pauses for recent requests or high sampled CPU use, and releases model RAM after each item.

A fine-tuned 26M-parameter [Needle][needle] model can route a small set of natural-language read requests. It emits a candidate action that Memento validates before running. When configured, `memory_answer` assembles a versioned, authorization-scoped evidence set before asking a model for a cited answer. Secret intent abstains before cache lookup, retrieval, repository reads, graph access or model invocation. Other optional model slots can draft proposals and maintenance suggestions; deployments without a configured provider keep the answer tool disabled.

Model setup and measurements live in [`docs/semantic-search.md`](docs/semantic-search.md), [`docs/needle-fine-tuning.md`](docs/needle-fine-tuning.md) and [`docs/needle-performance.md`](docs/needle-performance.md).

## Assets And Skills

A concept can carry an immutable versioned asset pack as an ordinary Git blob. The Markdown remains searchable while diagrams, templates, datasets or a complete agent skill travel in an attached ZIP. Packs that fit the configured MCP request ceiling use `attach_asset_pack.zip_base64`, keeping proposal creation inside MCP. Larger packs can use `memory_asset_stage_begin`/`memory_asset_stage_status`; the begin call returns a one-time raw-upload ticket so the upload command does not need the principal's bearer token.

Skill concepts live under `/skills/`, carry the `skill` tag and match the ZIP-root `SKILL.md` byte-for-byte. Reviewers check the generated manifest and digest before approval; clients check them again against the ZIP returned by `memory_asset_get`. `memento-skill-import` validates a recalled pack before placing it in a workspace. Memento does not install or execute recalled skills on behalf of a client.

The repository also ships an Agent Skills package at [`.agents/skills/memento/SKILL.md`](.agents/skills/memento/SKILL.md). It gives Pi, Piclaw and Codex agents a compact workflow for search, reads, proposals, curation, namespaces, assets and retry reconciliation.

## Visual Debugging

The optional `/graph` surface helps humans inspect how memories are being created and managed. It shows explicit links, provenance, sizes, assets, proposals and index state in a 2.5D scene. Embedding similarity is a separately labelled, default-off layer: selected nodes reveal top semantic neighbours as amber segmented arcs with cosine/model metadata.

![Memento visual debugger showing linked memories and the selected-memory inspector](docs/memento-graph-debugger.png)

The debugger is disabled by default and unauthenticated when enabled. It is meant for a trusted development network, not an Internet-facing service. [ADR 0011](docs/decisions/0011-embed-a-gated-visual-memory-debugger.md) and [`docs/graph-explorer-plan.md`](docs/graph-explorer-plan.md) describe the boundary, implemented API and deferred revision-playback work.

## Running It

Memento supports Python 3.12-3.14 and ships as a non-root multi-architecture container. Start with [`examples/config.v1.json`](examples/config.v1.json), set `MEMENTO_ADMIN_MASTER_KEY`, then use [`docs/operations.md`](docs/operations.md) for deployment, health checks, backup and recovery. [`docs/access-management.md`](docs/access-management.md) covers bootstrap, `/admin`, MCP access tools, one-time credentials and explicit container master-key rotation.

For development:

```bash
make install-dev
make check
```

Tagged images are published at `ghcr.io/rcarmo/memento`. The DiskStation profile, including the scalar Intel J3455 path, is in [`docs/diskstation.md`](docs/diskstation.md).

Client setup guides cover [Pi](docs/setup-pi.md), [Piclaw](docs/setup-piclaw.md) and [Codex](docs/setup-codex.md).

## Documentation

* [`docs/setup-pi.md`](docs/setup-pi.md) -- connect Pi through pi-mcp-adapter
* [`docs/setup-piclaw.md`](docs/setup-piclaw.md) -- keychain-backed Piclaw MCP setup
* [`docs/setup-codex.md`](docs/setup-codex.md) -- Codex MCP and Agent Skills setup
* [`docs/access-management.md`](docs/access-management.md) -- dynamic principals, `/admin`, access tools and credential lifecycle
* [`docs/contracts.md`](docs/contracts.md) -- MCP tools, schemas, roles and limits
* [`docs/implementation.md`](docs/implementation.md) -- storage, transactions and runtime architecture
* [`docs/diagrams.md`](docs/diagrams.md) -- request, write, recovery and model flows
* [`docs/decisions/`](docs/decisions/README.md) -- architecture decisions
* [`docs/threat-model.md`](docs/threat-model.md) -- trust boundaries and abuse cases
* [`docs/operations.md`](docs/operations.md) -- deployment, health, backup and recovery
* [`docs/release.md`](docs/release.md) -- packaging and release process
* [`docs/load-testing.md`](docs/load-testing.md) -- load harness and thresholds
* [`docs/evidence/`](docs/evidence/README.md) -- benchmark and operational reports
* [`PLAN.md`](PLAN.md) -- delivery ledger and roadmap

## Credits

[`rcarmo/umcp`][umcp] supplies the MCP server, Streamable HTTP transport and request context. Memento's Rust semantic-search runtime was validated against [`rcarmo/go-gte`][go-gte], using the [`thenlper/gte-small`][gte-small] weights. The shallow router is fine-tuned from [`cactus-compute/needle`][needle].

Memento is MIT licensed. Third-party models, code and vendored browser libraries are listed in [`docs/attribution.md`](docs/attribution.md).

[piclaw]: https://github.com/rcarmo/piclaw
[go-gte]: https://github.com/rcarmo/go-gte
[gte-small]: https://huggingface.co/thenlper/gte-small
[needle]: https://github.com/cactus-compute/needle
[umcp]: https://github.com/rcarmo/umcp
