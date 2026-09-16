# Memento documentation

Use the workflow guides for agent tasks, the contracts for exact arguments and the operations guide for service administration.

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
| Recover clean changes from a stale proposal | [Stale proposals](proposals.md#handle-a-stale-or-expired-proposal) |
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
| Run load tests | [Load testing](load-testing.md) |
| Check recorded deployment and benchmark results | [Validation reports](evidence/README.md) |

## Model development and project records

* [Needle fine-tuning](needle-fine-tuning.md) and [Needle performance](needle-performance.md) contain router training and measurement details.
* [Attribution](attribution.md) lists third-party code, models and licences.
* [Delivery plan](../PLAN.md) tracks implementation work; [releases](https://github.com/rcarmo/memento/releases) identify published versions.
