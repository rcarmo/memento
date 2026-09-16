# Submit, review and refile proposals

A proposal stores proposed changes for curator review. Git changes only when an authorised curator applies an approved proposal. Corrected content is submitted under a new proposal ID; there is no edit-and-resubmit API for an existing draft.

[Documentation index](README.md) · [Agent workflows](agent-workflows.md) · [Exact contracts](contracts.md#proposal-records-and-lifecycle)

Operation names below are `memory_execute` operations. Use the corresponding direct `memory_*` tool only when the connected catalog exposes it. `memory_status`, `memory_help` and `memory://catalog` describe the deployment. Proposing requires the `proposer` role and write access to the affected paths. Proposal get/list and staged-asset inspection also require `proposer`. Review, revision and apply require `curator` plus read/write access to every affected path. Give a working curator profile `reader`, `proposer` and `curator` roles with the intended namespace grants.

## Submit and review

Search and read the current concepts before preparing changes. `propose` creates a new record directly in `submitted` state. It has no separate submit step. Save the returned `proposal_id`, then inspect its diff, target paths, base revision and any asset manifest before requesting review.

```mermaid
sequenceDiagram
    participant A as Proposer agent
    participant M as Memento
    participant C as Curator agent
    participant G as Git and indexes
    A->>M: search / read / status
    M-->>A: Current content and repo_revision
    A->>M: propose(changes, base_revision, intent, rationale)
    M->>M: Validate roles, paths, changes and assets
    M-->>A: New proposal_id with status submitted
    A-->>C: Send proposal_id through an approved channel
    C->>M: proposal_get(proposal_id)
    M-->>C: Diff, conflicts, metadata and asset manifest
    opt Proposal includes assets
        C->>M: proposal_asset_get(proposal_id, asset_id, view)
        M-->>C: Bounded staged file or manifest
    end
    C->>M: proposal_review(decision, comment)
    M-->>C: Updated status and review comment
    alt Approved and base revision is current
        C->>M: proposal_apply(expected_revision, idempotency_key)
        M->>G: Publish Git commit and update indexes
        M-->>C: Applied proposal, revision and operation_id
        C->>M: read / asset_get / status
    else Changes needed or rejected
        A->>M: proposal_get(proposal_id)
        M-->>A: Review decision and requested corrections
        Note over A,M: Refile corrected content as a new proposal
    end
```

Memento stores the review result. Agents arrange hand-off through their approved messaging channel or inspect `proposal_list`; proposal creation does not send a peer message. An author can read their own accessible proposals. Curators can inspect other authors' proposals within their namespace grants. A curator can review their own proposal when policy permits it; keep the actual author and reviewer identities.

`proposal_list(status="submitted")` selects the submitted queue. `pending` is not a valid status. Follow `next_cursor` until null when reviewing more than one page.

An explicit patch submission through `memory_execute`:

```json
{
  "operations": [{
    "op": "propose",
    "args": {
      "intent": "Correct the service ownership record",
      "base_revision": "<fresh repo_revision>",
      "rationale": "The owner confirmed the change.",
      "changes": [{
        "kind": "patch",
        "path": "/projects/example.md",
        "body": "<complete revised body, retaining facts that still apply>"
      }]
    },
    "save_as": "submission"
  }],
  "returns": [{"ref": "$submission.proposal", "fields": ["proposal_id", "status", "base_revision"]}]
}
```

Replace example paths and angle-bracket values before calling. `patch.body` replaces the body; supply the whole intended body. `propose_update` is a model-assisted draft from an instruction, not an edit to a stored proposal. Model-assisted submission requires the deployment's proposal model configuration.

## Choose a review decision

| Decision | Stored status | Use and next action |
| --- | --- | --- |
| `approve` | `approved` | The reviewed changes are ready. Apply separately while the base revision is current. |
| `request_changes` | `draft` | Describe the corrections. The author files corrected content as a new proposal. |
| `reject` | `rejected` | Record why these changes should not be applied. If a corrected replacement is appropriate, file a new proposal and cite the old ID. |

Review changes status and review metadata; it does not edit the stored patch. Applied and expired proposals cannot be reviewed again. Other states, including draft and rejected, can receive a later curator decision on the *same unchanged patch*. Check freshness before approval: submitted/approved proposals with an old base become stale when status is refreshed. A new decision is not a rebase.

These are the normal submission and recovery paths; later review decisions on unchanged patches are described in the table above.

```mermaid
stateDiagram-v2
    direction TB
    [*] --> submitted: propose creates a new ID
    submitted --> approved: approve
    submitted --> draft: request_changes
    submitted --> rejected: reject
    approved --> draft: request_changes before apply
    approved --> rejected: reject before apply
    submitted --> stale: base differs on status refresh
    approved --> stale: base differs on status refresh
    approved --> applied: apply at expected revision
    state "new submitted proposal" as replacement
    draft --> replacement: correct locally and propose
    rejected --> replacement: refile if appropriate
    stale --> replacement: refile or curator revise clean changes
    expired --> replacement: prepare fresh proposal
    note right of expired
        Any unapplied proposal can expire.
        Default TTL is 30 days.
    end note
    note right of replacement
        New proposal ID; review again.
        Source record is retained.
    end note
    applied --> [*]
```

A changes-requested review call:

```json
{
  "operations": [{
    "op": "proposal_review",
    "args": {
      "proposal_id": "<proposal_id>",
      "decision": "request_changes",
      "comment": "Include the source for the ownership change and retain the existing recovery instructions. Refile the complete corrected body and cite this proposal ID."
    }
  }]
}
```

For approval, use `decision: "approve"` after inspection. Then issue a separate apply call with `proposal_id`, fresh `expected_revision` and a unique, durable `idempotency_key`. Preserve that exact request and key for [reconciliation](agent-workflows.md#reconcile-an-interrupted-write). Approval itself creates no Git commit.

## Refile corrected content

Read the review comment and current concept before changing the local draft. Address each requested correction, preserve still-valid content and submit against a fresh revision. Include the previous proposal ID and the corrections in `rationale`; this is a human-readable link. Ordinary `propose` does not create a structured supersession relationship.

```mermaid
flowchart TD
    reviewed[Read draft or rejected proposal and review comment] --> current[Read current target and status]
    current --> edit[Correct local content and any asset pack]
    edit --> validate[Check paths, sources, skill parity and new asset version]
    validate --> submit[propose with fresh base_revision and old ID in rationale]
    submit --> new[Save new proposal_id]
    new --> handoff[Send new ID to curator; inspect and review again]
    reviewed -. retained .-> old[Old record and its review remain accessible]
```

There is no `proposal_edit`, `proposal_resubmit` or operation that replaces a draft's patch. Avoid repeated `propose` calls after a lost submission response: proposal creation has no idempotency-key argument. Inspect your proposal list for the recorded submission before deciding whether to file another.

## Handle a stale or expired proposal

A changed repository revision can make a proposal stale even when its own paths did not change. Inspect `proposal_get` for current/base revisions and per-change conflict results. Approval does not make an old base current.

```mermaid
flowchart TD
    state[Inspect proposal status and conflicts] --> expired{Expired?}
    expired -->|yes| fresh[Read current content and file a fresh proposal]
    expired -->|no; stale| clean{Selected changes are clean?}
    clean -->|no| fresh
    clean -->|yes| archival{Any selected trash change?}
    archival -->|yes| impact[File fresh archival proposal with current impact]
    archival -->|no| curator[Curator calls proposal_revise with indexes and expected_revision]
    curator --> submitted[New submitted ID with source linkage and copied selected assets]
    fresh --> submittedFresh[New submitted ID]
    impact --> submittedFresh
    submitted --> review[Inspect, review and apply separately]
    submittedFresh --> review
```

`proposal_revise` accepts only a stale source. It copies selected clean changes into a new submitted proposal authored by the curator, using the current revision. It records `source_proposal_id` and `source_change_indexes`, retains the source and requires paired concept/asset changes to stay together. It cannot edit selected content. Conflicting changes require a freshly prepared proposal. Expired proposals also require fresh submission.

```json
{
  "operations": [{
    "op": "proposal_revise",
    "args": {
      "proposal_id": "<stale proposal_id>",
      "selected_change_indexes": [0],
      "expected_revision": "<fresh repo_revision>"
    }
  }]
}
```

For archival, obtain a fresh impact report through a new trash-only proposal. See [reviewed archival](trash.md#reviewed-archival). Do not confuse `proposal_revise` with Git history rewriting.

## Review a skill or asset update

Review both the concept and the packaged files. For a skill, ZIP-root `SKILL.md` must match the proposed concept body byte-for-byte in canonical UTF-8: LF endings, no trailing whitespace, no leading/trailing blank lines and no final newline. Preserve Unicode code points. Include the revised scripts and package metadata required to use the skill.

```mermaid
flowchart TD
    local[Prepare corrected concept and package] --> parity[Verify root SKILL.md matches concept body]
    parity --> version[Choose a new accepted-asset version]
    version --> size{Complete base64 request fits MCP limit?}
    size -->|yes| inline[attach_asset_pack with zip_base64]
    size -->|no| ticket[asset_stage_begin; upload with one-time ticket]
    ticket --> ready[asset_stage_status returns staged_asset_id]
    ready --> staged[attach_asset_pack with staged_asset_id]
    inline --> proposal[propose concept and asset changes together]
    staged --> proposal
    proposal --> inspect[Curator inspects diff, manifest, digest and selected files]
    inspect --> decision{Review}
    decision -->|corrections| local
    decision -->|approved| apply[Apply with revision and idempotency key]
    apply --> recall[Read accepted manifest and verify downloaded bytes]
```

When refiling, package corrected bytes and use a version not already accepted for that concept/kind. An upload stage consumed by the old proposal cannot be reused; stage the replacement again or use inline ZIP bytes. The stale-proposal `proposal_revise` path can copy selected stored assets without uploading them again.

Use `proposal_get` to obtain the generated asset ID and manifest, then `proposal_asset_get(view="file", file_path=...)` to inspect bounded content before approval. After apply, `asset_get` reads accepted versions. [Accepted-asset retrieval](accepted-assets.md) defines range, digest and EOF handling. Any client-side installation or script execution needs its own authorisation.

The lifecycle rules are implemented in [`control/proposals.py`](../src/memento/control/proposals.py) and [`service.py`](../src/memento/service.py); operation arguments are in [`executor.py`](../src/memento/executor.py). The [contracts](contracts.md#proposal-records-and-lifecycle) define the exposed fields.
