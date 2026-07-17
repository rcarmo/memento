# Shared Memory MCP Service — Detailed Implementation Plan

**Status:** Proposed implementation plan  
**Target:** Standalone service shared by multiple Piclaw instances  
**Protocol framework:** [`rcarmo/umcp`](https://github.com/rcarmo/umcp), using `AsyncMCPServer`  
**Deployment:** Docker/Portainer and native systemd  
**Knowledge format:** Git-backed, OKF-inspired Markdown concepts  
**Prepared:** 2026-07-16

---

## 1. Executive summary

Build a standalone Python daemon that exposes shared durable knowledge to several Piclaw instances through MCP. The service owns one authoritative Markdown repository and is the **only writer** to that repository. Piclaw instances remain independent MCP clients and keep their own conversations, Dream memory, schedules, and runtime state.

The initial service is deterministic:

- plain Markdown concepts with validated YAML-compatible frontmatter
- stable concept IDs and human-readable paths
- lexical/FTS5 search
- graph links, backlinks, orphan detection, and broken-link detection
- proposal-first mutations
- optimistic concurrency and durable idempotency
- serialized writes through a crash-recoverable operation journal
- Git commits for history and audit
- authenticated principals and role-based authorization

Later tiers add the Understory PR concepts that are deliberately excluded from the first cut:

1. **Exact query cache** keyed by repository revision.
2. **Hot working memory** over recent writes and recent answers.
3. **Deep read-only agent traversal** for natural-language answers.
4. **Model-assisted proposal generation**, never direct model-to-canonical writes.
5. **Scheduled Dream consolidation** driven by deterministic graph-health signals.
6. **Generic model-provider slots and model-level fallback** with explicit privacy and task policies.

The governing rule is:

> Git is authoritative for knowledge; SQLite is authoritative for operations; FTS, graph indexes, caches, and signals are derived; LLMs are advisory only.

---

## 2. Goals and non-goals

## 2.1 Goals

- Give Smith, Flint, and other Piclaw instances one shared durable knowledge base.
- Make knowledge human-readable, diffable, portable, and recoverable without the service.
- Preserve stable identity when concepts are renamed or moved.
- Support simultaneous reads and safe serialized writes.
- Prevent lost updates, duplicate retries, and partial multi-file mutations.
- Attribute every proposal and accepted mutation to an authenticated principal.
- Keep deterministic reads and writes usable when no LLM provider is available.
- Offer both Docker and systemd packaging from one source tree.
- Allow gradual rollout from read-only to curated multi-instance writes.
- Provide clean interfaces for later cache, hot-memory, deep-agent, Dream, and provider-fallback tiers.

## 2.2 Non-goals

The service will not replace:

- Piclaw's SQLite message history
- Piclaw sessions or compaction summaries
- `notes/daily/` or `notes/memory/`
- Piclaw Dream/AutoDream
- reminders or scheduled-task state
- credentials, keychains, or secret storage
- a general document-management system
- an MCP gateway or proxy
- a multi-tenant SaaS platform

The initial service will not include:

- embeddings or a vector database
- multiple active writer replicas
- automatic model-authored canonical writes
- autonomous deletion of concepts
- unrestricted external web retrieval
- direct filesystem access by clients
- a bespoke web UI

---

## 3. System context

```text
┌─────────────────┐
│ Piclaw: Smith   │──┐
└─────────────────┘  │
┌─────────────────┐  │   authenticated MCP
│ Piclaw: Flint   │──┼──────────────────────────────┐
└─────────────────┘  │                              │
┌─────────────────┐  │                              ▼
│ Other clients   │──┘                   ┌──────────────────────┐
└─────────────────┘                      │ shared-memory-mcp    │
                                         │ AsyncMCPServer       │
                                         ├──────────────────────┤
                                         │ auth + authorization │
                                         │ deterministic tools  │
                                         │ proposal/write queue │
                                         │ search + graph       │
                                         │ optional LLM tiers   │
                                         └──────────┬───────────┘
                                                    │
                        ┌───────────────────────────┼─────────────────────────┐
                        ▼                           ▼                         ▼
             ┌──────────────────┐       ┌────────────────────┐    ┌──────────────────┐
             │ Git repository   │       │ control.sqlite     │    │ derived.sqlite   │
             │ authoritative    │       │ operations/state   │    │ FTS/graph/cache  │
             └──────────────────┘       └────────────────────┘    └──────────────────┘
```

## 3.1 Trust boundaries

- Piclaw clients are authenticated independently.
- A client may be allowed to read, propose, curate, maintain, or administer.
- Tool arguments are untrusted even after authentication.
- Markdown content is untrusted data and may contain prompt injection.
- The reverse proxy provides TLS and may authenticate; the service must still receive a cryptographically or operationally trustworthy principal through uMCP request context.
- MCP session IDs are routing identifiers, not credentials.
- Only one service process may hold the write lease for a repository.

---

## 4. Prerequisite: uMCP modernization

The service depends on the separately specified uMCP work in `umcp-streamable-http-briefing.md`.

Required before broad production deployment:

- Streamable HTTP `/mcp`
- protocol-version negotiation
- request-scoped context via `contextvars`
- authenticated principal hook
- authorization hook
- Origin validation
- request body limits and content negotiation
- safe remote bind behavior

A pilot may use uMCP's existing legacy SSE transport behind an authenticated reverse proxy, but the production endpoint should use Streamable HTTP.

The shared-memory implementation must not carry a private fork of uMCP indefinitely. Framework changes should be upstreamed to `rcarmo/umcp` and consumed through a pinned release or commit.

---

## 5. Architectural planes

## 5.1 Knowledge plane — authoritative Git repository

Contains only durable, human-readable knowledge and generated knowledge indexes:

- concept Markdown files
- generated `index.md` files
- generated root `log.md`
- optional repository metadata such as `schema.json`

It must not contain:

- caches
- FTS databases
- traces
- lock files
- bearer tokens
- idempotency records
- scheduler state
- model prompts or raw model responses

## 5.2 Control plane — authoritative operational SQLite

`control.sqlite` owns:

- operation journal
- idempotency keys
- proposal lifecycle
- authorization policy cache, if not static
- scheduler watermarks
- maintenance-run state
- active write lease metadata
- bounded audit event index

Losing this database must not lose canonical knowledge, but may lose retry and scheduler history. It therefore needs backups alongside Git.

## 5.3 Derived plane — rebuildable SQLite and files

`derived.sqlite` owns:

- FTS5 search index
- concept metadata index
- links and backlinks
- graph degrees
- graph-health signals
- duplicate fingerprints
- exact query cache
- hot-memory metadata if persistence is enabled

It must be fully rebuildable from the Git repository and configuration.

## 5.4 Execution plane — MCP and background jobs

Contains:

- MCP read tools
- proposal and apply tools
- maintenance tools
- queue and locking
- optional internal LLM execution
- optional Dream scheduler

No execution-plane feature may bypass the canonical write pipeline.

---

## 6. Repository and filesystem layout

Recommended service state layout:

```text
/var/lib/shared-memory-mcp/
├── repo.git/                  # authoritative bare Git repository
├── current/                   # materialized checkout of refs/heads/main
├── worktrees/                 # temporary per-operation Git worktrees
├── control.sqlite             # durable operation/control state
├── control.sqlite-wal
├── derived.sqlite             # rebuildable FTS/graph/cache state
├── traces/                    # bounded model/query traces, outside Git
├── backups/                   # optional local snapshots
├── locks/
│   └── writer.lock
└── state.json                 # atomic service state/revision marker
```

Recommended configuration layout:

```text
/etc/shared-memory-mcp/
├── config.json
├── principals.json            # optional static authorization mapping
└── secrets.env                # mode 0600; or injected by container/keychain
```

The process must reject:

- symlinks inside the concept tree
- path traversal
- special device files
- concept files outside the configured knowledge root
- uncommitted changes in the materialized checkout

---

## 7. Knowledge model

## 7.1 Concept files

Every concept is one Markdown file with deterministic frontmatter.

Example:

```markdown
---
schema_version: 1
id: 01J2ABCDEF7PQRS8TUVWXYZ123
type: system
title: Smith
status: active
description: Primary Piclaw personal-assistant instance.
aliases:
  - smith-piclaw
tags:
  - piclaw
  - assistant
created_at: 2026-07-16T19:00:00Z
updated_at: 2026-07-16T19:20:00Z
updated_by: rui/tablet
---

# Smith

Smith is the primary Piclaw instance.

## Relationships

- Runs Piclaw: [Piclaw](/projects/piclaw.md)
- Hosted on: [Smith LXC](/systems/smith-lxc.md)
```

## 7.2 Required frontmatter

| Field | Rule |
|---|---|
| `schema_version` | Integer; initially `1` |
| `id` | Immutable ULID or UUIDv7 generated by the service |
| `type` | Controlled lower-case vocabulary, extensible by policy |
| `title` | Non-empty string |
| `status` | `active`, `deprecated`, or `tombstone` |
| `created_at` | Immutable UTC RFC3339 timestamp |
| `updated_at` | UTC RFC3339 timestamp |
| `updated_by` | Authenticated principal responsible for last accepted mutation |

Optional fields:

- `description`
- `aliases`
- `tags`
- `source_refs`
- `supersedes`
- type-specific namespaced metadata

Unknown top-level frontmatter keys should be rejected initially. Later schema extensions must be versioned.

## 7.3 Stable identity and paths

- `id` is immutable identity.
- Path is the human-readable locator.
- Renames preserve `id`.
- The derived index maps old paths and aliases to the current concept.
- Internal Markdown links use absolute bundle paths such as `/systems/smith.md`.
- Rename operations update inbound links in the same transaction.
- APIs accept either `id` or current path; responses return both.

## 7.4 Reserved files

- `index.md`: generated directory index, not a concept.
- root `log.md`: generated mutation summary, not a concept.
- `.memory-schema.json`: optional machine-readable repository schema.

Clients cannot write reserved files directly.

## 7.5 Canonical serialization

Implement one formatter with golden tests:

- UTF-8, LF endings
- deterministic frontmatter key ordering
- two-space YAML indentation
- sorted/deduplicated tags and aliases
- one trailing newline
- normalized timestamps
- no trailing whitespace
- body headings left human-authored except generated sections

The same semantic mutation at the same base revision must produce byte-identical files.

## 7.6 Deletion and tombstones

Initial policy:

- no general client-facing hard delete
- deprecate by setting `status: deprecated`
- replacement concept linked through `supersedes`
- admin-only tombstone retains `id`, title, redirect, and deletion provenance
- physical deletion is offline maintenance after a retention period

---

## 8. Git transaction model

## 8.1 Single active writer

- Exactly one daemon holds an OS-level exclusive writer lock.
- A second process may start read-only but must not mutate.
- Docker replica count is `1`.
- systemd uses one unit instance.
- Future failover requires an explicit fencing mechanism; shared storage alone is insufficient.

## 8.2 Per-operation temporary worktree

Use the bare repository as the authoritative Git object store.

For each accepted mutation:

1. Persist operation row as `queued` in `control.sqlite`.
2. Acquire the async write mutex.
3. Confirm the process still owns the writer lease.
4. Read current `refs/heads/main` as `base_rev`.
5. Compare caller's `expected_revision` with `base_rev`.
6. Create temporary worktree at `worktrees/<op_id>` from `base_rev`.
7. Apply concept changes only inside the temporary worktree.
8. Regenerate affected `index.md` chain and root `log.md`.
9. Validate schema, links, generated files, and requested invariants.
10. Stage exact changed paths—never `git add .`.
11. Commit with structured trailers.
12. Publish using compare-and-swap:

```text
git update-ref refs/heads/main <new_rev> <base_rev>
```

13. If compare-and-swap fails, mark operation `conflict`; do not force-update.
14. Update `current/` to the new revision under the repository read/write lock.
15. Incrementally update derived indexes to `new_rev`.
16. Mark operation `succeeded` and persist response.
17. Release lock and remove temporary worktree.
18. Emit resource/index-change notifications.

This avoids dirtying the canonical checkout and gives the Git ref update an atomic publication boundary.

## 8.3 Commit metadata

Commit message:

```text
memory: update Smith deployment details

Principal: smith
Client-Instance: smith-lxc
Operation-Id: op_01J...
Proposal-Id: prop_01J...
Base-Revision: abc123...
```

Only exact concept, index, and log paths are staged.

## 8.4 Read consistency

Every response includes:

```json
{
  "repo_revision": "<git sha>",
  "index_revision": "<git sha>",
  "index_stale": false
}
```

Rules:

- `memory_read` reads the materialized immutable snapshot associated with `repo_revision`.
- `memory_search` reports the revision indexed by FTS.
- Default search may be slightly stale during an index update.
- `freshness="strict"` waits for `index_revision == repo_revision`, subject to timeout.
- An apply response is not successful until `current/` and the incremental index have advanced, giving the submitting client read-your-writes behavior.

---

## 9. Control database

Use SQLite in WAL mode with foreign keys enabled.

## 9.1 `operations`

```text
op_id                  TEXT PRIMARY KEY
idempotency_key        TEXT NOT NULL
principal              TEXT NOT NULL
client_instance_id     TEXT
mcp_session_id         TEXT
source_chat            TEXT
tool_name              TEXT NOT NULL
request_hash           TEXT NOT NULL
base_revision          TEXT
result_revision        TEXT
state                   TEXT  # queued/running/succeeded/failed/conflict/recovering
request_json            TEXT
result_json             TEXT
error_class             TEXT
error_message           TEXT
created_at              TEXT
started_at              TEXT
finished_at             TEXT
```

Unique key:

```text
(principal, idempotency_key)
```

Semantics:

- same principal/key and same request hash: return stored result
- same principal/key and different request hash: hard idempotency conflict
- successful records retained for a configurable period, default 90 days

## 9.2 `proposals`

```text
proposal_id             TEXT PRIMARY KEY
author_principal        TEXT NOT NULL
client_instance_id      TEXT
base_revision           TEXT NOT NULL
intent                   TEXT NOT NULL
rationale                TEXT
patch_json               TEXT NOT NULL
patch_hash               TEXT NOT NULL
status                   TEXT  # draft/submitted/approved/rejected/applied/stale/expired
reviewed_by              TEXT
review_comment           TEXT
applied_operation_id     TEXT
applied_revision         TEXT
created_at               TEXT
updated_at               TEXT
expires_at               TEXT
```

## 9.3 `scheduler_runs`

```text
run_id                   TEXT PRIMARY KEY
job_name                 TEXT
window_key               TEXT
base_revision            TEXT
end_revision             TEXT
state                    TEXT
signal_count             INTEGER
proposal_count           INTEGER
model_chain_json         TEXT
started_at               TEXT
finished_at              TEXT
error_message            TEXT
```

Unique `(job_name, window_key)` prevents duplicate runs after restart.

## 9.4 `service_state`

Key/value records for:

- current materialized revision
- current index revision
- last successful full validation revision
- last Dream-scanned revision
- schema version
- writer lease identity

## 9.5 Recovery

At startup:

1. acquire or decline writer lease
2. verify Git repository and `main`
3. inspect operations left in `queued`, `running`, or `recovering`
4. compare any recorded result commit with `main`
5. classify each operation as safely retryable, published, conflicted, or failed
6. remove abandoned worktrees only after classification
7. verify or rebuild `current/`
8. verify/rebuild derived index if revision differs
9. become ready only after repository state is coherent

---

## 10. Search and graph indexing

## 10.1 Derived schema

### `concepts`

- `id`
- `path`
- `type`
- `title`
- `description`
- `status`
- `tags_json`
- `aliases_json`
- `content_hash`
- `updated_at`
- `repo_revision`

### `concept_fts`

FTS5 fields:

- title
- description
- aliases
- tags
- body
- path

Use weighted BM25 ranking:

1. title
2. aliases/path
3. description/tags
4. body

### `links`

- source concept ID
- target concept ID, nullable
- raw target
- target path
- anchor
- link kind
- resolution state
- first-seen revision
- last-checked revision

### `graph_metrics`

- inbound degree
- outbound degree
- broken-link count
- orphan flag
- connected-component ID

## 10.2 Search behavior

`memory_search` supports:

- `query`
- `type`
- `tags`
- `status`
- path prefix/namespace
- limit and cursor
- freshness mode

Return bounded snippets and stable IDs. Never return entire large concepts through search.

## 10.3 Rebuild and incremental updates

- Initial boot performs a full scan.
- Each successful Git operation supplies the exact changed paths for incremental update.
- A nightly or admin-triggered parity check compares incremental state with a clean scan.
- Index corruption causes derived DB quarantine and rebuild, not repository mutation.
- Service may remain available for direct reads while search reports degraded readiness.

## 10.4 Scale thresholds

Initial limits:

- 10,000 concepts
- 256 KiB maximum concept body
- 100 search results maximum
- 20 default search results
- 2 MiB maximum proposal diff
- 100 changed concepts per operation

Make limits configurable and expose them through `memory_status`.

---

## 11. Authentication and authorization

## 11.1 Principals

The updated uMCP request context supplies a trusted principal, for example:

```json
{
  "name": "smith",
  "roles": ["reader", "proposer"],
  "metadata": {
    "instance": "smith-lxc"
  }
}
```

Do not accept `principal` as an MCP tool argument.

## 11.2 Roles

| Role | Permissions |
|---|---|
| `reader` | status, list, search, read, graph, audit |
| `proposer` | reader permissions plus create/update proposals |
| `curator` | review and apply proposals; direct dry-run patches |
| `maintainer` | graph maintenance and Dream execution |
| `admin` | policy, tombstones, import, repair, and service operations |

## 11.3 Namespace policy

Authorization may constrain path prefixes:

```json
{
  "smith": {
    "roles": ["reader", "proposer"],
    "read_prefixes": ["/shared/", "/instances/smith/"],
    "write_prefixes": ["/instances/smith/"]
  }
}
```

Search results must be filtered before ranking/output to prevent metadata leakage.

## 11.4 Authentication options

Supported deployment patterns:

1. distinct bearer token per instance
2. trusted reverse-proxy identity header after proxy authentication
3. mTLS principal mapped by the proxy/service

Secrets are injected through environment/key files and never stored in the repository.

---

## 12. MCP surface — initial deterministic cut

All tools return a standard envelope:

```json
{
  "status": "success",
  "data": {},
  "warnings": [],
  "next_tools": [],
  "repo_revision": "...",
  "index_revision": "...",
  "index_stale": false,
  "operation_id": null
}
```

Errors use stable machine-readable classes:

- `validation_error`
- `not_found`
- `forbidden`
- `conflict`
- `idempotency_conflict`
- `repo_unavailable`
- `repo_dirty`
- `index_unavailable`
- `queue_full`
- `temporarily_read_only`

## 12.1 Discovery and status

### `memory_help`

Inputs:

- `goal`: optional free text or known goal key
- `format`: `summary` or `detailed`

Returns recommended tool chains, supported goals, warning fields, and exact next tools.

### `memory_status`

Returns:

- service version
- schema version
- repo/index revisions
- readiness/degraded reasons
- concept/type counts
- graph health
- proposal backlog
- write queue depth
- configured limits
- enabled optional tiers

## 12.2 Read tools

### `memory_search`

Deterministic FTS search with filters and bounded snippets.

### `memory_read`

Inputs:

- `id_or_path`
- optional `revision`
- output mode: `full`, `metadata`, or `section`

Returns canonical concept data, content hash, and revision.

### `memory_list`

Browse directories, types, tags, or concepts with pagination.

### `memory_graph`

Bounded graph neighbourhood around a concept:

- inbound/outbound
- depth, maximum 2 initially
- edge and node limits
- broken links and orphan state

### `memory_audit`

Audit one concept or the full repository for:

- schema validity
- generated index validity
- broken links
- duplicate IDs
- path/ID mismatch
- reserved-file misuse

## 12.3 Proposal tools

### `memory_propose`

Accepts a structured proposal:

- intent
- base revision
- proposed concept creates/patches/renames
- rationale
- idempotency key
- source reference metadata

The service validates and stores it but does not mutate Git.

### `memory_proposal_get`

Read proposal, validation, diff preview, and staleness.

### `memory_proposal_list`

Filter by status, author, target concept, or age.

### `memory_proposal_review`

Curator/admin only:

- approve
- reject
- request changes
- attach review comment

Approval does not automatically apply unless policy explicitly enables it.

### `memory_proposal_apply`

Curator/admin only. Requires:

- approved proposal
- expected repository revision
- idempotency key

Returns conflict if the proposal's base is stale unless an explicit, deterministic rebase succeeds.

## 12.4 Direct mutation tools

Initially curator/admin only.

### `memory_patch`

Modes:

- `dry_run`
- `commit`

Supports:

- frontmatter patch
- replace named top-level section
- append named section
- replace body, admin/curator policy only

Requires expected revision and idempotency key for commit.

### `memory_create`

Creates one concept with service-generated ID and path validation.

### `memory_rename`

Renames a concept and updates inbound links atomically.

No general `memory_delete` in the initial cut.

## 12.5 MCP resources

Expose read-only resources where client support is useful:

```text
memory://status
memory://concept/{id}
memory://path/{encoded_path}
memory://graph/{id}
```

On successful mutation:

- emit `notifications/resources/updated` for changed concepts
- emit `notifications/resources/list_changed` for creates, renames, or tombstones

Tools remain the primary Piclaw interface because `pi-mcp-adapter` is optimized around tool proxying.

---

## 13. Proposal-first workflow

Default cross-instance flow:

```text
client observes durable fact
  → memory_search
  → memory_read related concepts
  → memory_propose
  → deterministic validation/diff
  → curator reviews
  → memory_proposal_apply
  → Git transaction
  → index update
  → audit
```

Proposal policies:

- default expiry: 30 days
- proposal becomes `stale` when target revisions change
- stale proposal may be deterministically rebased only if patches do not overlap
- conflicting proposals remain reviewable but cannot apply
- proposal author cannot approve unless policy permits self-approval
- every apply links proposal, operation, principal, and Git revision

For the first pilot, Smith may act as curator while other instances are proposer-only.

---

## 14. Initial implementation milestones

## Milestone 0 — repository and contracts

Deliverables:

- new repository, suggested `rcarmo/shared-memory-mcp`
- AGENTS.md and contribution rules
- package metadata and MIT license
- concept schema v1
- MCP tool contract document
- threat model
- sample bundle
- Docker and systemd skeletons

Exit criteria:

- schemas and examples reviewed before implementation
- authority boundaries agreed

## Milestone 1 — deterministic repository core

Implement:

- safe path handling
- frontmatter parser/serializer
- concept validation
- stable IDs
- reserved files
- index generation
- root log generation
- graph extraction
- full repository validation

Tests:

- golden serialization
- traversal/symlink rejection
- malformed frontmatter
- duplicate IDs
- rename/link rewrite
- deterministic index/log output

## Milestone 2 — Git and control plane

Implement:

- bare repository bootstrap
- materialized checkout
- per-operation worktrees
- compare-and-swap ref publication
- SQLite schema/migrations
- operation journal
- idempotency
- crash recovery
- writer lease

Tests include injected crashes at every transaction step.

## Milestone 3 — derived search and graph

Implement:

- FTS5 schema
- full rebuild
- incremental changed-path update
- graph metrics
- index revision markers
- parity checker

## Milestone 4 — read-only MCP service

Implement:

- `AsyncMCPServer` integration
- auth principal consumption
- help/status/search/read/list/graph/audit
- standard envelopes and pagination
- health/readiness endpoints

Deploy one read-only canary and connect one Piclaw instance.

## Milestone 5 — proposals and curated writes

Implement proposal lifecycle, dry runs, review, apply, optimistic concurrency, resource notifications, and audit trailers.

Enable:

- Smith: reader/proposer/curator
- Flint: reader/proposer

No model features yet.

## Milestone 6 — production operations

Implement:

- Docker multi-arch image
- Compose/Portainer example
- hardened systemd unit
- reverse-proxy examples
- backup and restore tooling
- metrics/logging
- retention jobs
- upgrade/migration procedure

Only after this milestone should additional Piclaw instances join.

---

# 15. Deferred intelligent tiers

The following tiers are part of the complete roadmap but are feature-gated and excluded from the deterministic initial cut.

## Tier A — exact natural-language query cache

Inspired by Understory PR #8, layer one.

### Purpose

Avoid repeating identical expensive natural-language answer operations when neither repository content nor authorization scope has changed.

### API boundary

Introduce a separate tool:

```text
memory_answer(question, scope?, freshness?, answer_mode?)
```

Do not overload `memory_search`; deterministic search remains deterministic and transparent.

### Cache key

Use:

```text
sha256(
  repo_revision
  + normalized_question
  + authorized_namespace_fingerprint
  + answer_mode
  + model_policy_revision
  + prompt_version
  + tool_version
)
```

Using Git revision is more reliable and cheaper than Understory's path/mtime/size scan.

### Storage and limits

- derived SQLite table
- LRU, default 200 entries
- default TTL 24 hours
- bounded answer and citation payload
- entries scoped by authorization visibility
- optional principal scope for private namespaces

### Semantics

- cache hit returns `answer_source: exact_cache`
- preserve original citations and model chain
- cache hit creates a lightweight access audit record but no new model trace
- any relevant repository revision change naturally produces a different key
- policy/prompt/model changes invalidate through versioned key fields

### Gate

Ship only after deep query has stable citation and trace semantics.

---

## Tier B — hot working memory

Inspired by Understory PR #8, layer two.

### Purpose

Answer recently related questions using a small, cheap context before invoking the full deep traversal agent.

### Working set

Per authorization scope:

- last 10 successfully changed concepts, stored as IDs and read fresh
- last 10 accepted deep-query Q&A records
- configurable TTL, default 1 hour

### Staleness rules

- concept bodies are always read fresh at the current revision
- any canonical write clears recent Q&A entries whose scope intersects the changed concepts
- policy or authorization changes clear affected working sets
- manual imports advance revision and clear relevant sets

### Execution

One tool-free model call receives bounded excerpts and question.

System rule:

- answer only if fully supported by supplied excerpts
- otherwise return an exact sentinel such as `UNKNOWN`
- cite concept IDs/paths used

### Return metadata

```json
{
  "answer_source": "hot_memory",
  "model_chain": ["..."],
  "citations": [{"id": "...", "path": "...", "revision": "..."}]
}
```

### Safety

- no mutation tools
- strict excerpt size cap
- content wrapped as untrusted data
- no cross-principal or cross-namespace leakage
- hot answers are advisory, not repository facts

---

## Tier C — deep read-only agent traversal

Inspired by Understory's current `memory_query` and PR #8 layer three.

### Purpose

Answer complex questions that require iterative search, graph traversal, and reading multiple concepts.

### Internal agent tools

Expose only read operations to the internal agent:

- `search_knowledge`
- `read_concept`
- `list_directory`
- `graph_neighbors`
- `inspect_graph_health`

No write, patch, delete, web, shell, or external-fetch tools.

### Limits

- default 8 steps; hard maximum 12
- wall-clock timeout
- maximum concepts read
- maximum cumulative characters/tokens
- maximum answer tokens
- per-principal and global concurrency limits
- cancellation propagated from MCP

### Output contract

Require:

- concise answer
- citations to exact concept ID/path/revision
- confidence classification
- unresolved ambiguity list
- traversal trace ID
- model chain
- answer source `deep_agent`

Reject or mark incomplete an answer that cannot produce valid citations.

### Trace storage

Store outside Git:

- trace ID
- principal
- question hash and bounded/redacted question
- repository revision
- search/read steps
- paths accessed
- answer summary
- model chain
- duration and token usage

Apply bounded retention, default 50 traces or 30 days, configurable.

### Retrieval pipeline

```text
memory_answer
  1. exact cache
  2. hot working memory
  3. deep read-only agent
```

Each layer reports which source answered.

---

## Tier D — model-assisted proposal generation

Equivalent to Understory's `memory_add`/`memory_update`, but constrained to proposal generation.

### Tools

```text
memory_propose_freeform(content, suggested_path?, intent?)
memory_propose_update(instruction, target_hint?)
```

### Agent capabilities

The internal agent may:

- search
- read
- inspect graph
- construct a structured patch proposal

It may not write Git or approve/apply proposals.

### Required behavior

- search before proposing
- prefer enriching an owning concept over creating fragments
- identify contradictions explicitly
- propose reciprocal links where justified
- cite every concept consulted
- return a deterministic proposal object for validation

The normal proposal review and apply pipeline remains authoritative.

---

## Tier E — scheduled Dream consolidation

Inspired by Understory PR #9, but integrated with durable scheduler state and proposal-first safety.

## E.1 Deterministic signal scanner

Signals:

1. **Orphans** — active concepts with no inbound links.
2. **Broken links** — unresolved link targets.
3. **Likely duplicates** — title/description/tag similarity.
4. **Oversized concepts** — default ≥6,000 body characters or ≥6 top-level sections.
5. **Recent activity** — concepts changed since the previous successful Dream watermark.

Improvements over the Understory PR design:

- recent activity is bounded by `last_dream_revision`; a populated log alone does not trigger every run
- signal records have stable dedupe keys
- closed signals do not recur unless repository evidence changes
- maximum three oversized-concept candidates per run
- no model call when there are no actionable signals

## E.2 Signal table

Derived records:

```text
signal_id
signal_type
entity_refs_json
severity
repo_revision
dedupe_key
status              # open/acknowledged/proposed/resolved/ignored
first_detected_at
last_detected_at
resolved_revision
```

## E.3 Rollout modes

1. `disabled`
2. `report_only` — scan and report, no model
3. `propose` — model may create proposals
4. `auto_apply_safe` — future, limited deterministic fixes only

Never auto-apply model-authored merges, deletions, or concept splits in the initial roadmap.

## E.4 Dream model actions

The Dream agent may propose:

- linking genuine orphans
- repairing or removing broken links
- merging duplicate concepts
- splitting oversized concepts into hub-and-spoke structure while retaining original ID/path as hub
- creating one overview concept for a genuinely coherent recent theme

Every result is a proposal and enters normal review.

## E.5 Scheduling

Support two invocation styles:

### Built-in interval — Docker-friendly

```text
DREAM_INTERVAL=6h
```

- opt-in
- first run one interval after startup
- minimum 5 minutes
- never overlaps
- durable window key prevents rerun after restart

### External trigger — systemd-friendly

Provide CLI/admin call:

```text
shared-memory-mcp dream --mode report_only
```

A systemd timer can invoke this while the service owns the actual run through an authenticated local control path.

Do not permit two schedulers to run the same window.

## E.6 Budgets

- maximum signals per run
- maximum model proposals per run
- token and wall-clock budget
- daily spend ceiling
- quiet period after repository changes
- backoff after failures
- explicit model unavailability behavior

---

## Tier F — generic model provider slots and model-level fallback

Inspired by Understory PR #5.

## F.1 Provider abstraction

Support generic endpoint configurations rather than hardcoded provider names:

```text
base_url
api_format       # openai | anthropic
api_key_ref
model
headers
```

Local llama.cpp/llama-swap remains a first-class OpenAI-compatible configuration with optional model discovery.

## F.2 Task-specific model slots

Do not use one global model for every operation. Define slots:

| Slot | Purpose | Default fallback policy |
|---|---|---|
| `hot_query` | cheap bounded working-set answer | local-only preferred |
| `deep_query` | read-only iterative retrieval | fallback allowed by policy |
| `proposal` | model-assisted proposal drafting | fallback disabled initially |
| `dream` | background consolidation proposals | fallback disabled initially |

Each slot defines:

- primary model
- ordered fallback models
- task allowlist
- timeout
- token budget
- retry budget
- data-classification policy
- concurrency limit

## F.3 Model-level fallback

Fallback wraps one model generation step, not the whole agent run.

This prevents replaying a multi-step agent that may already have produced effects. Even though initial agents are read-only or proposal-only, model-level fallback remains the correct primitive.

Retry only recognized transient failures:

- connection failures
- timeouts, unless caller cancellation
- HTTP 5xx
- selected provider overload errors

Do not fallback on:

- 400 validation failures
- 401/403 authentication failures
- caller cancellation
- policy denial
- malformed model output
- 429 unless explicitly enabled

Mid-stream failures are not replayed automatically.

## F.4 Privacy policy

A fallback may move knowledge from a local model to a cloud provider. Therefore:

- every slot declares allowed data classifications
- fallback across trust boundaries is opt-in
- query-only fallback is the recommended first policy
- proposal and Dream fallback remain disabled until reviewed
- response and trace disclose every attempted/used model

Example:

```json
{
  "model_chain": [
    {"model": "local/llama", "outcome": "timeout"},
    {"model": "cloud/model", "outcome": "success"}
  ]
}
```

## F.5 Configuration and secrets

Use environment variables or secret files, never Git:

```text
MODEL_HOT_QUERY_CONFIG
MODEL_DEEP_QUERY_CONFIG
MODEL_PROPOSAL_CONFIG
MODEL_DREAM_CONFIG
```

Prefer structured JSON config referencing secret environment names rather than embedding API keys.

---

## 16. Optional tier interaction matrix

| Capability | Deterministic | Uses model | Can mutate canonical repo |
|---|---:|---:|---:|
| search/read/list/graph | Yes | No | No |
| structured proposal storage | Yes | No | No |
| reviewed proposal apply | Yes | No | Yes |
| exact answer cache | Derived | No on hit | No |
| hot working memory | No | Yes | No |
| deep query agent | No | Yes | No |
| freeform proposal agent | No | Yes | No; proposal only |
| Dream signal scan | Yes | No | No |
| Dream proposal generation | No | Yes | No; proposal only |
| deterministic safe repair | Yes | No | Future, policy-gated |

Optional tiers must never become dependencies of deterministic reads/writes.

---

## 17. Prompt-injection and content safety

Markdown concepts are untrusted data.

For every internal model call:

- place concept content inside explicit non-instruction data delimiters
- state that embedded instructions must not be followed
- expose only the minimum internal tool set
- validate all tool arguments server-side
- cap steps and context
- forbid external URLs unless a future explicit fetch tool is authorized
- require citations for answers and proposals
- reject paths not returned by deterministic search/list tools

Before proposal application:

- scan for likely credentials, private keys, bearer tokens, and high-entropy secrets
- block or require admin override
- validate links and namespaces
- enforce size and diff limits
- render exact diff for reviewer

---

## 18. Observability

## 18.1 Structured logs

Every request log includes:

- timestamp
- request ID
- principal
- client instance
- tool name
- operation/proposal ID
- repository/index revision
- status/error class
- latency
- model slot/chain where applicable

Never log tokens, complete concept bodies, or full sensitive prompts by default.

## 18.2 Metrics

Expose Prometheus text or structured health metrics:

- request count and latency by tool/status
- active clients
- write queue depth and oldest age
- operation success/conflict/failure counts
- repository versus index revision lag
- proposal backlog and age
- concept/link/orphan/broken-link counts
- FTS rebuild duration
- cache hit rates: exact/hot/deep
- internal model calls, latency, tokens, and fallback rate
- Dream runs/signals/proposals/failures
- repository/control/derived DB sizes

## 18.3 Health

### Liveness

Process/event loop is running.

### Readiness

Read-ready when:

- Git repository can be read
- materialized revision is valid
- authorization config is loaded

Write-ready additionally requires:

- writer lease held
- control DB writable
- no dirty/diverged repository state
- transaction recovery complete
- queue below hard limit

Search readiness reports whether the index is current or degraded.

---

## 19. Backup and disaster recovery

Back up:

- `repo.git`
- `control.sqlite` through SQLite backup API or consistent snapshot
- configuration excluding injected secrets where managed elsewhere

Do not need to back up:

- `current/`
- temporary worktrees
- `derived.sqlite`
- caches

Restore procedure:

1. stop or fence writer
2. restore `repo.git` and `control.sqlite`
3. remove abandoned worktrees/current checkout
4. start in recovery/read-only mode
5. reconcile operations against Git head
6. materialize `current/`
7. rebuild derived indexes
8. run full audit
9. enable writes

Required drills:

- restore to a clean host
- rebuild derived index from Git only
- recover commit published before operation row completion
- recover operation journaled but not committed
- rotate credentials without repository changes

Git remote push is audit/backup replication, not a substitute for control DB backup.

---

## 20. Deployment

## 20.1 Docker/Portainer

Container principles:

- non-root user
- read-only root filesystem
- writable `/data` only
- secrets injected independently
- one replica
- health checks
- graceful shutdown drains writes
- pinned uMCP/service versions
- multi-arch image if needed

Persistent mounts:

```text
/data   -> /var/lib/shared-memory-mcp equivalent
/config -> read-only configuration
```

Reverse proxy must:

- terminate TLS
- authenticate or preserve authorization headers
- disable buffering for any SSE streams
- set long stream timeouts
- limit body size
- preserve `/mcp`

## 20.2 systemd

Unit hardening:

- dedicated `shared-memory` user/group
- `StateDirectory=shared-memory-mcp`
- `ConfigurationDirectory=shared-memory-mcp`
- `ProtectSystem=strict`
- `ProtectHome=true`
- `PrivateTmp=true`
- `NoNewPrivileges=true`
- explicit `ReadWritePaths`
- restart on failure with bounded backoff
- readiness notification or health check
- graceful timeout long enough to finish/abort active transaction

A companion timer may trigger backup, audit, or Dream commands, but durable window dedupe remains in the service.

## 20.3 Primary deployment mode

Choose one mode for the first production pilot and test it fully. Add the second only after parity tests. Docker is preferred for Portainer infrastructure; systemd is preferred for a dedicated VM/LXC with host-native Git/restic integration.

---

## 21. Configuration

Use a versioned JSON configuration file plus environment overrides. Python 3.10 compatibility avoids relying on `tomllib`.

Top-level sections:

```json
{
  "schema_version": 1,
  "server": {},
  "repository": {},
  "authorization": {},
  "limits": {},
  "search": {},
  "proposals": {},
  "models": {},
  "dream": {},
  "observability": {},
  "retention": {}
}
```

Validate configuration at startup and fail before accepting requests. Redact secret values from diagnostics.

Feature flags:

```text
FEATURE_EXACT_CACHE
FEATURE_HOT_MEMORY
FEATURE_DEEP_QUERY
FEATURE_MODEL_PROPOSALS
FEATURE_DREAM
FEATURE_MODEL_FALLBACK
```

Each can be disabled independently without disabling deterministic service.

---

## 22. Source repository layout

```text
shared-memory-mcp/
├── pyproject.toml
├── README.md
├── LICENSE
├── AGENTS.md
├── Dockerfile
├── docker-compose.yml
├── systemd/
│   ├── shared-memory-mcp.service
│   └── shared-memory-mcp-dream.timer
├── src/shared_memory_mcp/
│   ├── server.py
│   ├── cli.py
│   ├── config.py
│   ├── context.py
│   ├── authz.py
│   ├── envelopes.py
│   ├── repository/
│   │   ├── bundle.py
│   │   ├── schema.py
│   │   ├── frontmatter.py
│   │   ├── serializer.py
│   │   ├── links.py
│   │   ├── indexer.py
│   │   ├── validator.py
│   │   ├── git_store.py
│   │   └── transactions.py
│   ├── control/
│   │   ├── db.py
│   │   ├── migrations.py
│   │   ├── operations.py
│   │   ├── proposals.py
│   │   └── scheduler.py
│   ├── derived/
│   │   ├── index.py
│   │   ├── search.py
│   │   ├── graph.py
│   │   ├── signals.py
│   │   └── cache.py
│   ├── tools/
│   │   ├── discovery.py
│   │   ├── reads.py
│   │   ├── proposals.py
│   │   ├── mutations.py
│   │   └── maintenance.py
│   ├── models/
│   │   ├── config.py
│   │   ├── providers.py
│   │   ├── fallback.py
│   │   ├── hot_memory.py
│   │   ├── deep_query.py
│   │   ├── proposal_agent.py
│   │   └── traces.py
│   └── dream/
│       ├── scanner.py
│       ├── duplicates.py
│       ├── oversized.py
│       ├── runner.py
│       └── service.py
├── tests/
│   ├── unit/
│   ├── integration/
│   ├── e2e/
│   ├── crash/
│   └── fixtures/
└── sample-bundle/
```

Keep optional tier modules present as interfaces/stubs only until their milestone begins; avoid premature provider dependencies in the initial package.

---

## 23. Testing strategy

## 23.1 Unit tests

- frontmatter parsing and canonical serialization
- path and symlink containment
- stable ID rules
- link parsing/resolution
- index/log generation
- schema validation
- authorization filtering
- normalized request hashes
- proposal state machine
- cache key construction
- Dream signal detectors
- retry/fallback classification

## 23.2 Determinism tests

- same request + same revision produces byte-identical output
- generated index and log ordering is stable
- full rebuild equals incremental derived state
- authorization filtering cannot affect ranking through hidden results

## 23.3 Concurrency tests

- simultaneous reads
- concurrent writes to same base revision: one succeeds, one conflicts
- duplicate idempotent requests
- same idempotency key with different payload
- queue fairness across principals
- cancellation before and during write
- async request-context isolation

## 23.4 Crash-injection tests

Fail after:

1. operation journal insert
2. worktree creation
3. concept write
4. index regeneration
5. Git commit
6. `update-ref`
7. materialized checkout update
8. derived index update
9. operation completion row

Restart and verify no lost or double-applied mutation.

## 23.5 Security tests

- traversal and symlink attacks
- forbidden namespace search leakage
- principal spoofing in tool arguments
- oversized request/diff
- Markdown prompt injection corpus
- secret scanner
- disallowed Origin/auth failures
- model fallback across disallowed privacy boundary

## 23.6 Optional tier tests

### Exact cache

- revision and policy invalidation
- no cross-principal leakage
- TTL/LRU
- cached citations unchanged

### Hot memory

- concepts read fresh
- Q&A invalidated by intersecting write
- `UNKNOWN` falls through
- scope isolation

### Deep query

- step/token/time caps
- cancellation
- citation enforcement
- no write tools
- trace retention

### Dream

- no signal means no model call
- recent activity watermark advances
- signals dedupe across restarts
- report-only makes no proposals
- proposal mode never writes Git
- no overlapping runs

### Provider fallback

- transient failure falls back one generation step
- auth/cancel/policy errors do not fallback
- mid-stream failure is not replayed
- model chain disclosed
- mutation/proposal slots obey fallback policy

## 23.7 Compatibility tests

- current and updated uMCP
- Piclaw `pi-mcp-adapter` Streamable HTTP connection
- bearer header reaches principal hook
- MCP timeout/cancellation propagation
- resource notifications where enabled
- Docker and systemd smoke tests
- Python 3.10, 3.11, and 3.12

---

## 24. CI/CD

Required gates:

- formatting/lint
- type checking
- unit/integration tests
- crash-recovery suite
- security tests
- container build
- non-root container smoke
- sample-bundle audit
- schema migration tests
- generated artifact cleanliness

Release artifacts:

- source archive
- Python package or self-contained source image
- multi-arch container
- SBOM
- checksums
- systemd unit and environment example
- migration notes

Pin production deployments to immutable image tags or commit SHAs, never `latest` alone.

---

## 25. Rollout plan

## Stage 1 — local read-only prototype

- seed a small private bundle
- connect Smith only
- exercise search/read/status
- compare with existing Piclaw workspace search
- measure latency and result quality

## Stage 2 — proposal-only pilot

- Smith and Flint can propose
- one human/Smith curator reviews
- no direct writes for other clients
- review proposal duplication and schema fit

## Stage 3 — curated writes

- enable curator apply
- run backup/restore and crash drills
- enforce repository and control DB monitoring

## Stage 4 — multi-instance production

- add remaining Piclaw clients
- per-principal tokens, quotas, and namespace policy
- establish SLOs and alerts

## Stage 5 — exact cache

- enable only after `memory_answer` exists
- observe hit rate and invalidation correctness

## Stage 6 — hot memory and deep query

- hot memory first with local model
- deep query behind explicit tool/feature flag
- enforce budgets and citations

## Stage 7 — model-assisted proposals

- proposals only
- measure reviewer acceptance and correction rate
- no direct apply

## Stage 8 — Dream report-only

- collect signal quality for several weeks
- tune duplicate/oversized thresholds
- verify no repeated recent-activity churn

## Stage 9 — Dream proposal mode and provider fallback

- Dream may generate proposals
- enable query-only fallback first
- retain local-only policy for proposal/Dream until privacy review

Every stage has an independent rollback flag.

---

## 26. Service-level objectives and operational limits

Initial targets:

- deterministic direct read p95 < 100 ms on local network
- FTS search p95 < 250 ms for 10,000 concepts
- status p95 < 100 ms
- successful write apply p95 < 2 seconds excluding Git remote push
- repository/index revision lag < 5 seconds
- zero acknowledged lost updates
- zero duplicate mutations for repeated idempotency keys
- restart recovery < 60 seconds for normal repository size

Queue limits:

- maximum global pending writes: 100
- maximum per principal: 20
- maximum concurrent deep queries: configurable, default 2
- maximum concurrent model call per principal: 1

---

## 27. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Two active writers | OS lease, one replica, future fencing before HA |
| External Git edits | prohibit canonical direct writes; detect divergence and enter read-only mode |
| Lost update | expected revision plus atomic `update-ref` CAS |
| Retried mutation duplicates | durable idempotency ledger |
| Partial multi-file mutation | temporary worktree, validation, atomic ref publication |
| FTS corruption | derived DB rebuild |
| Control DB loss | independent backup; Git still preserves knowledge |
| Secret ingestion | proposal scanner and review gate |
| Prompt injection | content-as-data prompts and read-only internal tools |
| Cross-instance data leakage | principal-scoped authorization before search/output/cache |
| Model fallback leaks private data | per-slot trust policy; cross-boundary fallback opt-in |
| Dream repeatedly spends tokens | revision watermark, dedupe keys, budgets, report-only rollout |
| Model writes corrupt truth | models only create proposals |
| Repository grows without bound | concept limits, maintenance reports, Git maintenance policy |
| uMCP transport churn | pin tested version and maintain compatibility smoke tests |

---

## 28. Completion criteria

The initial deterministic service is complete when:

- [ ] One Docker or systemd deployment serves at least two Piclaw clients.
- [ ] Clients authenticate as distinct principals.
- [ ] Read/search results honor namespace policy.
- [ ] The Git repository is the sole canonical knowledge source.
- [ ] Operations and idempotency survive restarts.
- [ ] Concurrent stale writes conflict safely.
- [ ] Proposal review/apply is fully audited.
- [ ] Derived index can be deleted and rebuilt without data loss.
- [ ] Crash-injection suite passes at all transaction boundaries.
- [ ] Backup/restore drill succeeds on a clean host.
- [ ] Piclaw connects through Streamable HTTP without legacy fallback after uMCP modernization.
- [ ] Docker and systemd deployment documentation is tested.

The complete roadmap is delivered when, additionally:

- [ ] Exact cache reports correct revision/policy-scoped hits.
- [ ] Hot memory falls through safely on insufficient evidence.
- [ ] Deep query returns validated citations and bounded traces.
- [ ] Model-assisted operations create proposals only.
- [ ] Dream uses durable signal watermarks and never calls a model without actionable work.
- [ ] Dream starts in report-only mode and can be promoted independently.
- [ ] Generic provider slots support local and compatible remote endpoints.
- [ ] Model-level fallback is limited to approved transient errors and trust boundaries.
- [ ] All optional tiers can be disabled while deterministic service remains healthy.

---

## 29. Recommended first implementation ticket sequence

1. Define concept schema v1 and canonical Markdown fixtures.
2. Implement bundle containment, parser, serializer, and validator.
3. Implement deterministic index/log generation and graph extraction.
4. Add bare Git repository and temporary-worktree transaction prototype.
5. Add control SQLite migrations, operations, and idempotency.
6. Add startup reconciliation and crash injection.
7. Add derived FTS/graph database and full/incremental parity tests.
8. Add uMCP read-only server and standard response envelope.
9. Add principal/role/namespace authorization.
10. Add proposal lifecycle and deterministic diff preview.
11. Add reviewed apply path and Git audit trailers.
12. Add Docker deployment, then systemd parity.
13. Connect Smith as read-only canary.
14. Connect Flint as second principal and test isolation/conflicts.
15. Run restore drill and enable curated writes.
16. Implement `memory_answer` deep read-only agent behind a flag.
17. Add exact cache, then hot working set.
18. Add model-assisted proposal generation.
19. Add Dream scanner in report-only mode.
20. Add generic model slots and query-only model-level fallback.
21. Promote Dream to proposal mode only after signal-quality review.

---

## 30. Final architecture decision

The service should remain useful if every optional intelligent tier is turned off:

```text
Git Markdown repository
  + deterministic validation/indexing
  + FTS and graph
  + authenticated MCP
  + proposal-first, revision-safe writes
```

Caching, hot memory, deep traversal, model-assisted proposals, Dream, and fallback improve latency or reasoning quality but never participate in the fundamental correctness of shared knowledge.
