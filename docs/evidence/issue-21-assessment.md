# Issue #21 assessment

## Validity and impact

The issue is valid. Accepted-asset retrieval read the entire ZIP with `Path.read_bytes()` and returned its base64 representation, even for metadata inspection. A maximum-size accepted pack could produce roughly 67 MiB of base64. Proposal assets already had a bounded file API, but accepted assets did not. The existing `asset_metadata` operation reduced some inspection work but did not provide a complete manifest/file/range contract or resumable downloads.

This affects agent response budgets, interrupted transfers and inspection of generic assets as well as skills. The implementation keeps small default archive responses compatible while requiring range-aware clients for larger packs. It does not change accepted bytes, publication workflows, namespace policy, retention or Git history.

## Related fixes included

Accepted and proposed file reads share canonical manifest-path validation, byte-range rules, encoding and integrity checks. Accepted metadata and ZIP reads now reject parent symlinks as well as symlink targets. Byte responses verify their archive/file digest; resumed accepted reads pin both version and ZIP digest.

Inspection also found that `AssetGetArgs` existed but no `AssetGetOperation` participated in the execute union. The operation is now registered. Generic executor record bounding could silently truncate a manifest containing more than 50 entries; asset manifests and their projections retain the validated entries, subject to the explicit output-byte cap. Default execute chunks are reduced to leave room for base64, trace and return duplication.

## Scope and trade-offs

Inputs, outputs, limits, errors and compatibility are specified in [accepted-assets.md](../accepted-assets.md). Manifest view returns metadata without opening ZIP contents. File and archive views stream/hash data with bounded buffers; they do not load the whole archive into a Python byte string. Hashing the complete archive on every range read favours corruption detection over throughput. Persistent digest caches, external download URLs and storage-format migrations are deliberately out of scope.

The feature is read-only, uses trusted principal context and current original-namespace permissions for Trash, rejects missing/pruned versions, and locks against service-owned checkout replacement. Tests use temporary local repositories and malformed fixtures; no production calls or mutations are needed. Release and deployment are not part of this implementation request.

## Validation

Full `make check` passes, including Ruff, mypy, Python tests, browser asset/build checks and Rust formatting/lint/tests. Coverage is approximately 86%. A clean installed wheel imports the shared reader and executable asset operation. An independent delegate review timed out and is not counted as completed review. No live deployment or production mutation was performed.

## Regression coverage

Regression coverage includes small legacy archives, manifests with 55 entries, manifest reads that cannot open the ZIP, direct MCP and execute schemas/results, binary file and ZIP reconstruction, UTF-8 boundary splits, empty files/EOF, final partial chunks, stable-version resumption after a newer version appears, digest corruption, unauthorized paths, file traversal, duplicate/undeclared/symlink ZIP entries, symlink filesystem parents/targets, Trash reads, purge and actual version pruning. Shared helper tests verify corruption outside the requested slice is detected.
