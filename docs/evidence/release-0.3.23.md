# Release 0.3.23 validation

This record separates release validation, live DiskStation checks and work that was not exercised in production. It contains no bearer credentials or provider secrets.

## Release identity

* Tag: `v0.3.23`
* Commit: `587fefa64faa2dd59067bc200592787612645024`
* Image: `ghcr.io/rcarmo/memento:0.3.23`
* Published multi-architecture manifest: `sha256:0a15f653b323e07008edc8f7337207089a277031a0f6dfef1f3f260741ee00d8`

The release workflow passed its Python 3.12-3.14, Rust, wheel, container, amd64, arm64, Westmere and security gates before publication.

## DiskStation acceptance

Portainer stack 111 on endpoint 18 was updated once to the immutable release. On 2026-08-07 the replacement container was:

* container `c8fbf878dcca81ce53cd345ae532067da30580b5af4036ef9445044fcee8acb4`;
* running image `ghcr.io/rcarmo/memento:0.3.23` with local image ID `sha256:86907b6b5958057a82dd8e22c2fddb5b5bce68127080a68c7ab9a44640ee6681`;
* healthy after the configured five-minute startup grace, with the last three recorded health checks successful;
* running as `65532:65532`, with a read-only root filesystem, all capabilities dropped, `no-new-privileges:true`, init enabled, a 512 MiB memory limit and a 256 MiB reservation;
* not restarting, dead or OOM-killed, with exit code zero.

Startup logs contained the structured `serve_starting` event and the Streamable HTTP listener message, with no traceback or restart loop. One bounded stats snapshot showed 164,696,064 bytes of memory usage, including 126,996,480 bytes of cache and 37,699,584 bytes of RSS.

Docker inspection reported no effective PIDs limit even though `deploy/diskstation.compose.yaml` requests `pids_limit: 128`. This needs an operator check against the Synology Docker/Compose implementation and remains a production hardening gap.

## Service and contract checks

Authenticated `memory_status` reported service version `0.3.23`, 114 visible concepts, semantic search ready and matching repository, lexical-index and embedding revisions:

```text
84dae8e9f218ad39e7634f44a43bb824ea9b1a97
```

The following live checks passed:

* unauthenticated MCP requests returned HTTP 401;
* an ordinary plain hybrid query, `Memento: DiskStation?`, treated punctuation literally and returned `/instances/memento-diskstation.md` first;
* an explicit raw FTS5 query, `"Memento" AND "DiskStation"`, succeeded with `query_syntax="fts5"`;
* the public graph operation returned two outbound and three inbound edges for `/instances/memento-diskstation.md`;
* every returned public edge had exactly the documented typed fields: `concept_id`, `path`, `title`, `depth`, `direction`, `broken_link_count` and `orphan_flag`;
* a disposable reader could not discover `access_*` tools and direct invocation of an admin tool returned HTTP 403; all disposable verification principals were disabled, revoked and deleted after the checks;
* the unauthenticated trusted-LAN graph overview returned HTTP 200 in direct mode with 114 nodes and 1,200 rendered relationships;
* repository, index and embedding revisions matched in both MCP status and the graph overview.

The production compact surface deliberately does not expose `memory_answer`: `mcp.compact_answer_enabled` is false and no model-provider slots are configured. Live model generation and secret-intent abstention were therefore not invoked. The release test suite validates secret abstention before cache, retrieval, repository, graph or model access; production acceptance verifies the expected disabled surface rather than claiming a live answer-model result.

## Tagged-source corpus rerun

The preserved 23-concept, 15-query synthetic harness was rerun locally at the exact release commit with GTE-small ready. All five hard gates passed:

* zero namespace leakage across every method and budget;
* prompt-injection evidence remained explicitly untrusted;
* the filtered variant abstained on every unanswerable or secret case;
* the filtered variant never preferred stale evidence;
* semantic search was ready.

Compared with the pre-change `0.3.22` baseline at `d7d22ee2490323629c5536739530269ce3e4015e`:

| Measure | 0.3.22 baseline | 0.3.23 tagged source |
|---|---:|---:|
| Plain lexical top-5 mean recall | 0.0000 | 0.8333 |
| Plain lexical top-5 answer coverage | 0.1333 | 0.8667 |
| Hybrid top-5 mean recall | 0.8974 | 0.8974 |
| Hybrid top-5 answer coverage | 0.8667 | 0.8667 |
| Filtered top-10 answer coverage | 1.0000 | 1.0000 |
| Filtered top-10 secret/unanswerable abstention | 1.0000 | 1.0000 |
| Peak RSS | 332,920 KiB | 333,052 KiB |
| Query-phase RSS growth | 5,928 KiB | 6,408 KiB |
| Total disposable state | 731,594 bytes | 731,593 bytes |

Peak RSS increased by 132 KiB and query-phase growth by 480 KiB, both within the existing worker and container ceilings. The corpus is small and synthetic, does not measure final-reader F1 and uses `DerivedIndex.graph` directly for graph ablations. Public `memory_graph` serialization was validated separately by unit, MCP and live production checks.

## Residual work

* Enforce or explain the missing production PIDs limit.
* Run a clean-host restore drill for the selected deployment path.
* Measure semantic and Needle performance on real ARM64 hardware.
* Produce a repeatable production semantic-search throughput report.
* Attach SBOM material to published releases.
* Add TLS before exposing any HTTP surface beyond the trusted LAN.
* Enable and verify the answer-model path only when an operator deliberately configures a trusted provider slot.
