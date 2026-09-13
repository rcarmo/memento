# Memento 0.5.4 deployment and issue #20

## Release and validation

* Release: <https://github.com/rcarmo/memento/releases/tag/v0.5.4>
* Tagged commit: `53e349de6f05c67775fa88aa142ccb4f818a6fb1`
* Published: 2026-09-13 20:56:45 UTC
* OCI index: `sha256:dea78c4a331b090a7b79a670fe3036944a82db8d843b0953d1ec3ee0d4b29528`
* Exact-commit CI: <https://github.com/rcarmo/memento/actions/runs/34781883784>
* Release workflow: <https://github.com/rcarmo/memento/actions/runs/34782010900>

The reviewed archival workflow shipped in 0.5.2. Before deployment, the requested semantic opacity and shared-force fixes were included in 0.5.3. Live verification after archival exposed a missing Show Trash request header; 0.5.4 fixes it and adds a browser test that checks both inclusion and removal of Trash nodes.

All release gates passed, including Python 3.12--3.14, wheel installation, Rust checks, amd64/arm64 image publication and baseline CPU smoke tests. The downloaded OCI index independently hashed to the published digest. Local `make check` passed 360 tests; the preceding coverage run reported approximately 85%. Clean installation of the final wheel imported version 0.5.4. The graph-force browser matrix passed 16 tests with 40 intentional platform skips; the added Show Trash regression passed separately in Chromium.

## Preserved deployment state

Portainer endpoint 18, stack 111 was updated by replacing only the image digest. The explicit original model-volume mapping was retained. Final inspection verified all original mount source/destination/read-write tuples, resource limits, ports, read-only root, dropped capabilities, no-new-privileges and init settings against the pre-0.5.3 container.

* Final container: `78dac9b7e127940f730cd499d14be4104a29186ab7fee4b6a313161444e65c42`
* Local image: `sha256:c42095d21dd1662babbad8890df146dbccdb562662ede192c2f5dc8a7a699d66`
* Started: 2026-09-13 21:08:39 UTC
* Healthy by: 2026-09-13 21:16:33 UTC
* Restart count: zero

Both replacements took several minutes to load the native models. No restart, forced termination, model regeneration or repository rebuild was needed. Unauthenticated MCP returned HTTP 401 with a Bearer challenge. The model-assets release remains present after release cleanup.

## Issue #20: reviewed archival

The user's two obsolete acceptance fixtures were archived through authenticated MCP only:

* `/systems/acceptance/acceptance-asset-8fa11c5266.md`
* `/systems/acceptance/acceptance-ordinary-8fa11c5266.md`

The proposal contained two `kind: trash` changes. Its `archival_impact` report was inspected before approval: both targets had no inbound references or conflicts. The asset fixture had accepted skill version 1.0.0, SHA-256 `c74bb2b5a3692c5d1bb0db1ca9c8711b73125b8628c35cdd7cb667745f1f81ff`, explicitly marked for retention. The ordinary fixture had no assets. The report explained that inbound links would be left unchanged, assets retained for restore and Git history preserved.

* Proposal: `d4236db0-6218-4c36-82d1-434643ecbad6`
* Author/reviewer principal: `sandbox`, with curator access to both paths
* Base revision: `5224a6d50df9f068bec4bc5e016482f419bb79f6`
* Durable apply key: `issue-20-reviewed-archive-8fa11c5266-v053`
* Operation: `00cd6e1b-e2c9-428a-a841-53d7430cb900`
* Applied revision: `a303ad9d57189abe3f35d937f100714bbd926e95`

Approval and apply were separate MCP calls. Apply committed one transaction moving both Markdown files to `/trash/systems/acceptance/`. `operation_get` returned `committed`, `partial: false`, and `safe_to_retry: false`. Local regressions cover idempotent replay; the successful live mutation was not repeated.

After apply, the original acceptance inventory and search for `8fa11c5266` were empty. Trash inventory retained both original concept IDs and the accepted asset digest. Both path-scoped audits returned `ok: true`, no issues and no graph diagnostics. The active graph contained 184 concepts and neither fixture ID nor associated diagnostic. The Trash-inclusive graph contained 186 concepts including both archived fixtures.

A separate read-only Git check confirmed that the base revision remains an ancestor, both old-path blobs equal the new Trash blobs byte-for-byte, and the accepted ZIP is byte-identical before and after archival. The old paths are absent from the current tree. No historical commit or asset was purged.

This covers issue #20's archival alternative: supported MCP removal from active inventory, pre-mutation impact reporting, curator proposal review/apply, revision checks, reconciliation, retained history and exclusion from active audit/orphan diagnostics. The [Trash contract](../trash.md) documents scope limits, current-policy filtering, collisions and retained history.

## Live graph checks

A headless Chromium visit to the deployed 0.5.4 page reported no page errors. The normal graph loaded 184 nodes; Show Trash added the two fixtures and a Trash label. Semantic opacity defaulted to 60%; changing it to 40% changed semantic line materials without advancing the layout generation. Enabling the layer exposed 331 bounded semantic edges, with material opacity scaled by their similarity bands; disabling it removed all semantic edges.

The active graph supplied 174 shared-namespace, 178 shared-type and 368 shared-tag relationships. Namespace and type controls were enabled, and changing namespace strength reached the worker settings. This corpus has no source-reference provenance data, so the provenance slider correctly remained disabled rather than pretending to affect layout. Local tests exercise actual movement for every shared-force kind with the other forces disabled.

The screenshot and bounded browser result are stored as `graph/release-0.5.4-live.png` and `graph/release-0.5.4-browser.json`.

## Remaining operational limits

Final status reports 0.5.4, 184 active concepts, repository/index/embedding revision `a303ad9d57189abe3f35d937f100714bbd926e95`, no stale index, no proposal backlog, semantic search ready, and Needle loaded. The earlier embedding backlog has cleared.

The original skill fixture's published root/body parity differs from its later concept body; archival deliberately preserved both bytes rather than rewriting an obsolete fixture. Docker still reports `PidsLimit: null`, the existing Synology enforcement limitation. Neither condition is represented as repaired by this release. The visual debugger remains an unauthenticated trusted-network read surface; archival and purge remain authenticated MCP operations.
