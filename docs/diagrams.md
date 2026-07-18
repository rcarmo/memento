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

## Model and storage architecture

Memento uses small specialist models behind deterministic boundaries. GTE-small embeds concepts for semantic retrieval. The Rust Needle router classifies shallow read requests. Optional completion-model slots handle answer synthesis, proposal drafting and Dream drafts; they never own policy or persistence.

```mermaid
flowchart LR
    clients[Piclaw and MCP clients] --> auth[Authentication and namespace policy]
    auth --> surface[Compact or full MCP surface]
    surface --> deterministic[Deterministic service methods]

    subgraph localModels[Local embedded models]
        needle[Needle shallow router<br/>26M params / Rust FFI]
        gte[GTE-small embedder<br/>384d / Rust FFI]
    end

    subgraph optionalModels[Optional completion-model slots]
        hot[hot_query<br/>bounded context answer]
        deep[deep_query<br/>read-only traversal answer]
        proposal[proposal<br/>proposal draft only]
        dream[dream<br/>maintenance proposal only]
    end

    surface -->|memory_route| needle
    needle -->|validated shallow action| deterministic
    deterministic -->|concept text and query| gte
    gte -->|normalised vectors| vector[(concept_embeddings)]

    deterministic -->|optional model request| hot
    deterministic -->|optional model request| deep
    deterministic -->|optional model request| proposal
    deterministic -->|optional model request| dream

    deterministic --> git[(Git Markdown<br/>canonical knowledge)]
    deterministic --> control[(control.sqlite<br/>operations and proposals)]
    deterministic --> derived[(derived.sqlite<br/>FTS, graph, vectors, caches)]

    hot -. no direct writes .-> deterministic
    deep -. read-only result .-> deterministic
    proposal -. strict draft .-> deterministic
    dream -. strict draft .-> deterministic
```

Needle and GTE run locally with vendored artefacts. Completion slots can also be local; cross-boundary fallback is opt-in per slot. Deterministic code validates every model result.

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

Direct tool counts vary by surface; optional answers and routing add one tool each where enabled.

```mermaid
flowchart TD
    compact[compact surface]
    readonly[read_only surface]
    standard[standard surface]
    curator[curator surface]
    admin[admin surface]

    compact --> ctools[6 direct tools<br/>7 with memory_answer]
    readonly --> rtools[9 direct tools]
    standard --> stools[20 direct tools]
    curator --> curtools[11 direct tools<br/>12 with memory_answer]
    admin --> atools[21 direct tools]

    curator --> execonly[create / patch / rename are execute-only here]
    standard --> directmut[create / patch / rename are direct tools]
    admin --> directmut2[create / patch / rename are direct tools]
```

## Needle router lifecycle

Needle has two distinct histories: the failed full-plan attempt and the successful shallow router. The router now runs through the embedded Rust FFI runtime; ARM64 performance evidence remains a deployment follow-up.

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
    RouterCheckpointPassed --> RustPort: NDL1 conversion and scalar parity
    RustPort --> SimdOptimised: AVX2/FMA and NEON kernels
    SimdOptimised --> Enabled: 360-case FFI parity and MCP smoke pass
    Enabled --> Disabled: runtime or model parity regression
    Disabled --> RustPort: corrected runtime available
```

The current repository state is `Enabled` when `intelligent_tiers.needle_router.enabled` is true. The default remains disabled so deployments opt into the extra model load explicitly.

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

## Typical request processing

This sequence covers the read path, with local routing and answer synthesis when enabled. Direct tool calls and Needle-routed requests use the same service methods.

```mermaid
sequenceDiagram
    participant Client as Piclaw / MCP client
    participant MCP as uMCP transport
    participant Policy as Auth and namespace policy
    participant Router as Rust Needle router
    participant Service as Memento service
    participant Search as FTS / graph / vector index
    participant GTE as Rust GTE embedder
    participant Answer as Optional answer model
    participant Store as Git and control state

    Client->>MCP: initialize and discover compact tools
    Client->>MCP: memory_route or direct memory_search/read
    MCP->>Policy: authenticate principal and scope
    alt memory_route enabled
        Policy->>Router: classify shallow request
        Router-->>Policy: search/read/status/graph/UNKNOWN
        Policy->>Service: deterministic action expansion
    else direct tool call
        Policy->>Service: validated tool arguments
    end

    alt request needs search
        Service->>Search: authorised lexical candidates
        opt semantic or hybrid mode
            Service->>GTE: embed query locally
            GTE-->>Service: normalised query vector
            Service->>Search: cosine rank authorised vectors
        end
        Search-->>Service: bounded visible results
    end

    alt direct read/result is sufficient
        Service-->>MCP: success envelope
    else memory_answer enabled
        Service->>Answer: bounded untrusted excerpts
        Answer-->>Service: answer plus citations or UNKNOWN
        Service->>Service: validate citations and limits
        Service-->>MCP: success envelope
    end

    opt proposal or mutation requested
        Service->>Store: journal, validate, commit or store proposal
        Store-->>Service: revision and operation result
        Service-->>MCP: attributed result envelope
    end

    MCP-->>Client: structured content and text compatibility content
```

An `UNKNOWN` router result stops before search or mutation. A model-produced proposal still enters the normal review lifecycle below.

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
