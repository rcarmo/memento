# Memento 1.0.5 production validation

Memento 1.0.5 is running on the DiskStation with the audited chunking and live metrics changes. Production mutation, restart and selected embedding checks passed on 21 September 2026.

## Build and deployment

- Source: `fedd0b4782006b60433a3791d2331f76c9301adc`.
- CI: <https://github.com/rcarmo/memento/actions/runs/35632853903>.
- Release: <https://github.com/rcarmo/memento/actions/runs/35634656533>.
- OCI index: `sha256:57299ee202db5389957d7a4bcb635953ed959ee285e3bdb867ff2c0ad780cd84`.
- amd64 image config: `sha256:5dbc83f79ad4b4e95ad6952522e26d70d86e93144031992fac54706a3633aa4a`.
- Target: Portainer endpoint 18, stack 111, production container `22934ad905e7`.
- Preserved port 18081, UID/GID 65532, state/config/secret mounts, external rollback model volume, read-only root, capability drop, no-new-privileges and 512 MiB limit. Synology reports that its kernel does not enforce the requested PID limit; no claim of enforcement is made.

## Verified rollback state

Backup `/volume1/docker/memento/config/backups/20260921T183310Z` was made while production was stopped, then exported for independent checksum, SQLite integrity and Git fsck checks. Repository revision was `130cb3e506f08ffb9bf8cc973949bfa02bf064b9`.

| File | SHA-256 |
| --- | --- |
| control.sqlite | `6f243ec314092cd143011250c3c822af386faf6f664a0ab4d45de06c1305d577` |
| derived.sqlite | `aa6809d6a981045fc488fce23ff1bdd703b560aa2364d34bdf4f6b9b1c15862e` |
| repo.git.tar.gz | `28eb99a0e1c38f953501ac0dc88440866953939507fc908918ce6d5b3957b778` |

The exact prior NAS 1.0.2 image is preserved under immutable amd64 manifest `sha256:07cf50633a25a087242905091ca91755ae6683a88e5e0439e9b0196cb8106ea1`, image config `sha256:22e4bb5ccca9836f3304201af0e840d6d0e9dd88cdd60fde80cc2cb1a1c37513`. The old registry index had been deleted by retention; the same installed image was exported and republished, not rebuilt.

## Live checks

The exact-digest canary passed authenticated create, patch, read, rename, trash, restore and purge against a backup-derived copy. Production then passed the same disposable lifecycle, including reading the patched/renamed document after restart. Slow mutations returned `indeterminate`; reconciliation with the original keys confirmed single committed operations. No uncertain mutation was blindly retried.

After purge, lexical search found no validation marker and the visible count returned to 257. Final repository and index revision: `ad74840c18ceebcc27b1e1ddc51288b9175df6c2`. Git history intentionally retains the disposable commits.

A selected refresh of `/skills/documents/bento-slides.md` cleared its inherited input-length error. Read-only snapshot inspection confirmed 10 persisted chunks, each at most 4096 characters, with 1536-byte FP32 vectors (384 dimensions). The database had 259 ready embeddings and zero pending/stale/error/missing rows. The graph exposes 257 non-trash visible concepts. Semantic search ranked the selected document first at cosine 0.9151. A subsequent restart preserved readiness, revision equality and the document's successful embedding timestamp.

## Metrics and resources

`http://192.168.1.250:18081/metrics` serves Prometheus exposition. This is an exposed scrape target, not a claim that an external Prometheus/Grafana server was configured.

During cold/storage-heavy periods, some collections exceeded the two-second SQL context and correctly reported `memento_metrics_collect_success 0`; filesystem/kernel stalls can extend observed wall time. Four later minute-spaced samples succeeded at 1.6965 s, 0.2060 s, 0.0094 s and 0.0081 s. Do not interpret HTTP 200 alone as a successful collection.

The inspected daemon RSS during paced embedding was 23,572 KiB; cgroup RSS was 13,918,208 bytes with 178,573,312 bytes of cache, under the 536,870,912-byte limit. No OOM or automatic restart was observed. Explicit operator restarts were verified from start/finish timestamps. After refresh, derived SQLite was 8,978,432 bytes with zero WAL bytes; control SQLite was 69,644,288 bytes with an 82,432-byte WAL.

Evidence and scripts are retained locally under `/workspace/tmp/memento-rollout-105/`. Subsequent diagnostics/UI issues are tracked separately in [the graph audit](diagnostics-ui-audit-2026-09-21.md).
