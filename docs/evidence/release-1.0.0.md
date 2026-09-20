# Release 1.0.0

This record separates source/release validation from the live DiskStation replacement performed on 2026-09-20. No canonical concept, proposal, accepted asset, managed principal or bearer credential was created or changed during deployment checks.

## Published release

Tag `v1.0.0` points to `d99768d68717ad9cc80229a63648da2a99ebe8ad`. [Release run 35511063305](https://github.com/rcarmo/memento/actions/runs/35511063305) passed runtime validation, native amd64/ARM64 image builds, the Westmere no-AVX smoke, multi-architecture publication, provenance attestation, SPDX SBOM generation and release retention.

The published OCI index is:

```text
sha256:132c782718b9a4a5b041a558d2fa747dab7564ba1bcb65d5230c5a64b386ce5c
```

Its native manifests are `sha256:297a8d0cbc5af8f02d2ddac375d4f95df4b3c9527428b52238edf4ea1db32e4d` for linux/amd64 and `sha256:5b9cbb5992cb1a92f382dc988a071b0beb9475b00cf68171399b3d227072efe5` for linux/arm64. The two additional unknown-platform manifests are attestations. GitHub's SLSA statement binds the index to the release workflow, tag and commit.

The attached `memento-v1.0.0.spdx.json` is 802,901 bytes with SHA-256 `5d8af26aae8a412325e299ca25b6ad44dbcd9c8513cb64e7b1403c1e9d077998`; it parses as SPDX JSON and contains package records.

## Deployment backup

Before stack replacement, the `0.5.9` container was stopped and its state was backed up to:

```text
/volume1/docker/memento/config/backups/20260920T144107Z
```

The destination is outside `/volume1/docker/memento/state`, which is the replaceable runtime root. A second no-network, read-only helper verified every file against `manifest.json`:

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| `control.sqlite` | 69,644,288 | `cf1dcfa133cfc522b8126dabc52e2a3feb95dff77446ff3b603ef255650246a6` |
| `derived.sqlite` | 8,511,488 | `760c95c216c1e400b9e2d6490096f15061ec8fea2e4c23393328e4829201fb15` |
| `repo.git.tar.gz` | 39,709,737 | `2113340c19ff2c05592f12de58cc3fa0ad2beb1c88e4fccf8dd08a5b1db5f72b` |

The backup records repository revision `130cb3e506f08ffb9bf8cc973949bfa02bf064b9`. Completed one-shot containers were removed after verification.

## Live replacement

Portainer endpoint `18`, stack `111`, was updated to the immutable v1.0.0 index. The stack preserves port 18081, UID/GID 65532, the read-only root, `/tmp` tmpfs, the 512 MiB limit, 256 MiB reservation, capability drop, no-new-privileges, state/config/secret bind mounts and the old external `/models` volume. Keeping that unused volume and the old `0.5.9` image digest makes an image-and-command rollback immediate.

The runtime command now calls `/usr/local/bin/memento-go` directly and the healthcheck uses its native `healthcheck` subcommand; the previous shell, Python entrypoint and Python socket probe are gone. The stack continues to request `pids_limit: 128`, but Docker inspection reports `PidsLimit: null`, preserving the known Synology/Compose discrepancy rather than claiming enforcement.

The running image has OCI version `v1.0.0`, revision `d99768d68717ad9cc80229a63648da2a99ebe8ad`, image ID `sha256:2a23522fa15d0eeb3dd1078719889c37f6ab923bf12854ad55261cf443ae90c9` and the published index digest above. The retained rollback image is `ghcr.io/rcarmo/memento@sha256:bebc0a3eaf935a5b4f07c3e060fd8e22a11dacff90cd55532ec04306c30e81bc`.

## Live checks

The original pre-deployment status reported service `0.5.9`, 257 visible concepts, proposal backlog 52 and matching repository/index revision `130cb3e506f08ffb9bf8cc973949bfa02bf064b9`.

The first v1 start and a subsequent explicit restart passed native healthchecks. After restart:

* authenticated status reports service `1.0.0`, schema 2, the same 257 visible concepts and proposal backlog 52;
* repository, index and embedding revisions all equal `130cb3e506f08ffb9bf8cc973949bfa02bf064b9`, with no stale index;
* semantic search is ready at 384 dimensions, uses the retained `rust-gte` model identity and reports no SQLite vector extension;
* Needle is enabled and loaded as `go-mmap-subprocess` from the release-generated `.nfp32` sidecar;
* a real `memory_route` request produced a validated `status_field` action and returned readiness data;
* Docker top captured the short-lived `memento-needle-go` process during that request, then showed only init and `memento-go` ten seconds later;
* `/graph/api/v1/overview` and `/graph` return HTTP 200; unauthenticated `/mcp` returns 401;
* the graph reports direct mode, 257 nodes and repository/index/embedding revision agreement;
* managed principal records, roles, read/write prefixes and enabled/revoked state are readable after migration;
* mounts still point to the original config, secret, state and preserved model volume.

After model activity and restart, daemon RSS from Docker top was 42,568 KiB. Docker cgroup statistics reported 29,130,752 bytes RSS and 255,881,216 bytes total usage, mostly file cache, with a 457,293,824-byte recorded peak under the 536,870,912-byte limit. The worker was absent from the final process list.

## Rollback readiness

Rollback requires stopping the Go container, restoring stack `111` to the retained `0.5.9` digest and its shell/Python command/healthcheck, then checking authenticated status and revision equality. If state recovery is necessary, restore from the verified timestamped backup above while the service is stopped. The old image and external model volume remain on the DiskStation; no rollback was required during this deployment.
