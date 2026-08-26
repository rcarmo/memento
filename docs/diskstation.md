# DiskStation deployment notes

The live deployment runs on a Synology DiskStation with an Intel Celeron J3455 (Apollo Lake). That CPU supports SSE4.2 but not AVX, AVX2 or FMA.

Memento's vector kernels select AVX2/FMA only after runtime feature detection. On the J3455 they use the scalar implementation automatically. The amd64 release build also sets Rust's target CPU to baseline `x86-64`, which prevents the GitHub runner's newer CPU features from leaking into ordinary generated code.

Before publishing any NAS candidate, the release workflow runs the amd64 image under QEMU's Westmere CPU model and checks:

* SSE4.2 is visible;
* AVX2 and FMA are not visible;
* GTE-small loads and produces a 384-value embedding;
* the fine-tuned Needle router loads and produces one valid shallow action.

Native image tests measured Needle at 185 MiB peak RSS. GTE-small reached about 297 MiB because its FP32 model is expanded during inference. The DiskStation profile uses short-lived GTE workers, disables bulk startup refresh and sets a 512 MiB container limit. Progressive generation and selected/visible/full manual requests share one low-priority gated queue, so the model is not kept resident in the service process.

The DiskStation Compose template is [`deploy/diskstation.compose.yaml`](../deploy/diskstation.compose.yaml). It uses:

```text
/volume1/docker/memento/config/config.json
/volume1/docker/memento/config/memento.env          container bearer tokens
/volume1/docker/memento/config/compose.env          MEMENTO_VERSION for Compose
/volume1/docker/memento/state/
```

Prepare the files from the examples and replace both token placeholders with independent random values:

```bash
mkdir -p /volume1/docker/memento/config /volume1/docker/memento/state
cp deploy/diskstation.config.example.json /volume1/docker/memento/config/config.json
cp deploy/diskstation.env.example /volume1/docker/memento/config/memento.env
cp deploy/diskstation.compose.env.example /volume1/docker/memento/config/compose.env

docker compose \
  --env-file /volume1/docker/memento/config/compose.env \
  -f deploy/diskstation.compose.yaml config
docker compose \
  --env-file /volume1/docker/memento/config/compose.env \
  -f deploy/diskstation.compose.yaml up -d
```

The template pins `MEMENTO_VERSION`, publishes MCP on port 18081, runs as UID/GID 65532, drops Linux capabilities, uses a read-only root filesystem and sets a 512 MiB memory limit for Needle plus subprocess GTE embedding refresh. Its TCP healthcheck has a five-minute startup grace because persisted-state reconciliation and SQLite/Git recovery complete before the listener opens. The bearer-token file is mounted read-only and sourced by the container entrypoint because a remote Portainer server cannot resolve an endpoint-local `env_file` during Compose parsing.

The trusted-LAN profile also enables the visual debugger at `http://192.168.1.250:18081/graph`. Browser module requests carry an Origin header, so the exact LAN origin appears in `mcp.allowed_origins`; arbitrary origins remain blocked. Leave `observability.graph_explorer.enabled` off on an Internet-facing deployment.

The deployed J3455 profile uses a 30-second `memory_execute` budget. A real-target benchmark found exact reads at 9.32 ms p50, lexical search at 601 ms p50/1.80 s p95, graph lookup through `memory_execute` at 392 ms p50, Git-backed patch/rename operations at 2.1--3.1 seconds and scalar Needle routes at 10--13 seconds. The full report is [`docs/evidence/diskstation-memory-benchmark-2026-07-19.json`](evidence/diskstation-memory-benchmark-2026-07-19.json).

A commit may finish just after the execute deadline and still return a controlled timeout to the client. Mutation callers must reconcile an ambiguous timeout using the idempotency key, repository revision and target path before retrying. Natural-language search should use the default `query_syntax="plain"`, which tokenises terms and treats punctuation and operator words literally. Use `query_syntax="fts5"` only for deliberate raw FTS5 expressions.

No DiskStation deployment is performed by GitHub Actions. Release automation builds and tests the image, then publishes it to GHCR. An operator pulls the immutable release, runs a uniquely named one-shot config helper to completion, updates the Portainer stack with an explicit version and verifies container, MCP, graph and revision health before considering the update complete. The helper runs as UID/GID 65532, writes and fsyncs a sibling file, then atomically replaces the root-owned `config.json`; this fits the Synology ACL, where the service UID owns the directory but cannot overwrite that file in place. It has no network, capabilities or writable root filesystem, and only the config directory is writable. Helper or stack failures stop the deployment instead of being treated as an asynchronous success.

## Current release

The trusted-LAN service runs `ghcr.io/rcarmo/memento:0.3.24` from the immutable multi-architecture manifest `sha256:f04004c89e915c5c6c35197bfbba7b3dbf7f23fcf42b664ef3800aab38a27c08`. The 2026-08-25/26 acceptance check found the replacement container healthy, non-root, read-only, capability-free, within its 512 MiB limit and not restarting or OOM-killed. MCP status, plain and raw-FTS5 search, bounded inventory, execute-only manifest comparison, graph traversal and the trusted-LAN overview passed. The complete record is [`docs/evidence/release-0.3.24.md`](evidence/release-0.3.24.md).

The existing production configuration has not yet opted into `authorization.protected_read_prefixes`; enabling the example's `/work/`, `/personal/` and `/infrastructure/` mask requires an explicit migration of broad-reader grants. Production deliberately omits `memory_answer`: the compact answer tool is disabled and no provider slots are configured. Release tests cover protected namespace policy, the versioned evidence contract and secret-first abstention, but the DiskStation check does not claim those disabled paths as live acceptance.

Docker inspection still reports no effective PIDs limit despite the template's `pids_limit: 128`; operators must resolve that Synology/Compose discrepancy before treating the limit as enforced. The low-priority progressive worker completed the release's 154-concept rebuild on 2026-08-26 without an error or restart. Repository, lexical-index and embedding revisions now agree, and semantic and hybrid ranking are ready.

## Progressive embeddings

The DiskStation profile uses one low-priority concept every 30 seconds after a two-minute startup grace and 15 seconds of interactive idle time. Work pauses when sampled CPU utilization exceeds 75% over a 15-second window. Linux I/O wait is treated as idle, so normal NAS storage load does not block progress. `nice 15` and single-thread native pool variables keep inference subordinate to MCP and storage workloads. The `/volume1/docker/memento/state:/var/lib/memento` mount preserves `derived.sqlite` and completed vectors across image upgrades.

Do not delete `derived.sqlite` during routine releases. A deliberate rebuild reuses unchanged embeddings and schedules only stale or missing paths. The original live `v0.3.12` exercise preserved 99 ready rows through an image update/rebuild, then progressively completed six queued paths with no errors until repository, index and embedding revisions matched. Later image replacements use the same persisted state and gated worker path.

## Managed access on DiskStation

Provide `MEMENTO_ADMIN_MASTER_KEY` in `/volume1/docker/memento/config/memento.env`. Existing `MEMENTO_TOKEN_*` values are imported at bootstrap and retained only for emergency recovery. Open `http://<diskstation>:18081/admin` with the bootstrap credential, now identified as `sandbox`, and issue least-privilege instance credentials from presets. Follow [Access Management](access-management.md) for backup and explicit one-shot key rotation.
