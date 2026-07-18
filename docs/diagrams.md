# Memento transition diagrams

These diagrams describe the implemented control flow and durable states. They use the same names as the Python models, SQLite rows, MCP tools and Git references.

## Shared memory boundary

Piclaw instances share durable concepts through Memento. Conversations, local Dream memory, schedules and credentials do not cross this boundary.

```mermaid
flowchart LR
    smith[Smith Piclaw] -->|authenticated MCP| memento[Memento]
    flint[Flint Piclaw] -->|authenticated MCP| memento
    other[Other client] -->|authenticated MCP| memento

    memento -->|canonical concepts| git[(Git Markdown)]
    memento -->|operations and proposals| control[(control.sqlite)]
    memento -->|FTS, graph and vectors| derived[(derived.sqlite)]

    smith -. private .-> smithLocal[Chats, Dream, schedules, keychain]
    flint -. private .-> flintLocal[Chats, Dream, schedules, keychain]
```

Git owns knowledge. The control database owns durable operation state. Derived indexes can be deleted and rebuilt.

## Compact MCP request routing

The compact MCP surface keeps detailed operation schemas out of initial model context. Catalog and workflow resources disclose them when needed.

```mermaid
stateDiagram-v2
    [*] --> Authenticate
    Authenticate --> Forbidden: invalid token or principal
    Authenticate --> Discover: valid principal

    Discover --> DirectRead: help, status, search or read
    Discover --> ExecutePlan: memory_execute
    Discover --> CatalogRead: memory://catalog or workflow
    Discover --> OptionalAnswer: memory_answer enabled

    CatalogRead --> Discover: operation selected
    ExecutePlan --> ValidatePlan
    ValidatePlan --> Rejected: invalid reference, limit or authority
    ValidatePlan --> Dispatch: bounded typed operations
    Dispatch --> DirectRead: read operation
    Dispatch --> CommitPipeline: one commit-capable operation at most

    DirectRead --> Envelope
    OptionalAnswer --> Envelope
    CommitPipeline --> Envelope
    Envelope --> [*]
    Forbidden --> [*]
    Rejected --> [*]
```

Every dispatched operation still enters the ordinary service method, so compact execution cannot bypass authorisation or mutation rules.

## Compact surfaces and execute-only operations

This is the part that is easy to blur in prose, so the diagram makes it explicit.

```mermaid
flowchart TD
    compact[compact surface]
    readonly[read_only surface]
    standard[standard surface]
    curator[curator surface]
    admin[admin surface]

    compact --> ctools[5 direct tools<br/>or 6 with memory_answer]
    readonly --> rtools[8 direct tools]
    standard --> stools[18 direct tools]
    curator --> curtools[9 direct tools<br/>or 10 with memory_answer]
    admin --> atools[19 direct tools]

    curator --> execonly[create / patch / rename are execute-only here]
    standard --> directmut[create / patch / rename are direct tools]
    admin --> directmut2[create / patch / rename are direct tools]
```

## Needle router lifecycle

Needle now has two distinct histories: the failed full-plan attempt and the later successful shallow router. The passing checkpoint is still not enabled.

```mermaid
stateDiagram-v2
    [*] --> FullPlanBaseline
    FullPlanBaseline --> FullPlanEvaluatedAMD64: offline baseline run
    FullPlanEvaluatedAMD64 --> FullPlanRejected: low routing, weak UNKNOWN, invalid plans
    FullPlanRejected --> FullPlanFineTuned: local fine-tune experiment
    FullPlanFineTuned --> FullPlanStillRejected: unseen-family and plan gates still fail

    FullPlanStillRejected --> ShallowRouterDesign: stop generating nested plans
    ShallowRouterDesign --> ShallowRouterFineTuned: family-separated shallow corpus
    ShallowRouterFineTuned --> RouterCheckpointPassed: 100% AMD64 held-out routing and UNKNOWN gates
    RouterCheckpointPassed --> RuntimePending: embedded runtime and ARM64 parity still pending
    RuntimePending --> Enabled: pinned embedded or Cactus runtime passes parity
    Enabled --> Disabled: runtime or parity regression
    Disabled --> RuntimePending: corrected runtime available
```

Passing the router checkpoint does not enable the runtime. The current repository state is `RuntimePending`.

## Needle shallow router action boundary

Needle classifies a request into a shallow action. It does not generate Git mutations, authoritative paths or nested execution plans. Memento expands those actions deterministically.

```mermaid
flowchart TD
    request[User request] --> enabled{Embedded Needle runtime enabled?}
    enabled -->|no| normal[Normal MCP and configured model paths]
    enabled -->|yes| classify[Needle shallow classification]

    classify --> unknown{Action is UNKNOWN?}
    unknown -->|yes| abstain[Return safe abstention]
    unknown -->|no| validate[Validate strict shallow schema]

    validate -->|invalid| abstain
    validate -->|valid| action{Shallow action}

    action -->|search_paths| search[Direct memory_search]
    action -->|status_field| status[Direct memory_status plus field projection]
    action -->|read_field| read[Direct memory_read plus field projection]
    action -->|search_then_read| expandRead[Build fixed search-then-read plan]
    action -->|search_then_graph| expandGraph[Build fixed search-then-graph plan]

    expandRead --> executor[memory_execute validator]
    expandGraph --> executor
    search --> result[Bounded result]
    status --> result
    read --> result
    executor --> result
```

