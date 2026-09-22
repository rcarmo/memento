# Memento 1.0.7 qualification and rollback

Memento 1.0.7 passed canary and production semantic-cache checks, then was rolled back to 1.0.6 because its metrics collector counted legacy embeddings as `other`. The item-relative and persisted-cache implementation remains in the source for the corrective release.

## Build and evidence

- Source: `9b2e89649d1fe3f85521416d4bcef27153cb32aa`.
- [Exact-commit CI](https://github.com/rcarmo/memento/actions/runs/35743925903): passed.
- [Release](https://github.com/rcarmo/memento/actions/runs/35746605514): passed, including amd64 baseline-CPU and ARM64 image gates.
- OCI index: `sha256:07f9da3c16e43048498e6a4ffe8d6bdb6fdda7f8b8c3f4a1468b7e296dad086b`.
- Installed amd64 image config: `sha256:d20ef2f3e130763a5bd59307d34dba27b137e44d444e878b6dd63cf2b590caca`.

## Backup and canary

Backup `/volume1/docker/memento/config/backups/20260922T145918Z` was captured while production was stopped. Both SQLite integrity checks, Git fsck and all manifest SHA-256 checks passed after export. Repository revision was `19b64ec9ad1d52fc64a56153172dfbbf917611d2`.

| File | SHA-256 |
| --- | --- |
| control.sqlite | `da467681ccab2e791c3d6ad986531b045426f8ea505821e53160f62c6c622ef9` |
| derived.sqlite | `08dfab3f6338946a884860e9c54a4f036eb705d56597c15aa7642cedcae53bff` |
| repo.git.tar.gz | `7a6732f8dd0e7fcc1cb2106a8814f51413ebc8fa779e3a9eff04331415faf155` |

The NAS was slow: the backup orchestration timed out after backup success, before restart completed. Inspection found production stopped; an explicit start restored healthy 1.0.6. No backup or mutation was blindly repeated.

Canary state was a separate copy. Initial migration preserved five chunked items byte-for-byte and built ten pair scores. It labelled 252 visible legacy items and one missing item without false repository-relative `embedding_stale` findings. Explicitly refreshing `/work/skills/customer/catalog.md` yielded six ready items and 15 cached pairs. Exported SQLite confirmed six aggregate rows, 15 pair rows and 57 chunks, with integrity `ok`.

Unrelated create/patch/rename operations changed the repository revision without changing existing embedding metadata or scores. The disposable lifecycle completed through purge and empty-marker search. Restart preserved all six ready items and 15 pairs. Chromium navigation, diagnostic scoping and version ordering passed. The 16 stored unresolved explicit references were unchanged.

## Production and defect

Only the image digest changed. Production passed graph/API/browser checks and disposable create, patch, rename, restart/read, trash, restore and purge. The five existing chunked items and ten cached scores survived unrelated edits and restart. Snapshot edge revision fields advanced as expected; score and embedding provenance remained unchanged.

Final disposable lifecycle revision: `22b7497924acee09e217b63ef8c5a06d4e5a2964`; marker search returned no results. Security options, mounts, UID/GID, ports and resource limits matched the prior stack. No OOM or automatic restart was observed. Existing `refresh_on_startup: false` and progressive CPU/idle pacing were retained; full migration was explicitly queued after validation.

Three successful metrics collections took 13, 463 and 156 ms. They exposed the defect: ready=6, missing=1, legacy=0 and other=253, despite the 253 rows having status `legacy`. `readEmbeddingMetrics` initialized the legacy bucket but omitted it from its switch. The corrective test covers every known status, unknown-status fallback, and real SQLite-to-HTTP exposition.

The rollback stack request was interrupted locally. Subsequent inspection confirmed healthy production on the retained v1.0.6 index `sha256:0837a0559704d454f882c7df08b0d6311f683435ef61005014e1f94ab8b36699`. State was retained: no repository, chunk or cache tables were restored or deleted. Full migration stopped with the old process and needs an explicit request after the corrected deployment.

Raw evidence: `/workspace/tmp/memento-rollout-107/`. Conversion is not complete. The rollout helpers and state-copy directories remain available for the corrective release.
