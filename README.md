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

The compact MCP surface exposes those frequent operations and keeps less common schemas in `memory://catalog` and `memory://workflow/{goal}`. `memory_inventory` returns bounded metadata, body digests and asset summaries without concept bodies. The execute-only `compare_manifest` operation compares a caller-provided local manifest with one authorised namespace page; Memento treats local paths as opaque labels and never reads caller files. `memory_execute` can also chain known operations using saved results, such as searching for a project and reading the first match.

Writes normally go through review:

```text
search -> read -> propose -> review -> apply -> Git commit -> index update
```

Memento checks the caller's namespace, the expected repository revision and the request's idempotency key. A retry returns the recorded result instead of creating another commit. Curators may review proposals they authored when they have write access to every affected path; `author_principal` and `reviewed_by` preserve that fact in the audit trail. Curators can also create, patch and rename concepts directly when those tools are exposed. Curators can also [trash, restore or purge concepts](docs/trash.md). Trash preserves original namespace permissions; purge removes current content and assets with explicit confirmation, retaining Git history.

Administrators manage principals through the preset-driven [`/admin`](docs/access-management.md) UI or role-filtered `access_*` tools on the same `/mcp` endpoint. Ordinary principals cannot discover or invoke those tools. New and rotated credentials are shown once; only verifiers are retained.

Start with the [documentation index](docs/README.md), [agent workflow diagrams](docs/agent-workflows.md) or [proposal review and refiling guide](docs/proposals.md). The [tool contracts](docs/contracts.md) define roles, arguments, limits and response envelopes.

## Search, Links And Local Models

FTS5 handles exact and lexical search. Markdown links supply backlinks and graph neighbourhoods. Neither needs a model.

GTE-small can add semantic ranking when different wording describes the same subject. Embeddings live in persistent `derived.sqlite`: they remain rebuildable from Markdown, but routine container updates and derived rebuilds preserve reusable vectors. On memory-constrained hosts, progressive mode processes one missing or stale concept at a time in a short-lived, low-priority, single-threaded worker, pauses for recent requests or high sampled CPU use, and releases model RAM after each item.

A fine-tuned 26M-parameter [Needle][needle] model can route a small set of natural-language read requests. It emits a candidate action that Memento validates before running. When configured, `memory_answer` assembles a versioned, authorisation-scoped evidence set before asking a model for a cited answer. Secret intent abstains before cache lookup, retrieval, repository reads, graph access or model invocation. Other optional model slots can draft proposals and maintenance suggestions; deployments without a configured provider keep the answer tool disabled.

Model setup and search behaviour are documented in [`docs/semantic-search.md`](docs/semantic-search.md); retained Needle training and performance results live under [`docs/evidence/needle/`](docs/evidence/needle/).

## Assets And Skills

A concept can include an immutable versioned asset pack as an ordinary Git blob. The Markdown remains searchable while diagrams, templates, datasets or a complete agent skill travel in an attached ZIP. Packs that fit the configured MCP request ceiling use `attach_asset_pack.zip_base64`, keeping proposal creation inside MCP. Larger packs can use `memory_asset_stage_begin`/`memory_asset_stage_status`; the begin call returns a one-time raw-upload ticket so the upload command does not need the principal's bearer token.

Skill concepts live under `/skills/`, have the `skill` tag and match the ZIP-root `SKILL.md` byte-for-byte. Reviewers check the generated manifest and digest before approval; clients verify accepted bytes through [`memory_asset_get` manifest, file and archive-range views](docs/accepted-assets.md). Large ZIPs are downloaded in chunks pinned to their version and digest. `memento-skill-import` validates a recalled pack before placing it in a workspace. Memento does not install or execute recalled skills on behalf of a client.

The repository also ships an Agent Skills package at [`.agents/skills/memento/SKILL.md`](.agents/skills/memento/SKILL.md). It gives Pi, Piclaw and Codex agents a compact workflow for search, reads, inventory, manifest comparison, proposals, curation, namespaces, assets and retry reconciliation.