## Proposal lifecycle

Models and ordinary clients may create proposals. Only authorised curators can review and apply them.

```mermaid
stateDiagram-v2
    [*] --> Draft
    Draft --> Submitted: submit proposal
    Submitted --> Approved: curator approves
    Submitted --> Rejected: curator rejects
    Submitted --> Draft: curator requests changes
    Submitted --> Stale: base revision changes
    Submitted --> Expired: TTL elapses

    Approved --> Applied: memory_proposal_apply succeeds
    Approved --> Draft: curator requests changes
    Approved --> Rejected: curator rejects before apply
    Approved --> Stale: expected revision no longer matches

    Applied --> Applied: identical idempotent replay

    Draft --> [*]
    Rejected --> [*]
    Stale --> [*]
    Expired --> [*]
    Applied --> [*]
```

Model-assisted proposal creation enters the same ordinary lifecycle. It does not gain review or apply powers.

## Canonical mutation publication

The detached worktree is an isolation and recovery boundary. Readers do not see the mutation until the Git ref is published and `current/` advances.

```mermaid
sequenceDiagram
    participant Client
    participant Service
    participant Control as control.sqlite
    participant Worktree
    participant Git as repo.git/main
    participant Current as current/
    participant Index as derived.sqlite

    Client->>Service: commit request(expected_revision, idempotency_key)
    Service->>Control: insert operation = queued
    Service->>Git: read main as base_revision
    alt expected revision differs
        Service->>Control: operation = conflict
        Service-->>Client: conflict envelope
    else revision matches
        Service->>Control: operation = running
        Service->>Worktree: create detached tree at base_revision
        Service->>Worktree: apply, validate and stage exact paths
        Worktree->>Git: create commit object
        Service->>Git: update-ref main new_commit old_base
        alt compare-and-swap fails
            Service->>Control: operation = conflict
            Service-->>Client: conflict envelope
        else publication succeeds
            Service->>Current: materialise new revision
            Service->>Index: update FTS, graph and optional vectors
            Service->>Control: operation = succeeded
            Service-->>Client: success with result revision
        end
    end
```

The worktree decision and measured overhead are recorded in [ADR 0001](decisions/0001-keep-operation-worktrees.md).

## Operation recovery

Interrupted journal rows are reconciled with canonical Git history before abandoned worktrees are removed.

```mermaid
stateDiagram-v2
    [*] --> Queued
    Queued --> Running: writer starts
    Running --> Succeeded: ref, current and indexes advance
    Running --> Conflict: main moved
    Running --> Failed: validation or permanent error
    Running --> Recovering: process stops mid-operation

    Recovering --> Succeeded: worktree commit equals published main
    Recovering --> Conflict: base revision is stale
    Recovering --> Queued: safe retry remains possible
    Recovering --> Failed: state cannot be reconciled

    Succeeded --> [*]
    Conflict --> [*]
    Failed --> [*]
```

Idempotent callers observe the stored successful result rather than creating a second commit.

## Search and semantic degradation

Lexical search remains available when optional semantic components are missing or stale.

```mermaid
stateDiagram-v2
    [*] --> LexicalReady
    LexicalReady --> SemanticBuilding: model and FFI available
    SemanticBuilding --> SemanticReady: embedding revision equals repository revision
    SemanticBuilding --> SemanticDegraded: load or embedding failure

    SemanticReady --> SemanticStale: repository or model revision changes
    SemanticStale --> SemanticBuilding: incremental update or rebuild
    SemanticReady --> SemanticDegraded: runtime or vector failure
    SemanticDegraded --> SemanticBuilding: operator repairs runtime

    LexicalReady --> LexicalResult: lexical request
    SemanticReady --> SemanticResult: semantic request
    SemanticReady --> HybridResult: hybrid request
    SemanticDegraded --> LexicalFallback: semantic or hybrid request
    SemanticStale --> LexicalFallback: strict semantic readiness unavailable
```

Semantic failure does not roll back a canonical write. Memento advances lexical indexes, records degraded semantic readiness and returns an explicit warning.

## Dream maintenance

Dream scans deterministic repository signals. Model use is optional and can only produce ordinary proposals.

```mermaid
stateDiagram-v2
    [*] --> Disabled
    Disabled --> ReportOnly: operator enables scanner
    ReportOnly --> Scan: durable window claimed
    Scan --> NoSignals: no actionable changes
    Scan --> SignalsRecorded: orphan, broken link, duplicate, oversized or recent activity
    SignalsRecorded --> ReportOnly: report_only mode
    SignalsRecorded --> Propose: propose mode and budget available
    Propose --> ProposalSubmitted: strict draft validates
    Propose --> ProposalRejected: invalid, unsafe or over budget

    NoSignals --> WindowComplete
    ReportOnly --> WindowComplete
    ProposalSubmitted --> WindowComplete
    ProposalRejected --> WindowComplete
    WindowComplete --> Scan: next durable window
```

Dream never reviews, applies or publishes its own proposal.
