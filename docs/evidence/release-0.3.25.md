# Release 0.3.25 validation

Release publication and the DiskStation deployment completed on 2026-08-26 UTC.

## Release identity

* Tag: `v0.3.25`
* Commit: `01e9f586fa6d1a6a8862f8a821adf149e912be56`
* Image: `ghcr.io/rcarmo/memento:0.3.25`
* Published multi-architecture manifest: `sha256:1c6341d977b0470caecc25a6aa9b819981ba44c8200835be7107d36115b4e37b`
* GitHub release: <https://github.com/rcarmo/memento/releases/tag/v0.3.25>

The exact release commit passed [ordinary CI run 32946691797](https://github.com/rcarmo/memento/actions/runs/32946691797): Python 3.12, 3.13 and 3.14, cached runtime-model preparation, container build and smoke tests all succeeded. [Release run 32946886722](https://github.com/rcarmo/memento/actions/runs/32946886722) then passed tag validation, the Python matrix, native amd64 and arm64 builds, the no-AVX Westmere smoke, multi-architecture publication, GitHub release publication and retention.

This release adds bounded, execute-only asset metadata inspection. Readers can inspect immutable sidecar, archive, manifest, file and concept-body parity metadata through `memory_execute` without receiving concept bodies or ZIP payload bytes.

## DiskStation deployment

Portainer endpoint 18, stack 111 was updated from the explicit `0.3.24` image tag to `0.3.25`. The Compose diff changed that tag and nothing else. The existing read-only configuration and secret mounts, writable `/volume1/docker/memento/state:/var/lib/memento` bind and semantic settings were retained; the config-update helper, runtime-model preparation helper and full embedding-refresh endpoint were not invoked. GitHub Actions did not perform the deployment.

Short Portainer pull requests timed out while Synology was extracting the image, leaving their server-side completion uncertain. Container Manager restarted during those retries and stopped the old container cleanly. Docker recovered without a stack replacement or state-volume change, and the old `0.3.24` container was started and checked before proceeding. A single long-lived pull then completed with the published digest before the stack update. Future runs should use one long pull and inspect Docker state after a client timeout rather than submitting overlapping retries.

The replacement container reported:

* container ID `d144781b431cb43990cab01bf21e0fcda4d23cb150de835ef03e3c65ea0bd7ac`;
* local image ID `sha256:79323a0b2a0e6668405ba2735ff0323d4dd4c8aedb454b0d759db827c7aa9ee6` and the published repository digest above;
* linux/amd64, running and healthy, with no restart, OOM kill, error or non-zero exit;
* UID/GID `65532:65532`, read-only root, all capabilities dropped, `no-new-privileges:true` and the default AppArmor profile;
* read-only config and secret mounts, plus the intended writable state and model volumes;
* a 512 MiB memory limit and 1 GiB memory-plus-swap limit.

Docker again reported `PidsLimit: null` despite `pids_limit: 128` in the Compose file. The Synology/Compose discrepancy remains unresolved and the requested limit must not be described as enforced.

## Live MCP, graph and asset checks

An unauthenticated MCP `initialize` request returned HTTP 401. Authenticated checks then passed:

* `memory_status` reported service version `0.3.25`, schema version 2, 155 visible concepts and no proposal backlog;
* repository, lexical-index and embedding revisions all matched `8de378e5a23d488cc9ec08e5744a3451a8828e51`, with `index_stale: false`;
* semantic search reported the `rust-gte` model, 384 dimensions and SQLite vector search ready, and a semantic search for `DiskStation Memento` returned the DiskStation instance, Memento project and benchmark as its first three results without warnings;
* the compact MCP surface omitted a direct `memory_asset_metadata` tool while retaining `asset_metadata` through `memory_execute`;
* a bounded 20-entry `/skills/` metadata page returned stable path ordering and a continuation cursor;
* exact metadata inspection for `/skills/extension-troubleshoot.md`, skill version `1.0.0`, returned publication, ZIP, manifest and `SKILL.md` file digests and sizes. The archive contained one 1,356-byte file, and `skill_root_matches_current_concept_body` was true; no concept body, ZIP bytes or base64 payload was returned;
* unauthenticated `/graph/api/v1/overview` returned HTTP 200 in direct mode with 155 nodes and 1,699 rendered relationships. All 59 skill nodes had tags;
* graph revisions matched the authenticated status, `stale` was false and no `embedding_missing` diagnostic was present.

## Preserved derived state

The earlier `0.3.24` snapshot at `d16f3f7c84a8bd0d5a7aee5e0b8a6d924b4e854f` contained 154 concepts. During recovery, before the `0.3.25` stack update, the live repository had advanced to 155 concepts and revision `8de378e5a23d488cc9ec08e5744a3451a8828e51`; repository, lexical-index and embedding revisions already matched at that immediate pre-update baseline.

The `0.3.25` replacement came up with the same three revisions and no missing embeddings. The progressive worker reported `running: false`, `pending: false`, zero queued paths and no last error. The image replacement therefore preserved the derived state it received and did not enqueue a full refresh.

## Remaining work

* Enforce or explain the missing production PIDs limit.
* Migrate principal grants before enabling protected read prefixes on the existing deployment.
* Run a clean-host restore drill.
* Measure semantic and Needle performance on real ARM64 hardware.
* Attach SBOM material to published releases.
* Add TLS before exposing any HTTP surface beyond the trusted LAN.
* Enable and verify the answer-model path only with a deliberately configured trusted provider slot.
