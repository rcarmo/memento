# Reading accepted assets

`memory_asset_get` supports archive, manifest and file views. These are read-only and available directly or as `asset_get` inside `memory_execute`. Every view resolves the concept under the caller's current read permissions and selects an accepted version; omitted `version` selects the highest stable version. An explicit Trash path is readable under the original namespace permissions. Purged or pruned versions are unavailable; retrieval never falls back to Git history or staged proposals.

## Choose a view

```json
{
  "id_or_path": "/projects/example.md",
  "asset_kind": "document",
  "view": "manifest"
}
```

`view: "manifest"` returns the resolved version, ZIP SHA-256 and byte count, canonical manifest SHA-256, file count, total uncompressed bytes and all manifest entries. It does not open the ZIP, extract files or include `zip_base64`. It checks stored metadata consistency and the ZIP's existence/type/size; it does not claim to have verified ZIP contents. The byte views verify actual contents.

```json
{
  "id_or_path": "/projects/example.md",
  "asset_kind": "document",
  "version": "1.0.0",
  "view": "file",
  "file_path": "templates/example.txt",
  "offset": 0,
  "limit": 65536,
  "expected_sha256": "<ZIP SHA-256 from manifest>"
}
```

`view: "file"` returns a `file` object containing the manifest-relative path, media type, full-file SHA-256, total size, offset, returned byte count, `next_offset`, chunk `content_sha256`, `encoding` and `content`. Text chunks use UTF-8 when independently decodable; binary data and chunks that split a UTF-8 code point use base64. Reassemble bytes according to each chunk's encoding, then decode the complete text. Only manifest-listed regular files can be read. Traversal, undeclared or duplicate entries, symlinks, special files and manifest/ZIP disagreements fail before bytes are returned.

```json
{
  "id_or_path": "/projects/example.md",
  "asset_kind": "document",
  "version": "1.0.0",
  "view": "archive",
  "offset": 0,
  "limit": 65536,
  "expected_sha256": "<ZIP SHA-256 from manifest>"
}
```

`view: "archive"` returns `zip_base64` containing at most the requested bytes, plus top-level range metadata and the resolved version/digest. Ranged responses omit manifest entries; request the manifest separately. The default view is archive. For compatibility, an omitted limit at offset zero returns the complete ZIP and manifest when the archive is at most 16 KiB. Larger archives default to a 64 KiB chunk. Existing clients must check `next_offset` rather than assuming every `zip_base64` is a complete ZIP.

## Resume without mixing versions

For every request after the first chunk, send the resolved `version` and `expected_sha256` with the returned `next_offset`. Nonzero offsets require both. If a newer version becomes latest, the explicit version still selects the original bytes. A digest mismatch, missing/pruned version or unauthorised path returns an error rather than substituting content. Continue until `next_offset` is null; verify the assembled ZIP against `zip_sha256`, or assembled file against `file.sha256`.

Offsets and lengths count bytes, not characters or base64 symbols. Negative offsets, limits outside 1--262,144 and offsets beyond EOF are rejected. Offset equal to EOF is valid and returns an empty chunk with no next offset. `truncated` means the response is not the whole object, including a final chunk starting at a nonzero offset; `next_offset` alone indicates whether more bytes follow. Final short chunks and empty files have no next offset.

`file_path` is required for file view and rejected for other views. Manifest view rejects ranges. `expected_sha256` is a lowercase 64-character hexadecimal ZIP digest, not a file or manifest digest.

## Bounds and integrity

`memory_status.limits.assets` advertises the current upload/archive, total uncompressed, per-file, file-count, default chunk, maximum chunk, inline-archive and metadata limits. Current limits are 50 MiB ZIP/uncompressed, 16 MiB per file, 512 files, 64 KiB default chunks, 256 KiB maximum chunks, 16 KiB inline archives and 2 MiB metadata.

`memory_execute` chooses a smaller default byte chunk when necessary for its response budget. Explicit requested limits retain their meaning; oversized plans return a bounded-output error. Manifest arrays are never silently sliced by the executor's generic record-count limit. Large manifests may exceed a small execute byte budget; use the direct tool or a sufficiently sized execute budget. Projection `limit` can deliberately select fewer entries.

Archive reads stream and verify the full ZIP while retaining only the requested slice. File reads verify the ZIP, inspect its entry directory and stream/hash the selected file before returning its slice. This bounds memory and prevents returning a valid-looking prefix of a corrupt archive, but each resumed request still reads the full ZIP for hashing. It is not a zero-I/O range server. The existing archive size ceiling bounds that cost, and a transaction lock prevents service-owned checkout replacement or pruning during a read.

Accepted and proposal file reads share range, manifest-entry and digest validation. `proposal_asset_get` keeps its existing schema and UTF-8/base64 file envelope; it still selects a proposal and staged asset ID, never an accepted version. No upload format, accepted ZIP bytes, database schema or retention policy changes are required.
