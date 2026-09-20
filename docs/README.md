# Memento documentation

Use the workflow guides for agent tasks, the contracts for exact arguments and the operations guide for service administration. Current operational pages describe the pure-Go `v1.0.0` replacement; dated evidence, ADRs and completed work items preserve the historical Python/Rust baseline where relevant.

## Pure-Go replacement

The [pure-Go port index](go-port/README.md) documents the completed CGO-free daemon, standalone uMCP module, storage/recovery implementation, GTE/Needle runtimes, SIMD results, allocation budgets and strict quality gates. The release image contains only static Go executables and model/data assets.

## Connect a client

| Client or task | Guide |
| --- | --- |
| Pi | [Pi setup](setup-pi.md) |
| Piclaw | [Piclaw setup](setup-piclaw.md) |
| Codex | [Codex setup](setup-codex.md) |
| Give an agent a scoped credential | [Access management](access-management.md) |
| Load the compact agent instructions | [Memento Agent Skill](../.agents/skills/memento/SKILL.md) |

## Work with shared memory

| Task | Start here |
| --- | --- |
| Discover tools, search and read | [Agent read workflow](agent-workflows.md#discover-search-and-read) |
| Compare local documents with shared concepts | [Inventory and comparison](agent-workflows.md#compare-local-and-shared-content) |
| Chain operations with saved values | [Execute plans](agent-workflows.md#chain-operations-with-saved-results) |
| Submit a change and inspect the result | [Proposal submission](proposals.md#submit-and-review) |
| Approve, reject or request changes | [Review decisions](proposals.md#choose-a-review-decision) |
| Correct and refile a proposal | [Refile-corrected-content](proposals.md#refile-corrected-content) |
| Rebase a clean proposal or resolve conflicts without losing the queue | [Rebase and conflict handling](proposals.md#handle-a-stale-or-expired-proposal) |
| Publish or update a packaged skill | [Skill review](proposals.md#review-a-skill-or-asset-update) |
| Retrieve accepted asset files or ZIPs | [Asset recall](agent-workflows.md#retrieve-an-accepted-skill-or-asset) and [range contract](accepted-assets.md) |
| Resolve a lost mutation response | [Reconciliation](agent-workflows.md#reconcile-an-interrupted-write) |
| Archive, restore or purge a concept | [Trash](trash.md) |

## Check an interface or boundary

* [MCP contracts](contracts.md) define tool arguments, roles, response envelopes, limits and execute operations. The live `memory://catalog` resource reflects the connected deployment.
* [Implementation](implementation.md) describes repository layout, transactions, proposal storage, indexes and runtime components.
* [System diagrams](diagrams.md) show transport, storage, publication, recovery and optional model flows. Agent task diagrams are in [agent workflows](agent-workflows.md) and [proposals](proposals.md).
* [Threat model](threat-model.md) defines trust boundaries and abuse cases. [Architecture decisions](decisions/README.md) record design choices.

## Run and diagnose the service

| Task | Guide |
| --- | --- |
| Deploy, check health, back up or recover | [Operations](operations.md) |
| Configure a DiskStation deployment | [DiskStation](diskstation.md) |
| Manage namespaces, principals and credentials | [Access management](access-management.md) |
| Configure embeddings and progressive refresh | [Semantic search](semantic-search.md) |
| Inspect links, assets, proposals and visibility in the trusted graph UI | [Graph debugger](graph-explorer-plan.md) |
| Build, publish or deploy a release | [Release process](release.md) |
| Check v1 validation, recorded deployments and benchmarks | [Validation reports](evidence/README.md) |

## Model development and project records

* [Needle evidence](evidence/needle/README.md) contains retained router corpus, training and performance records.
* [Attribution](attribution.md) lists third-party code, models and licences.
* [Delivery plan](../PLAN.md) records completed scope and the remaining operator-owned deployment work; [releases](https://github.com/rcarmo/memento/releases) identify published versions.
