---
name: memento
description: Use Memento shared memory effectively through MCP. Covers discovery, scoped search and reading, inventory and manifest comparison, proposals and curation, assets and shared skills, namespace policies, retry reconciliation, and the trusted graph debugger. Use whenever an agent needs to recall, file, compare, review, or diagnose durable shared knowledge.
license: MIT
compatibility: Requires an MCP client connected to a Memento Streamable HTTP endpoint.
---

# Memento

Memento stores durable shared concepts as Markdown in Git. Use it for facts that should survive a chat and be visible to other authorised agents. Keep conversations, credentials, reminders, schedules and machine-local scratch state elsewhere.

## Orient First

Read service status before making assumptions about the deployment:

```text
memory_status
```

Use `memory_help` or `memory://catalog` when you need operation names, schemas or workflow templates. Compact deployments expose common tools directly and route less common operations through `memory_execute`.

Client setup is covered in:

* Pi: `docs/setup-pi.md`
* Piclaw: `docs/setup-piclaw.md`
* Codex: `docs/setup-codex.md`

## Read Workflow

Search before reading unless you already have an exact path:

```text
memory_search(query="embedding worker", limit=10)
memory_read(id_or_path="/projects/memento.md")
```

Use `query_syntax="plain"`, the default, for ordinary terms. Plain mode tokenises natural language and treats punctuation and operator words literally. Use `query_syntax="fts5"` only when you deliberately supply an FTS5 phrase, prefix or boolean expression. Punctuation-only plain queries and malformed raw FTS5 expressions return `validation_error`.

Request semantic or hybrid search only when status says embeddings are ready. Treat returned paths as opaque identifiers and pass them back exactly.

For a bounded compound read, use `memory_execute` with saved references:

```json
{
  "plan": {
    "operations": [
      {
        "op": "search",
        "args": {"query": "DiskStation Memento", "limit": 5},
        "save_as": "hits"
      },
      {
        "op": "read",
        "args": {"id_or_path": "$hits.results.0.path"},
        "save_as": "memory"
      }
    ],
    "returns": [
      {"name": "memory", "ref": "$memory"}
    ],
    "stop_on_error": true
  }
}
```

Keep plans small. Use saved references instead of copying paths between steps. Project only the fields you need when responses may be large.

For a bounded namespace overview, use `memory_inventory`. It returns stable path-ordered metadata, canonical body digests and asset summaries without concept bodies. Use its `next_cursor` for another page rather than raising the limit beyond the advertised ceiling.

To compare local documents with Memento, build a local manifest and pass it to the execute-only `compare_manifest` operation. It accepts at most 50 caller-supplied rows and one authorised inventory page, then separates matching, differing, local-only and Memento-only records. Memento never reads the local paths; treat them as opaque labels. Do not look for a direct `compare_manifest` tool.

When `memory_answer` is enabled, use it for a bounded cited answer rather than as a shortcut around search policy:

```text
memory_answer(question="Which rack owns Atlas?", answer_mode="summary")
```

Inspect `evidence` as well as the prose. The query profile, authorisation fingerprint, retrieval strategy, ranks, concept state and support chains explain what reached the reader. Graph-derived citations include their primary anchor chain. `policy_abstention` means the question requested secret material and no cache, retrieval or model call ran; `evidence_abstention` means authorised support was insufficient. Do not rephrase either result to evade the policy.

Explicit personal/work wording narrows evidence to that namespace. Current questions exclude deprecated, stale and conflicting concepts, while historical wording may deliberately retain them.

## Decide What Belongs

Good shared memories include:

* project purpose, architecture and durable constraints;
* service and instance relationships;
* reasons behind accepted technical decisions;
* reusable engineering practices;
* stable user preferences that affect multiple agents;
* reviewed skills and their versioned asset packs.

Do not file:

* passwords, bearer tokens, private keys or credential locations that reveal them;
* complete conversations or private reasoning;
* reminders and schedules;
* transient task progress or build output;
* guesses presented as facts;
* information outside the caller's authorised namespace.

Search for the subject first. Prefer enriching an existing concept over creating a near-duplicate.

## Write Workflow

Proposers submit explicit changes against the current repository revision. Curators review and apply them. Direct curator writes are useful for small, factual maintenance changes.

A proposal through `memory_execute` looks like:

```json
{
  "plan": {
    "operations": [
      {
        "op": "propose",
        "args": {
          "intent": "Record the service deployment model",
          "base_revision": "<repo_revision from memory_status>",
          "rationale": "The fact is durable and useful to several agents.",
          "changes": [
            {
              "kind": "patch",
              "path": "/projects/memento.md",
              "body": "Updated reviewed body",
              "tags": ["mcp", "memory"]
            }
          ]
        },
        "save_as": "proposal"
      }
    ],
    "stop_on_error": true
  }
}
```

