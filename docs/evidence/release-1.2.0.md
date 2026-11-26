# Memento 1.2.0 qualification

The candidate adds opt-in [MCP file transfers](../mcp-files.md) on the `2026-07-28` Streamable HTTP revision. It leaves the legacy MCP versions and existing asset tools in place. The extension stays off unless configured.

## Compatibility

A separate HTTP client compared the real 1.1.1 and 1.2.0 daemons against the same disposable state, enabling the extension on the candidate. MCP `2025-03-26` and `2024-11-05` negotiated their original versions, created sessions, returned identical tool metadata, answered ping and closed their sessions. The draft client staged and confirmed a ZIP, attached it through proposal approval, and downloaded the accepted archive with the declared size and SHA-256. The 1.1.1 binary then reopened the same state using the original config; its repository revision, asset manifest and legacy discovery matched. This was an independent wire client for the pinned draft, not a third-party SDK test.

The audit closed a staging lifetime gap: a second logical call cannot bind to another call's ready bytes. Those bytes can be consumed or expire before the second call confirms. A replay of the original continuation still returns the same receipt. An additive SQLite table holds the draft state; it need not be removed for downgrade. The older parser requires the original configuration without `mcp.experimental_files`.

## Local gates

Go 1.26.11 on Linux/amd64: `make audit` and `make release-check` passed, including root/nested 100% statement coverage, race and fuzz checks, the no-AVX SIMD contract, cross-builds, allocation budgets and release-archive checks. The vulnerability gate required `golang.org/x/net` 0.57.0 to address GO-2026-5026; the rerun reported no reachable vulnerabilities. Staticcheck and govulncheck pins now understand the current Go export data. The real GTE/Needle model checks and model allocation budgets passed, and the full Needle corpus passed in 373 seconds. The amd64 container passed its runtime contract at 512 MiB (19.7 MiB idle container memory, 33.8 MiB daemon RSS); the transient connection reset while polling did not recur after startup.

CPU, `alloc_space` and `alloc_objects` were reviewed in the profiled suites, including worker and fuzz processes. The previous [file-transfer measurements](mcp-files-2026-11-22.md) cover the material new-path allocation fix. On this run the 8 MiB upload's buffer was bounded, and archive streaming no longer repeats the accepted-path lookup within the repository lock. The corpus CPU remains concentrated in the existing attention and vector kernels (64% and 23% flat in `attendInto` and `axpyRowsAVX2`). This release does not change model kernels. Some short package/process runs had no CPU samples; this is a sampling limitation, not CPU evidence for those runs. CI reports analysed hotspots in the job log and no longer uploads raw profiles, test binaries or copied sources.

Publication and NAS activation require the release workflow and registry checks. This local qualification did not change production.
