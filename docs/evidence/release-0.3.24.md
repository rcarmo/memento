# Release 0.3.24 validation

Release publication and deployment began on 2026-08-25. The final acceptance snapshot below was taken on 2026-08-26 UTC.

## Release identity

* Tag: `v0.3.24`
* Commit: `d6da9fb0d2159e44d1b7f3ea5cb7584d4e32d124`
* Image: `ghcr.io/rcarmo/memento:0.3.24`
* Published multi-architecture manifest: `sha256:f04004c89e915c5c6c35197bfbba7b3dbf7f23fcf42b664ef3800aab38a27c08`
* GitHub release: <https://github.com/rcarmo/memento/releases/tag/v0.3.24>

The exact release commit passed [ordinary CI run 32908404237](https://github.com/rcarmo/memento/actions/runs/32908404237): Python 3.12, 3.13 and 3.14, cached runtime-model preparation and the container job all succeeded. [Release run 32908612211](https://github.com/rcarmo/memento/actions/runs/32908612211) then passed tag validation, the same Python matrix, native amd64 and arm64 builds, the no-AVX Westmere smoke, multi-architecture publication, GitHub release publication and retention.

The release adds bounded namespace inventory, execute-only local-manifest comparison and optional protected read namespaces. It also fixes failed idempotent retries so they retain the persisted operation ID, correctly serialises status-only `deprecated` and `tombstone` patches, and serialises repository audit issues from slotted dataclasses. The release tests cover those mutation fixes; production acceptance did not create a status transition merely to exercise them again.

## DiskStation deployment

Portainer endpoint 18, stack 111 was updated with the explicit `0.3.24` tag after the documented one-shot config helper completed. GitHub Actions did not perform the deployment.

The replacement container reported:

* container ID `dba2c99f272acee39a19229f2d2dce201e096574b876b14c3a364c0514abc5e8`;
* local image ID `sha256:27f1c99ac8ff1d0fc58b2daf019f03c817a0cd0e4b8a7d50c38b49addb2ac5f7` and the published repository digest above;
* running and healthy, with no restart, OOM kill or non-zero exit;
* UID/GID `65532:65532`, read-only root, all capabilities dropped, `no-new-privileges:true` and the default AppArmor profile;
* read-only config and secret mounts, plus the intended writable state and model volumes;
* a 512 MiB memory limit and 1 GiB memory-plus-swap limit.

A resource snapshot at `2026-08-25T23:59:19Z`, while embedding refresh work was pending, showed 158,363,648 bytes of current memory use, 444,047,360 bytes maximum use and 27,021,312 bytes RSS. The container remained below its limit and healthy.

Docker still reported `PidsLimit: null` despite `pids_limit: 128` in the Compose profile. The Synology/Compose discrepancy therefore remains unresolved and the requested limit must not be described as enforced.

## Live MCP and graph checks

An unauthenticated MCP `initialize` request returned HTTP 401. Authenticated checks then passed:

* `memory_status` reported service version `0.3.24`, schema version 2, 154 visible concepts and no proposal backlog;
* shared repository and lexical-index revisions both matched `d16f3f7c84a8bd0d5a7aee5e0b8a6d924b4e854f`, with `index_stale: false`;
* the compact read surface exposed `memory_inventory` directly and kept `compare_manifest` available only through `memory_execute`;
* plain lexical search for `DiskStation Memento` and deliberate raw FTS5 search for `"DiskStation" AND Memento` both returned the expected Memento project, DiskStation instance and benchmark among the first results;
* `/instances/` inventory returned two stable path-ordered records with body SHA-256, byte counts and empty asset summaries;
* a two-row manifest comparison classified `/instances/memento-diskstation.md` as matching, a synthetic `/instances/release-smoke-missing.md` row as local-only and `/instances/flint.md` as Memento-only, with no differing row;
* a depth-one graph read for `/instances/memento-diskstation.md` returned two outbound and three inbound neighbours with no broken targets;
* unauthenticated `/graph/api/v1/overview` returned HTTP 200 in direct mode with 154 nodes and 501 rendered relationships: 135 canonical explicit edges and 366 derived shared-tag edges;
* the graph snapshot carried the same repository and index revisions and reported no lexical staleness.

The production compact answer surface remains disabled and no provider slots are configured. This acceptance therefore makes no claim about live answer generation or secret-intent abstention.

## Progressive embedding state

Lexical search, inventory, manifest comparison and explicit graph traversal were healthy, but semantic readiness was still degraded. The replacement initially queued all 154 concepts for the current derived embedding revision. A confirmed full refresh used the same low-priority progressive worker as automatic generation; it did not bypass startup, interactive-idle, sampled-CPU or pacing gates.

At `2026-08-26T00:02:40Z`, graph status reported 16 completed paths from the original 154-path queued scope, `pending: true`, `running: false`, `pause_reason: "cpu-sampling"`, `current_path: null` and no last error. MCP status consequently reported `embedding_revision: "partial"` and 138 of 154 embeddings not ready. A 30-minute observer reached its client timeout with work still pending, rather than a worker failure.

At `2026-08-26T03:02:05Z`, graph status reported all 154 paths complete, `pending: false`, `running: false`, `current_path: null` and no last error. Authenticated MCP status reported semantic readiness with the `rust-gte` model, 384 dimensions and SQLite vector search enabled. Repository, lexical-index and embedding revisions all matched `d16f3f7c84a8bd0d5a7aee5e0b8a6d924b4e854f`, with `index_stale: false`; an authenticated semantic search completed without warnings.

The mounted progressive state converged without another refresh or container restart. Semantic and hybrid ranking are ready for the current repository revision.

## Protected namespaces and remaining work

The release and configuration examples support `authorization.protected_read_prefixes`, but the existing DiskStation `config.json` did not yet set that option. Existing broad `/` readers therefore retained their previous visibility during this deployment. Enabling the example mask for `/work/`, `/personal/` and `/infrastructure/` requires a deliberate principal-policy migration and a separate live authorisation check; CI covers the feature contract, but this production run does not claim it as enabled.

Remaining operational work is:

* enforce or explain the missing production PIDs limit;
* migrate principal grants before enabling protected read prefixes on the existing deployment;
* run a clean-host restore drill;
* measure semantic and Needle performance on real ARM64 hardware;
* attach SBOM material to published releases;
* add TLS before exposing any HTTP surface beyond the trusted LAN;
* enable and verify the answer-model path only with a deliberately configured trusted provider slot.
