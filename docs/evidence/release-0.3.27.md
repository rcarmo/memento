# Release 0.3.27 validation

Release publication and the DiskStation deployment completed on 2026-08-29 UTC.

## Release identity

* Tag: `v0.3.27`
* Commit: `d77aaac50ad4ef9a75cf77d016f4ed8d1846a6f3`
* uMCP dependency: `v0.2.2` at `9c89a708d14ae804e32aa65de10af7c02922617d`
* Image: `ghcr.io/rcarmo/memento:0.3.27`
* Published multi-architecture manifest: `sha256:b233ef6121f9f19f1c3890f141a3442465dc43b9a42ca21bb8869d0fd060c1c5`
* GitHub release: <https://github.com/rcarmo/memento/releases/tag/v0.3.27>

The exact release commit passed [ordinary CI run 33247081418](https://github.com/rcarmo/memento/actions/runs/33247081418): cached runtime-model preparation, Python 3.12, 3.13 and 3.14 checks, wheel build and installation, container build and runtime-model smoke tests all succeeded. [Release run 33247173712](https://github.com/rcarmo/memento/actions/runs/33247173712) then passed all 11 jobs, including tag validation, the Python matrix, native amd64 and arm64 builds, the no-AVX Westmere smoke, multi-architecture publication, GitHub release publication and retention.

The `0.3.27`, `0.3`, `0` and `latest` registry tags all resolved to the published manifest above. Local validation passed Ruff, formatting, mypy, 321 tests, browser graph checks, Rust formatting, Clippy, tests and doc-tests. The coverage run also passed all 321 tests at 85% total coverage. Building and installing `memento-0.3.27-py3-none-any.whl` succeeded, and imports resolved Memento `0.3.27`, `umcp` and `aioumcp` from the installed environment.

## Issue 13 contract and diagnostic checks

Production evidence before the change showed that malformed `access_principal_create` requests reached Memento with an empty `params.arguments` object. Correctly formed requests retained all five fields. No uMCP code was found removing supplied arguments, so this release does not change uMCP or infer security-sensitive values from malformed calls.

The deployed `tools/list` response exposed 20 direct tools. `access_principal_create` had a self-contained description and schema with:

* required `name`, `roles`, `read_prefixes`, `write_prefixes` and `idempotency_key` fields;
* the constrained principal-name pattern and role enumeration;
* namespace path constraints that describe `/path/` prefixes rather than `memory://` resource URIs;
* the rule that every write prefix must be inside a readable prefix;
* `additionalProperties: false`.

A live call with empty `arguments` returned JSON-RPC `-32602` and named all five required fields in one actionable message. It also explained that prefixes are namespace paths such as `/skills/`, not `memory://` URIs.

A second live call supplied all five fields but deliberately used `memory://invalid/` as a read prefix. It reached normal validation and returned JSON-RPC `-32602` with `namespace prefixes must start and end with '/'`. The principal list contained 18 entries before the check and 18 afterwards, and did not contain the deliberately invalid principal name. No principal or credential was created during acceptance.

## DiskStation deployment

Portainer endpoint 18, stack 111 was updated from the explicit `0.3.26` image tag to `0.3.27`. The Compose diff changed that tag and nothing else. The image pull completed through a long-lived Portainer API request because the add-on's 30-second request timeout aborted the streaming pull. A subsequent `stack.pull_and_update` completed normally from the locally available image.

The existing read-only configuration and secret mounts, writable `/volume1/docker/memento/state:/var/lib/memento` bind, semantic settings and runtime-model volume were retained. No config helper, runtime-model preparation, index rebuild or embedding refresh was invoked.

The replacement reported:

* container ID `0f77d7118f5307e127c89615498f1d85b8cc83b9059213d9e0fb10b6b210923c`;
* local linux/amd64 image ID `sha256:ee8a67ae93e91d9f268948efd7fcd9286e18f5d8e862100a6a2864ed7477722a` and the published repository digest above;
* OCI version `v0.3.27` and revision `d77aaac50ad4ef9a75cf77d016f4ed8d1846a6f3`;
* running and healthy status, with zero restarts;
* UID/GID `65532:65532`, read-only root, all capabilities dropped, `no-new-privileges:true` and the default AppArmor profile;
* the original read-only config and secret mounts, plus the intended writable state and model volumes.

Docker again reported `PidsLimit: null` despite `pids_limit: 128` in the Compose file. The Synology/Compose discrepancy remains unresolved and the requested limit is not enforced.

## Live service and preserved state

The unauthenticated endpoint returned HTTP 401 with a Bearer challenge. The authenticated admin surface reported Memento `0.3.27`, schema version 2, semantic search ready through `rust-gte` with the SQLite vector extension, and the Needle router loaded through Rust FFI.

Immediately before replacement, `memory_status` reported 172 visible concepts, repository and index revision `341132b9d3adbc5c9116e0670d33535d4f3f9149`, and `index_stale: false`. The same authenticated check after deployment reported 172 visible concepts, the same repository and index revision, no proposal backlog, and `index_stale: false`. The image replacement therefore preserved the repository and derived state without rebuilding or refreshing it.

Container startup took approximately four and a half minutes while the runtime models loaded. It then logged the normal `serve_starting` record, listened on `http://0.0.0.0:8000/mcp`, and passed its healthcheck without a restart.

## Remaining work

* Enforce or explain the missing production PIDs limit.
* Migrate principal grants before enabling protected read prefixes on the existing deployment.
* Run a clean-host restore drill.
* Measure semantic and Needle performance on real ARM64 hardware.
* Attach SBOM material to published releases.
* Add TLS before exposing any HTTP surface beyond the trusted LAN.
