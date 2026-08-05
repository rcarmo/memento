# ADR 0004: Use proposals for shared writes

**Status:** accepted  
**Date:** 2026-07-18

## Decision

Agents contribute shared knowledge through proposals. Creating a proposal, reviewing it and applying it are separate operations with separate role checks.

A proposer can search, read and submit a diff. A curator can approve, reject or request changes. Applying an approved proposal requires the current repository revision and an idempotency key, then uses the same Git transaction path as direct curator writes.

Curators may review proposals they authored when they have write access to every affected path. The proposal records both `author_principal` and `reviewed_by`, preserving that fact in the audit trail. Stale and expired proposals cannot be applied.

## Why

A model response is not enough to establish a shared fact. Several agents may have different local context, and one agent should not be able to redefine common memory without leaving a reviewable record.

Separating review from apply also closes a race: approval does not freeze Git. The apply step checks the revision again before publication.

## Consequences

* Ordinary agent principals receive `reader` and `proposer`; curator access is assigned separately.
* Proposal status contributes to the operator backlog and metrics.
* A curator can inspect the proposal, diff, author and source revision before deciding, including when author and reviewer are the same authenticated principal.
* Every review requires curator role membership and write access to all affected paths.
* Retries after a lost response return the recorded operation instead of creating a second commit.
* Direct create, patch and rename tools are reserved for controlled curator workflows.
* Model-assisted drafting produces an ordinary proposal and cannot review or apply it.

## Alternatives considered

* **Let every agent write directly:** rejected because prompt injection or mistaken local context would become shared history immediately.
* **Approval automatically commits:** rejected because Git may have moved between review and publication.
* **Let a review model act as curator:** rejected after testing showed correct broad reasoning but unreliable state-changing tool choices, especially around secrets and stale revisions.
* **Require a second curator identity:** rejected because authorship is not an authorisation boundary. Curator role membership, per-path write policy and the recorded review trail provide the intended control without credential or profile switching.
