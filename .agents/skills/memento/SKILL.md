---
name: memento
description: Use Memento shared memory effectively through MCP. Covers discovery, scoped search and reading, proposals and curation, assets and shared skills, namespace policies, retry reconciliation, and the trusted graph debugger. Use whenever an agent needs to recall, file, review, or diagnose durable shared knowledge.
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

* [`../../../docs/setup-pi.md`](../../../docs/setup-pi.md)
* [`../../../docs/setup-piclaw.md`](../../../docs/setup-piclaw.md)
* [`../../../docs/setup-codex.md`](../../../docs/setup-codex.md)

## Read Workflow

Search before reading unless you already have an exact path:

```text
memory_search(query="embedding worker", limit=10)
memory_read(id_or_path="/projects/memento.md")
```

Use ordinary terms for lexical search. Request semantic or hybrid search only when status says embeddings are ready. Treat returned paths as opaque identifiers and pass them back exactly.

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

Review and apply are separate operations. Do not assume proposal authors may approve their own submissions; deployments can enforce separation. An apply operation is commit-capable, so keep at most one commit-capable operation in an execution plan.

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

Never accept a principal name as a memory operation argument. Identity comes from the authenticated MCP request. Search ranking, graph traversal and writes are filtered by the effective namespace policy.

The trusted `/graph` debugger can show the full repository and simulate configured principals with **View as**. Simulation is diagnostic only and is labelled as not being an authorisation boundary.

## Skills And Assets

A shared skill is an ordinary concept under `/skills/`, tagged `skill`, with an attached versioned asset pack whose root `SKILL.md` matches the concept body.

Use `memory_asset_get` with the concept path, `asset_kind="skill"` and an optional version. Validate the returned digest and manifest before installing files into an agent skill directory. Memento stores and returns skill packs; it does not execute them.

Skill changes should normally use proposals so a curator reviews both Markdown and packaged files.

## Graph And Audit

Use `memory_graph` for authorised concept neighbourhoods. Use `/graph` for human diagnosis of links, tags, proposals, assets, search, embeddings and simulated principal visibility.

Broken links, missing embeddings and orphaned concepts are derived diagnostics. Fix canonical Markdown or refresh the derived index rather than editing SQLite directly. Derived state can be deleted and rebuilt from Git; concept Markdown and control records are the durable state.

## Safety Checks

Before reporting success:

* confirm repository and index revisions match;
* confirm the intended path is readable by the expected principal;
* confirm proposal backlog or operation status when writes were involved;
* verify links resolve and tags are present;
* verify embedding readiness only when semantic behavior matters;
* keep tokens and client configuration out of concepts and logs.
