# Trash and deletion

Deletion has two stages. `trash` moves `/projects/example.md` to `/trash/projects/example.md`, preserving the concept ID, exact Markdown and accepted asset packs. `restore` moves it back. `purge` deletes the trashed Markdown and all accepted versions of its assets from the current Git tree.

Git history is retained in every case. Purge does not erase old commits, proposal records, staged submissions or backups, and is not a privacy-erasure operation.

[Documentation index](README.md) · [Archive/restore diagram](agent-workflows.md#archive-and-restore-shared-knowledge) · [Review and refile proposals](proposals.md)

## Reviewed archival

To inspect consequences before archiving, submit a proposal with `changes: [{"kind": "trash", "path": "/projects/example.md"}]` and the current `base_revision`. An archival proposal may contain 1--20 distinct trash targets and cannot mix in other mutations. Explicit creation, inspection, curator review and apply use the normal proposal workflow; model-generated archival is not allowed.

The response and `proposal_get` include `archival_impact`: the destination, concept ID, current revision, readable inbound references that will become unresolved, accepted asset kinds/versions/digests that will be retained, and link/history behaviour. Reports are bounded to 100 inbound references and 100 accepted asset versions per target; excessive input fails before a proposal is stored. Hidden backlinks are never listed. A curator inspecting a current proposal receives a fresh report under their own permissions; stored reports are filtered again if grants change.

Approval and apply require a current content index and repository revision. Stale archival proposals must be submitted again for a fresh impact report rather than adopted through generic subset revision. Source or destination conflicts prevent the whole transaction. Apply uses a durable idempotency key and can be reconciled through `operation_get`; retries do not archive twice. Use reviewed archival for routine cleanup. Direct curator operations below remain available for exceptional use.

## Authenticated operations

Use `memory_execute` on the compact MCP surface, or `memory_trash`, `memory_restore` and `memory_purge` on the standard/curator/admin surfaces. Each requires the curator role, read and write permission on the original path, a fresh `expected_revision` and a durable `idempotency_key`. Purge additionally requires `confirm: true` and only accepts a path already under `/trash/`.

```json
{
  "plan": {
    "operations": [{
      "op": "trash",
      "args": {
        "path": "/projects/example.md",
        "expected_revision": "<current revision>",
        "idempotency_key": "trash-example-1"
      }
    }]
  }
}
```

For restore, use `op: "restore"` and the returned trash path. For purge, use `op: "purge"`, the trash path and `confirm: true`. Each is a separate commit-capable operation; a plan can contain at most one. Reconcile interrupted requests through `operation_get` before retrying with the same key.

The embedded original path determines permissions. A grant on `/trash/` does not grant access to `/trash/secret/` or any other original namespace. Changing the original namespace grants changes trash visibility too. Ordinary create, patch, rename and attachment proposals cannot write into `/trash/`; use these operations instead.

Destination collisions fail without replacing either item. This includes a second item already at the same trash path, or a replacement created at the original path before restore. Inbound links are deliberately left untouched: they stop resolving while the target is trashed and resolve again on restore. They remain broken after purge unless separately curated.

## Finding trashed items

Normal search, inventory and graph overview exclude trashed concepts. Inventory with `path_prefix: "/trash/"` opts in, subject to the original namespace permissions. Exact reads of authorised trash paths remain available for review.

The graph's **Show Trash** switch adds the Trash namespace to the overview. Aggregate views keep a named Trash cluster; direct views label its group. Graph search still searches active items. The visual debugger remains an unauthenticated read-only diagnostic interface -- it does not expose delete or restore buttons that could bypass MCP authentication. Its full diagnostic view can see all trash just as it can see other content; simulated principal views apply the original path restrictions.