## Visual Debugging

The optional `/graph` surface helps humans inspect how memories are being created and managed. It shows explicit links, provenance, sizes, assets, proposals and index state in a 2.5D scene. Embedding similarity is a separately labelled, default-off layer: selected nodes reveal top semantic neighbours as amber segmented arcs with cosine/model metadata.

![Memento visual debugger showing linked memories and the selected-memory inspector](docs/memento-graph-debugger.png)

The debugger is disabled by default and unauthenticated when enabled. It is meant for a trusted development network, not an Internet-facing service. [ADR 0011](docs/decisions/0011-embed-a-gated-visual-memory-debugger.md) and [`docs/graph-explorer-plan.md`](docs/graph-explorer-plan.md) describe the boundary, implemented API and deferred revision-playback work.

## Running It

Memento supports Python 3.12-3.14 and ships as a non-root multi-architecture container. Start with [`examples/config.v1.json`](examples/config.v1.json), set `MEMENTO_ADMIN_MASTER_KEY`, then use [`docs/operations.md`](docs/operations.md) for deployment, health checks, backup and recovery. [`docs/access-management.md`](docs/access-management.md) covers the dedicated admin/curator profile split, Piclaw and Pi MCP configuration, `/admin`, direct MCP access tools, one-time credentials and explicit container master-key rotation.

The `go` branch prepares the pure-Go `v1.0.0` replacement for `ghcr.io/rcarmo/memento`. It builds static amd64-v1 and arm64 binaries and a non-root distroless image:

```bash
make -C go audit
make -C go release-check VERSION=1.0.0
python3 tools/prepare_runtime_models.py
make go-container-contract MEMENTO_VERSION=1.0.0
```

`memento-go` is the native daemon and maintenance CLI, `memento-embed-go` is the framed GTE worker, and `memento-skill-import-go` installs recalled skill packs. The image preserves the existing `/etc/memento/config.json`, `/var/lib/memento`, `/models`, port 8000, `/mcp`, graph/admin routes, authentication and response contracts. Existing repository and SQLite formats are opened directly and remain readable by the previous `0.5.9` image for rollback. Python/Rust executables and libraries are not retained as runtime compatibility shims.

The DiskStation profile and its J3455 baseline constraints are in [`docs/diskstation.md`](docs/diskstation.md).

Client setup guides cover [Pi](docs/setup-pi.md), [Piclaw](docs/setup-piclaw.md) and [Codex](docs/setup-codex.md).

## Documentation

The [documentation index](docs/README.md) groups setup, agent tasks, contracts, operations and project records. Common starting points:

* [Agent workflows](docs/agent-workflows.md) -- discovery, comparison, execute plans, asset recall and interrupted writes.
* [Proposals](docs/proposals.md) -- submission, review decisions, rejection, refiling, stale proposals and skill updates.
* [Operations](docs/operations.md) -- deployment, health, backup and recovery.
* [System diagrams](docs/diagrams.md) -- transport, storage, publication and model flows.

## Credits

[`rcarmo/umcp`][umcp] supplies the MCP server, Streamable HTTP transport and request context. Memento's Rust semantic-search runtime was validated against [`rcarmo/go-gte`][go-gte], using the [`thenlper/gte-small`][gte-small] weights. The shallow router is fine-tuned from [`cactus-compute/needle`][needle].

Memento is MIT licensed. Third-party models, code and vendored browser libraries are listed in [`docs/attribution.md`](docs/attribution.md).

[piclaw]: https://github.com/rcarmo/piclaw
[go-gte]: https://github.com/rcarmo/go-gte
[gte-small]: https://huggingface.co/thenlper/gte-small
[needle]: https://github.com/cactus-compute/needle
[umcp]: https://github.com/rcarmo/umcp
