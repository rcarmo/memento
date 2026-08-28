# Release 0.3.26 validation

Release publication and the DiskStation deployment completed on 2026-08-28 UTC.

## Release identity

* Tag: `v0.3.26`
* Commit: `3387e7091bb63ec4c5282568e4623f304212f16f`
* uMCP dependency: `v0.2.2` at `9c89a708d14ae804e32aa65de10af7c02922617d`
* Image: `ghcr.io/rcarmo/memento:0.3.26`
* Published multi-architecture manifest: `sha256:1187a06938a845f70d0449419900ba00daa25feeee29f2b7bebf76e96dd9174e`
* GitHub release: <https://github.com/rcarmo/memento/releases/tag/v0.3.26>

The exact release commit passed [ordinary CI run 33217803453](https://github.com/rcarmo/memento/actions/runs/33217803453): cached runtime-model preparation, Python 3.12, 3.13 and 3.14 checks, wheel build and installation, container build and smoke tests all succeeded. [Release run 33217940219](https://github.com/rcarmo/memento/actions/runs/33217940219) then passed tag validation, the Python matrix, native amd64 and arm64 builds, the no-AVX Westmere smoke, multi-architecture publication, GitHub release publication and retention.

A separate clean virtual environment installed `memento-0.3.26-py3-none-any.whl[mcp]`. `pip check` found no broken requirements, and imports resolved Memento `0.3.26`, uMCP `0.2.2` and `aioumcp` from that environment.

This release moves Memento to uMCP's persistent Streamable HTTP transport. Sequential POST requests can share one HTTP/1.1 connection and MCP session, while authenticated GET requests carry SSE notifications for that same principal-bound session. Memento now advertises its own name and version and only the tools, resources and logging capabilities it supports.

## DiskStation deployment

Portainer endpoint 18, stack 111 was updated from the explicit `0.3.25` image tag to `0.3.26`. The Compose diff changed that tag and nothing else. The existing read-only configuration and secret mounts, writable `/volume1/docker/memento/state:/var/lib/memento` bind, semantic settings and runtime-model volume were retained. No config helper, runtime-model preparation, index rebuild or embedding refresh was invoked.

The image pull completed through one long Portainer request. The subsequent stack-update request exceeded its 30-second client timeout, but Portainer stored the new Compose file and created the replacement container from the pulled image. The replacement reported:

* container ID `d3df25a9325d222048eb24338fd6bc6319f3fe131ddf266491bdaac28e442965`;
* local image ID `sha256:5dd3590e0d57218aaa7034e4605a42371d4250c6b9d30a968d6725319053f4a1` and the published repository digest above;
* OCI version `v0.3.26` and revision `3387e7091bb63ec4c5282568e4623f304212f16f`;
* linux/amd64, running and healthy, with zero restarts, zero healthcheck failures, no OOM kill and no container error;
* UID/GID `65532:65532`, read-only root, all capabilities dropped, `no-new-privileges:true` and the default AppArmor profile;
* the original read-only config and secret mounts, plus the intended writable state and model volumes.

Docker again reported `PidsLimit: null` despite `pids_limit: 128` in the Compose file. The Synology/Compose discrepancy remains unresolved and the requested limit must not be described as enforced.

## Live Streamable HTTP checks

An authenticated `initialize` returned `memento` version `0.3.26` with the exact advertised capability object:

```json
{
  "logging": {},
  "resources": {
    "listChanged": true,
    "subscribe": true
  },
  "tools": {}
}
```

The compact reader surface exposed 10 direct tools. `memory_status` reported service version `0.3.26`, 77 concepts visible to `shared-reader`, matching repository and index revision `341132b9d3adbc5c9116e0670d33535d4f3f9149`, and `index_stale: false`.

Transport checks then passed:

* 100 sequential `tools/list` requests completed on one TCP connection and one MCP session with zero unexpected drops;
* a `memory://status` subscription received `notifications/resources/updated` over SSE after a read-only `memory_execute` status operation;
* the MCP session resumed on a replacement POST connection after the original POST connection closed;
* an abruptly reset SSE connection was accepted again after the old stream's keepalive cleanup completed. The client saw 45 one-second HTTP 409 retries before the replacement GET succeeded;
* deleting the MCP session closed its active SSE stream, and a subsequent GET for the deleted session returned HTTP 404.

The reconnect delay reflects disconnect detection through the default periodic SSE keepalive; no MCP request or notification was dropped. Container logs after the checks contained the expected startup records and no exception, disconnect error or restart.

## Preserved derived state

A read-only database check immediately before the image replacement found 172 concepts and 172 ready embeddings. Repository, lexical-index and semantic-embedding revisions all matched `341132b9d3adbc5c9116e0670d33535d4f3f9149`.

The same read-only check after deployment found 172 concepts, 172 ready embeddings and the same three revisions. The image replacement therefore preserved the repository and derived state it received without scheduling or forcing a refresh.

## Remaining work

* Enforce or explain the missing production PIDs limit.
* Decide whether production should use a shorter SSE keepalive to reduce abrupt-disconnect replacement latency.
* Migrate principal grants before enabling protected read prefixes on the existing deployment.
* Run a clean-host restore drill.
* Measure semantic and Needle performance on real ARM64 hardware.
* Attach SBOM material to published releases.
* Add TLS before exposing any HTTP surface beyond the trusted LAN.
