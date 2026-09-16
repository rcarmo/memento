# Memento 0.5.5 deployment and issue #21

## Release

* Pull request: <https://github.com/rcarmo/memento/pull/22> (merged)
* Release: <https://github.com/rcarmo/memento/releases/tag/v0.5.5>
* Commit: `dd5d1dd1faa89f0bc1a94bec4cc825c740f5d423`
* Published: 2026-09-16 16:35:10 UTC
* OCI index: `sha256:9afc3ad5a976d85b61c33ce95d92a61c4390a9264b36dc43c0928f29658cb9e8`
* CI: <https://github.com/rcarmo/memento/actions/runs/35121819252>
* Release workflow: <https://github.com/rcarmo/memento/actions/runs/35122099740>

Local validation passed 392 tests, approximately 86% coverage, Ruff, mypy, browser build/integrity and Rust checks. The wheel built and imported from a clean environment. Exact-commit CI and release checks passed, including the Python 3.12--3.14 matrix, container smoke tests, native amd64/arm64 publication and baseline CPU checks. The downloaded OCI index independently hashed to the published digest. The runtime model-assets release survived retention.

## DiskStation

Endpoint 18, stack 111 received a digest-only image change. The existing environment, state/configuration/secret binds, original explicitly mapped `/models` volume, ports, resource and security settings were compared before and after and remained unchanged. No configuration helper, model preparation, repository mutation or manual index/embedding rebuild was invoked.

* Container: `2a49403a17fd5242053e7a145b78cb5e2a542b058770ac898aeb6ca812de2412`
* Local amd64 image: `sha256:109d85442c91923e3740ae3d2804ad8b91f8f743a7138cf04955d874d5d83f0c`
* Started: 2026-09-16 16:43:57 UTC
* Healthy by: 2026-09-16 16:51:02 UTC
* Restart count: zero

Authenticated MCP reports version 0.5.5, schema version 2, 184 active concepts and two pending proposals, matching the pre-deployment counts. Repository, content-index and embedding revision remain `a303ad9d57189abe3f35d937f100714bbd926e95`. Semantic search is ready, Needle is loaded and the index is not stale. Unauthenticated MCP returned HTTP 401 with a Bearer challenge.

## Live bounded retrieval

All acceptance calls were read-only. Existing accepted assets were used; no test content was published or deleted.

The archived acceptance skill at `/trash/systems/acceptance/acceptance-asset-8fa11c5266.md`, version 1.0.0, returned a manifest-only response with two entries, ZIP size 360 bytes and no base64 payload. A file request through `memory_execute` read the first 16 bytes of `scripts/check.txt`; a subsequent direct call resumed at numeric offset 16 and returned the final 15 bytes with no next offset. Both calls pinned version and ZIP digest.

Two archive calls returned 200 and 160 bytes. Their decoded concatenation was a valid ZIP with SHA-256 `c74bb2b5a3692c5d1bb0db1ca9c8711b73125b8628c35cdd7cb667745f1f81ff`, matching the accepted metadata. The reconstructed 31-byte file matched the ZIP member and SHA-256 `167af2e4d46fe58b0d59be40abd639f5f9f4b06cf84c42a4db87ddccd4b63bf7`. ZIP CRC checks passed. A default call retained the complete small-archive ZIP/manifest response. Wrong expected digest and traversal file paths were rejected before returning bytes.

A larger accepted pack, `/skills/docx.md` version 1.0.0, returned its entire 61-entry manifest without its 157,348-byte ZIP. Its ZIP SHA-256 was `5f77d5f03735bd97aba174b4e0eca3e3608540de482d1fdac7d7bfd00bd697d9`. Execute-based reads returned a 64-byte slice of its 20,700-byte `SKILL.md` and the final archive range at offset 157,300: 48 bytes, `next_offset: null`. No whole-pack retrieval was needed for those inspections.

`memory_status.limits.assets` exposes archive/upload, uncompressed, per-file, file-count, chunk, inline and metadata ceilings. Local regressions additionally cover unauthorized namespaces, digest corruption outside requested slices, undeclared/duplicate/symlink ZIP entries, filesystem parent/target symlinks, final/empty/UTF-8 boundary chunks, version drift, actual pruning, purge, Trash permissions and complete manifests through execute.

## Compatibility and follow-up

Small default archives up to 16 KiB retain a full ZIP and manifest. Larger clients must follow `next_offset`, pin the resolved version and digest, and verify assembled bytes. Byte reads hash the full bounded archive on each call, trading I/O for integrity without retaining the archive in memory. [Accepted-asset documentation](../accepted-assets.md) describes the contract and execute output budgets.

A test plan using a saved reference as a numeric offset (`$first.file.next_offset`) failed before execution because typed argument validation precedes reference resolution. Literal offsets in subsequent requests work and were used for the successful resume checks. This existing executor limitation and its verbose union error response are tracked separately in [#23](https://github.com/rcarmo/memento/issues/23); it is not presented as supported by 0.5.5.

The final deployment retains the existing Synology `PidsLimit: null` discrepancy. No independent delegate review is claimed: that review attempt timed out during implementation.