Review and apply are separate operations. An authenticated curator may review a proposal they authored when their policy grants write access to every affected path. Use the same curator profile for propose, review, apply and retrieval; do not swap credentials to manufacture a second identity. An apply operation is commit-capable, so keep at most one commit-capable operation in an execution plan.

Direct creates and patches require:

* `expected_revision` from fresh status;
* a stable, unique `idempotency_key`;
* a path inside the caller's write prefixes.

If a mutation times out or the connection drops, reconcile before retrying:

1. read status and compare repository revision;
2. read the target path;
3. inspect the proposal or operation using the same idempotency key;
4. retry only when the first attempt did not commit.

## Namespaces

Paths define knowledge domains. A deployment can share `/skills/` and `/public/` while isolating `/work/`, `/personal/` and `/infrastructure/` through principal read/write prefixes.

When `authorization.protected_read_prefixes` is configured, a broad `/` read grant covers only unprotected paths. A non-admin principal needs an explicit equal or nested read prefix for each protected namespace it should see. The `admin` role bypasses that mask but does not imply ordinary read or write roles.

Never accept a principal name as a memory operation argument. Identity comes from the authenticated MCP request. Search ranking, inventory, graph traversal and writes are filtered by the effective namespace policy before content is ranked, parsed or returned.

The trusted `/graph` debugger can show the full repository and simulate managed principals with **View as**. Simulation is diagnostic only and is labelled as not being an authorisation boundary.

## Skills And Assets

A shared skill is an ordinary concept under `/skills/`, tagged `skill`, with an attached versioned asset pack whose root `SKILL.md` matches the concept body byte-for-byte. Before submission, both use canonical UTF-8 text with LF endings, no trailing whitespace, no leading or trailing blank lines and no final newline. Unicode code points are preserved rather than converted between NFC and NFD. Non-canonical or mismatched skill submissions are rejected.

Publish packs that fit the configured MCP request ceiling with an `attach_asset_pack` change containing `zip_base64`. This keeps the complete proposal inside MCP. Read the created proposal and verify its generated manifest, SHA-256, target path, asset kind and version before review. After apply, retrieve the accepted version with `memory_asset_get`, then verify the returned manifest, SHA-256 and decoded ZIP bytes before installing it. Memento stores and returns skill packs; it does not execute them.

Use staging when the complete base64 request would exceed the MCP ceiling, or when the client deliberately uses the separate raw binary HTTP path: call `memory_asset_stage_begin`, upload the raw file to its returned `upload_path` with the one-time `X-Memento-Upload-Ticket`, then call `memory_asset_stage_status` and put its `staged_asset_id` in the proposal. Do not pass an agent bearer token to the upload command; the ticket is short-lived, principal-bound and single-use.

Skill changes should normally use proposals so a curator reviews both Markdown and packaged files.

## Graph And Audit

Use `memory_graph` for authorised concept neighbourhoods. Use `/graph` for human diagnosis of links, tags, proposals, assets, search, embeddings and simulated principal visibility.

Broken links, missing embeddings and orphaned concepts are derived diagnostics. Fix canonical Markdown or request refresh rather than editing SQLite directly. Derived state remains rebuildable from Git, but persisted `derived.sqlite` preserves expensive embeddings across container upgrades and rebuilds; do not delete it casually. Progressive deployments regenerate missing/stale paths over time and manual refresh uses the same gated worker.

## Safety Checks

Before reporting success:

* confirm repository and index revisions match;
* confirm the intended path is readable by the expected principal;
* confirm proposal backlog or operation status when writes were involved;
* verify links resolve and tags are present;
* verify embedding readiness only when semantic behaviour matters;
* keep tokens and client configuration out of concepts and logs.

## Access Management

Principals are managed through `/admin` or admin-only `access_*` tools on the existing MCP endpoint. The access tools are direct MCP tools, not `memory_execute` operations; non-admin agents cannot discover or invoke them.

Use a separate administrator profile with `admin`, `reader`, root read access and no content write prefixes. Give curators `reader`, `proposer`, `curator` and only the namespaces they manage. Ordinary agents use their own reader/proposer credentials. Do not configure or use the administrator token in an ordinary agent runtime, memory operation, chat or tool input/output.

Create and rotate return new principal credentials once. Capture them directly into the intended keychain or secret store, remove temporary files, and never file them in Memento or copy them into concepts, chat or logs. Keep ordinary agents off the `sandbox` bootstrap credential. The separate-runtime Piclaw and Pi setup is in `docs/access-management.md`.
