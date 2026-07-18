---
id: versioned-skill-packs
title: Store and recall complete versioned skill packs
status: done
priority: high
created: 2026-07-18
updated: 2026-07-18
completed: 2026-07-18
target_release: next
estimate: XL
risk: high
tags: [work-item, kanban, skills, storage, security]
owner: pi
---
# Store and recall complete versioned skill packs

## Summary
Store complete Piclaw skills as immutable stable-semver ZIP packs while indexing an exact searchable copy of `SKILL.md`. Readers retrieve a complete ZIP; Piclaw imports it into `.pi/skills/<name>/` and refuses an existing destination. Memento never executes, installs, or merges packs.

## Acceptance Criteria
- Skill names match `^[a-z0-9]+(?:-[a-z0-9]+)*$`; versions are stable `MAJOR.MINOR.PATCH` only.
- ZIP root contains `SKILL.md`; its bytes exactly match searchable text.
- Binary assets and scripts are allowed; executable binaries, nested archives, links, traversal, absolute paths and unsafe ZIP entries are rejected.
- Maximum uncompressed size is 50 MiB; limits also cover file count, per-file size and compression ratio.
- Memento generates path/size/media-type/SHA-256 entries and a pack digest; recalled files are non-executable.
- Accepted versions are immutable, highest semver is searchable/default, explicit older versions are retrievable, latest five are retained by curator-approved pruning.
- ZIPs are Git LFS objects; metadata/searchable text are ordinary Git Markdown.
- Proposers submit versions; curators accept/prune; readers retrieve visible packs.
- Retrieval returns the ZIP and never extracts it server-side.

## Implementation Paths
### Path A — Dedicated skill artifact contract (recommended)
Add isolated validator/storage models, explicit skill proposal/retrieval operations and derived latest-version indexing. Keep binary handling out of ordinary concept mutation APIs.

### Path B — General binary attachments
Generalise concepts to attachments, then specialise skills. Rejected for v1 because it expands the trust and migration surface without another use case.

## Test Plan
- Unit: hostile ZIP corpus, semver ordering, exact SKILL.md match, generated manifest and retention eligibility.
- Integration: propose/review/apply/retrieve through authenticated MCP; Git LFS pointer and immutable-version checks.
- Production: retrieve a real bundled Piclaw skill and verify the ZIP imports cleanly into a fresh workspace.
- Full Python/Rust/container gates remain green.

## Definition of Done
- [x] All acceptance criteria satisfied and verified
- [x] Tests added or updated and passing locally
- [x] Type check clean
- [x] Docs and contracts updated
- [x] Operational migration/backup impact assessed
- [x] End-to-end Piclaw recall verified
- [x] Update history complete with evidence
- [x] Quality score at least 9/10
- [x] Merged to main and moved to done

## Updates
### 2026-07-18 — implementation complete
- Added hostile ZIP validation, immutable stable-semver Git/LFS storage, dedicated durable proposals, role-aware MCP lifecycle, latest-only search, exact ZIP recall, idempotent apply, protected five-version pruning, LFS-aware materialisation and bounded 72 MiB transport configuration.
- Added `memento-skill-import`, which revalidates and atomically imports a recalled pack into `.pi/skills/<name>/`, refuses existing destinations and symlinked parents, and normalises permissions.
- Updated README/contracts/operations/release docs and clarified that any MCP-enabled agent can use Memento; Piclaw is the motivating use case.
- Evidence: focused skill/security/lifecycle tests pass; full gate reached 169 Python tests plus Rust workspace and all load profiles before final hardening.
- Quality: ★★★★★ 10/10 (problem: 2, scope: 2, test: 2, dependencies: 2, risk: 2).

### 2026-07-18 — started
- Created from the completed one-question-at-a-time refinement.
- Moved directly to doing; implementation starts with the isolated ZIP validation boundary.
- Quality: ★★★★★ 10/10 (problem: 2, scope: 2, test: 2, dependencies: 2, risk: 2).

## Notes
Memento is storage-only. Client-side import must fail if `.pi/skills/<name>/` already exists; merge behavior belongs to the client/auditor.

## Links
- `docs/contracts.md`
- `src/memento/repository/`
