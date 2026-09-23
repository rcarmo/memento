# Memento 1.0.9 production migration

Memento 1.0.9 completed production embedding migration and restart qualification on 23 September 2026. Progressive delay applies between entries; cancellation, interactive idle and CPU admission are checked between chunk batches. The Go embedding child lowers its scheduling priority before loading the model.

## Release and cutover

- Source: `bcd7b32fb88775af3b8109d16307b12a6d4f7e55`; pacing and priority implementation: `fa0537c`.
- [Exact-commit CI 35782473531](https://github.com/rcarmo/memento/actions/runs/35782473531): passed, including quality, browser and amd64/ARM64 model/container gates.
- [Release workflow 35785186168](https://github.com/rcarmo/memento/actions/runs/35785186168): passed, including both architectures, baseline CPU, provenance and SPDX SBOM.
- [Release v1.0.9](https://github.com/rcarmo/memento/releases/tag/v1.0.9); OCI index `sha256:03c1b2ebdb2c5146bd751fa34b44f02caf79d702e7c52503d2b6857e01dc24a7`.
- Portainer endpoint 18, stack 111: digest-only replacement; running container `281e5125713e13ddceac8c54f80c26e3b7c964bcd9b3fb7d642ca2558b1eb962` reports the release revision, healthy, zero automatic restarts and no OOM. Its state/config/secret/model mounts, UID/GID, port, read-only root, dropped capabilities and memory limits were preserved. A timed-out stack update was reconciled against the stored Compose and container rather than repeated.
- Live Docker process inspection reported `NI=15` for `/usr/local/bin/memento-embed`, while the daemon remained at `NI=0`.

## Migration and repairs

Before cutover, 46 items were ready, 213 legacy and one missing; the existing queue was in memory. After replacement, an explicit selected request enqueued the original 253 target IDs. This included already-ready IDs, so the process-level completion counter counts refresh calls and cannot alone measure newly converted rows. No state was restored or deleted. `refresh_on_startup` remains false.

The user authorised fixing imported broken links during reindexing. Thirteen concept bodies and new skill-pack versions corrected 16 pre-existing references using the verified canonical concept paths and HTTPS source permalink. Fresh authenticated body/asset hashes and paired `SKILL.md` parity were checked before each mutation. Uncertain applies were reconciled by their original idempotency keys; no indeterminate mutation was blindly repeated. The final repository/index revision is `761313cec2c6a9e60310ed11d3bb24cca300e323`. The graph overview reports zero `broken_link` diagnostics. A selected refresh for the 13 repaired concept IDs was queued once, with duplicate queue entries coalesced by the worker.

At 2026-09-23 08:18 UTC, a successful Prometheus scrape showed all 260 embedding rows ready; legacy, missing, stale, pending, error and other were zero. The worker was alive, idle and error-free after 255 completed refresh calls. Authenticated MCP reported semantic search ready at repository, index and embedding revision `761313cec2c6a9e60310ed11d3bb24cca300e323`. Some scrapes under NAS storage load returned `memento_metrics_collect_success 0`; only successful scrapes supplied row counts. `queued_paths` records the last submitted scope, not remaining queue depth.

## Restart and persistence

Before restart, the graph overview contained 258 visible concepts, 522 explicit edges, no broken edges and 1,500 semantic edges. The sorted semantic-edge digest was `a201c4e5139da02fefd232434b8f9b16e70a9d18e18f606ca7433cb3dae09cb6`. The worker was idle, Prometheus reported 260 ready rows, and authenticated MCP reported matching repository/index/embedding revisions. The container was healthy with zero automatic restarts and `OOMKilled=false`.

The one planned Portainer restart request timed out, as did initial read-only inspection. Reconciliation found that the container had exited with code 128 and Docker state error `context canceled`; it had not restarted and `OOMKilled` was false. An explicit start of the *same* container also timed out at the client. A subsequent inspection confirmed a new start at `2026-09-23T14:27:18Z`, with no duplicate start request. Health became healthy after startup grace; the service continued on the pinned v1.0.9 image with identical mounts, UID/GID, port, read-only root, dropped capabilities, security options and 512 MiB limit. Docker reported `RestartCount=0`, which does not count this explicit stop/start sequence.

After startup, a successful Prometheus scrape again showed 260 ready rows, all other embedding buckets zero, index ready and no stale index. Authenticated MCP reported semantic search ready and the same three revisions. The graph had the same 258 concepts, 522 explicit edges, zero broken edges, 1,500 semantic edges and exact semantic-edge digest. The new process reported zero completed embedding refreshes and zero index rebuilds. These read-only results verify retained chunk eligibility and persisted graph results across process restart; no direct post-migration SQLite `integrity_check` or table-count proof was obtained. A Docker archive response converted binary data to text, so it was not used as a SQLite copy or evidence.

A post-start Docker snapshot measured about 40 MiB RSS and 100 MiB cgroup memory usage against the 512 MiB limit. The v1.0.6 rollback image digest `sha256:0837a0559704d454f882c7df08b0d6311f683435ef61005014e1f94ab8b36699` remains installed, and the verified backup directory `/volume1/docker/memento/config/backups/20260922T145918Z` remains available. No rollback was needed.

Raw migration, graph, Prometheus, container inspection and restart reconciliation observations: `/workspace/tmp/memento-rollout-108/`. Prepared repairs, accepted-version hashes and per-proposal reconciliation: `/workspace/tmp/memento-link-regressions-20260922/`.
