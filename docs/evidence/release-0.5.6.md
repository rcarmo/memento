# Memento 0.5.6 deployment and issue #23

Memento 0.5.6 is running on DiskStation. A single execute plan now reads two file chunks using saved offset, version and ZIP digest values. Invalid resolved values produce short operation/field errors, and direct asset calls still require concrete offsets.

## Release

* Pull request: <https://github.com/rcarmo/memento/pull/24>
* Release: <https://github.com/rcarmo/memento/releases/tag/v0.5.6>
* Commit: `e211edfeec1109bd812b3da69b07f0295b6a9fbb`
* Published: 2026-09-16 19:22:11 UTC
* OCI index: `sha256:3c63f175bffa55fee02377188c5ccd15901c15eebd38d03770110dbff8080d0e`
* CI: <https://github.com/rcarmo/memento/actions/runs/35139175772>
* Release workflow: <https://github.com/rcarmo/memento/actions/runs/35139464412>

The final local release gate passed 416 tests, Ruff and mypy across 103 source files. Coverage measured approximately 86% on the preceding 415-test run. The wheel built and imported from a clean environment. Exact-commit CI passed the Python 3.12--3.14 matrix and container/runtime-model checks. The downloaded OCI index hashed to the published digest, and the installed image labels identify the release commit. The `model-assets-v1` release survived retention.

## DiskStation

Endpoint 18, stack 111 received only the image-digest change. Before/after comparisons confirmed identical stack and container environments, mounts, host resource/security settings and the original explicitly mapped `/models` volume. Mounts were compared by destination rather than array order. No configuration helper, manual repository mutation, model preparation or manual index/embedding rebuild was run.

* Container: `6ba1f008b2c30f445bc2138b68860cc23ae489d8e2a15a4b7a20383244ffd28b`
* Local amd64 image: `sha256:e89eeecd9ce5747f28a0d1663a8515bb5342e7599e6aa1af8c4f36c6addc5963`
* Started: 2026-09-16 19:46:19 UTC
* Healthy by: 2026-09-16 20:15:54 UTC
* Restart count: zero

The deployment client was interrupted after Portainer accepted the update. Inspection confirmed the new stack and container, so the update was not repeated. Startup took nearly 30 minutes and temporarily exceeded the health-check grace period. Read-only process inspection showed disk sleep in `wait_for_commit`, `wait_on_page_bit` and `btrfs_log_inode_parent`; Portainer requests also intermittently timed out. These observations suggest storage latency, but do not identify its underlying cause. The normal startup recovery rebuild progressed from 161 to 183 concepts while all 186 stored embeddings remained ready. The process was left running and recovered without intervention.

At the post-startup verification, authenticated MCP reported version 0.5.6, schema 2, 184 active concepts and two submitted proposals. The Trash inventory contains the same two archived acceptance fixtures, for 186 concepts including Trash. Repository, content-index and embedding revision remain `a303ad9d57189abe3f35d937f100714bbd926e95`. Semantic search is ready, Needle is loaded and the index is not stale. Unauthenticated MCP returned HTTP 401 with a Bearer challenge.

The deployment preserved submitted proposals `3d2e02c8-f258-47ed-a478-839af36ed241` and `8ad8eb9b-7985-4380-8104-48950cd7411c`, both unreviewed and unapplied at that check. A later check at approximately 20:26 UTC found a zero backlog: both retained records were now `rejected`, with `reviewed_by: flint` and review comments requesting complete versioned skill assets. Both applied-operation/revision fields remained null, and the repository revision was unchanged. This concurrent curator review was separate from the deployment and read-only verification. No acceptance content was created, archived, restored or purged.

## Live typed references

All live acceptance operations were read-only. The existing accepted skill at `/trash/systems/acceptance/acceptance-asset-8fa11c5266.md` supplied `scripts/check.txt` from version 1.0.0. The first operation returned 16 bytes; the second used these references:

```json
{
  "id_or_path": "$first.concept_path",
  "asset_kind": "$first.asset_kind",
  "version": "$first.version",
  "expected_sha256": "$first.zip_sha256",
  "view": "file",
  "file_path": "$first.file.path",
  "offset": "$first.file.next_offset",
  "limit": 16
}
```

Both operations succeeded. The second returned the final 15 bytes at offset 16 with `next_offset: null`. Their concatenation was 31 bytes and hashed to `167af2e4d46fe58b0d59be40abd639f5f9f4b06cf84c42a4db87ddccd4b63bf7`, matching the file metadata. Both calls selected ZIP digest `c74bb2b5a3692c5d1bb0db1ca9c8711b73125b8628c35cdd7cb667745f1f81ff`.

Sequential negative checks returned bounded `validation_error` messages:

| Input | Result |
| --- | --- |
| Saved null EOF offset | `operation 2 (asset_get): args.offset: Input should be a valid integer` |
| Saved service-version string as offset | Same integer validation error |
| Saved `index_stale: false` as offset | Same integer validation error; no boolean coercion |
| `$missing.file.next_offset` | `operation 1 (asset_get): unknown reference: $missing.file.next_offset` |
| Literal offset `-1` | `operation 1 (asset_get): args.offset: Input should be greater than or equal to 0` |
| Saved 16 MiB limit | `operation 2 (asset_get): args.limit: Input should be less than or equal to 262144` |
| Reference string passed to direct `memory_asset_get` | `asset offset must be a nonnegative integer` |

No message expanded into the old 261-branch union dump. An initial parallel batch hit the existing single-execute admission guard; rejected read-only checks were rerun sequentially after the active read completed.

Local regressions additionally cover negative resolved values, malformed and out-of-range references, nested arrays/booleans, namespace authorisation and preservation of a completed write's trace/reconciliation ID when a later reference fails. Production writes were not repeated to demonstrate those paths. The live profile has broad namespace access, so restricted-namespace rejection is supported by the local tests rather than a new production principal.

## Remaining operational notes

The existing Synology `PidsLimit: null` discrepancy is unchanged. The prolonged storage-bound startup warrants monitoring at the next deployment; this release changes executor validation, not startup or storage handling. No independent delegate approval is claimed: its review attempt timed out. The optional standalone JSON Schema checker was unavailable locally; it is not included among the successful checks.

The issue tracker was rechecked after live verification: #23 was the only open issue. Its typed-reference and compact-error requirements are covered by the release and tests above.
