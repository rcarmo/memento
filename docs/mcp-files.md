## Experimental MCP file transfers

Memento can stage ZIP asset packs and download accepted ZIP versions through the experimental `io.modelcontextprotocol/files` extension. Enable it with `mcp.experimental_files: true`; the default is false. The adapter is new in 1.2.0 and follows the [pinned experimental draft][draft].

The server adapter requires a client that implements the draft. It adds no file picker, client transfer helper or gateway relay. Existing staging tools, `memory_asset_get` and proposal workflows keep their signatures and behaviour.

### Enable and negotiate

Merge this field into the existing configuration:

```json
{
  "mcp": {
    "experimental_files": true
  }
}
```

Keep `mcp.max_request_bytes` at least 52,429,824 bytes (50 MiB plus header allowance); the default is 72 MiB. A command-line `--max-request-bytes` override must also allow the intended pack size. The existing ZIP limits still apply: 50 MiB compressed and uncompressed, 16 MiB per file and 512 entries.

Use stateless Streamable HTTP POST requests with `MCP-Protocol-Version: 2026-07-28`, the matching `params._meta["io.modelcontextprotocol/protocolVersion"]`, and `Mcp-Method` matching the JSON-RPC method. For `tools/call`, also send `Mcp-Name` matching `params.name`. No `initialize` or session is needed. `server/discover` advertises tools and the files extension with `upload: {}` and `maxInlineBytes: 0`. A read-only tool surface omits upload support. Older revision negotiation keeps its existing defaults; file tools are unavailable over stdio and legacy SSE.

Declare capabilities in each relevant request, including `tools/list`, and keep the settings constant through an upload's retries:

```json
{
  "_meta": {
    "io.modelcontextprotocol/protocolVersion": "2026-07-28",
    "io.modelcontextprotocol/clientCapabilities": {
      "extensions": {
        "io.modelcontextprotocol/files": {
          "upload": {},
          "download": {}
        }
      }
    }
  }
}
```

Discovery lists `memory_asset_upload` for clients declaring uploads and `memory_asset_download` for clients declaring downloads. Unknown extension revisions disable these tools. These calls are separate from `memory_execute`.

### Stage an asset pack

`memory_asset_upload` accepts `asset_kind`, `version`, `idempotency_key` and one `file` object marked with `x-mcp-file`. The object requires `id`, `mimeType: "application/zip"`, positive integer `size`, and `digest: {"algorithm":"sha-256","value":"<base64url SHA-256, without padding>"}`. `name` is optional metadata; it is never a server filesystem path. Inline `data` and empty files are rejected.

The authenticated principal needs the proposer role. The initial call checks metadata and transfer quotas, then returns `resultType: "upload_required"`, one upload target and opaque `requestState`. The target uses POST, raw ZIP bytes, a path under the current MCP address, `credentials: "include"` and an additional `X-Memento-File-Token` header. Both reference HTTP transports check the bearer, token, media type and exact `Content-Length` before reading the body or sending `100 Continue`. The transfer uses the configured MCP endpoint, including custom paths.

After a successful transfer, repeat `tools/call` with the same tool arguments and put `requestState` in the request parameters, alongside `name` and `arguments`. Do not place it inside `arguments`. The retry checks the actual stored length and the SHA-256 receipt computed by the validated, immutable staging write, then returns its `staged_asset_id` and manifest. The normal proposal/review/apply workflow attaches the pack; upload never edits the accepted repository.

Continuation records live in shared control SQLite for one hour. They bind the principal, tool name, all accepted arguments, recognised capability settings and transfer target. JSON object order and equivalent integer spellings do not change the binding; absent and empty optional fields remain distinct. The staging write and transfer binding commit together. Use a new idempotency key for each new logical call. Only the continuation that staged a pack can repeat its upload against those bytes. Retries of a completed call still return its original receipt after a proposal consumes the staged bytes; that receipt describes the completed staging operation, not current staging status. A new adapter instance can confirm an existing call without an MCP session or process-local signing key.

There can be 64 live transfer records, four unconfirmed uploads per principal and 100 MiB of reserved unconfirmed upload bytes. Expired records are pruned on issuance. Staged packs retain the existing 24-hour staging expiry. The server buffers only an admitted upload's declared bytes and uses the existing ZIP validator. A transfer that finds the slot busy returns HTTP `429` without reading the body.

### Download an accepted version

`memory_asset_download` accepts `id_or_path`, `asset_kind` and an optional `version`. It requires reader access and returns a completed result describing the asset. Its `_meta["io.modelcontextprotocol/files"].downloads` contains the target, size and base64url SHA-256 digest. Targets and file bytes are absent from model-visible `content` and `structuredContent`.

The target uses GET under the MCP address, the caller's MCP credential and `X-Memento-File-Token`. Memento reauthenticates the principal, resolves current read permissions, pins the accepted version and digest, and verifies the complete archive before sending headers. An open descriptor pins that file for a bounded streamed response. A short or failed stream is aborted. Range downloads and individual-file downloads use the existing retrieval API instead.

One upload or download slot is active per process. Download copy buffers are bounded at 32 KiB and cleared before reuse. No archive copy is retained for a download target.

### Client and operator responsibilities

Clients must implement approval, target validation, credential restrictions, redirect refusal and download integrity checks from the draft. Memento emits only same-address targets and never fetches a client-supplied URL. It reauthenticates target requests and rejects a principal different from the one bound in the record. Transfer secrets and inline data are redacted by uMCP logging.

Use HTTPS, loopback, or an explicit client operator allowlist for a trusted-LAN HTTP MCP address. A reverse proxy must route the MCP endpoint and its child transfer paths to the same server. Gateways need their own extension support; passing these results through a generic executor is insufficient.

Modern HTTP metadata mismatches return HTTP `400` with JSON-RPC `-32020`; a matching but unsupported revision returns `-32022`. Unsupported modern methods return HTTP `404` with `-32601`. Missing upload/download capabilities return JSON-RPC `-32021`. Invalid, expired or differently bound continuation state returns `-32602`; metadata, storage or integrity failures return completed tool errors. Clients follow the draft's unknown-outcome rules and must not automatically restart a failed logical call.

Enabling the extension preserves the existing tool catalogue and session behaviour for MCP `2025-03-26` and `2024-11-05`. To roll back to 1.1.1, restore the original config or remove `mcp.experimental_files` entirely, even if it is `false`; the older parser rejects unknown keys. The new control table is additive. A disposable 1.1.1 -> 1.2.0 -> 1.1.1 test preserved the repository revision and an accepted ZIP added through the extension. Finish or abandon in-flight draft transfers before downgrading.

[Development measurements](evidence/mcp-files-2026-11-22.md) cover CPU and allocation profiling. The draft can change before standardisation.

[draft]: https://github.com/modelcontextprotocol/experimental-ext-file-uploads/blob/8a859b8f268926fe4beb84d78565e7cc6f9fa617/specification/draft/file-uploads.md
