# DiskStation deployment notes

The deployment candidate targets a Synology DiskStation with an Intel Celeron J3455 (Apollo Lake). That CPU supports SSE4.2 but not AVX, AVX2 or FMA.

Memento's vector kernels select AVX2/FMA only after runtime feature detection. On the J3455 they use the scalar implementation automatically. The amd64 release build also sets Rust's target CPU to baseline `x86-64`, which prevents the GitHub runner's newer CPU features from leaking into ordinary generated code.

Before any NAS deployment, the release workflow runs the amd64 image under QEMU's Westmere CPU model and checks:

* SSE4.2 is visible;
* AVX2 and FMA are not visible;
* GTE-small loads and produces a 384-value embedding;
* the fine-tuned Needle router loads and produces one valid shallow action.

Native image tests measured Needle at 185 MiB peak RSS. GTE-small reached about 297 MiB because its FP32 model is expanded during inference. The DiskStation profile uses short-lived GTE workers, disables startup embedding refresh and sets a 512 MiB container limit so the trusted-LAN graph UI can refresh selected, visible or full embeddings on demand without keeping the model resident in the service process.

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

The template pins `MEMENTO_VERSION`, publishes MCP on port 18081, runs as UID/GID 65532, drops Linux capabilities, uses a read-only root filesystem and sets a 512 MiB memory limit for Needle plus subprocess GTE embedding refresh. The bearer-token file is mounted read-only and sourced by the container entrypoint because a remote Portainer server cannot resolve an endpoint-local `env_file` during Compose parsing.

The trusted-LAN profile also enables the visual debugger at `http://192.168.1.250:18081/graph`. Browser module requests carry an Origin header, so the exact LAN origin appears in `mcp.allowed_origins`; arbitrary origins remain blocked. Leave `observability.graph_explorer.enabled` off on an Internet-facing deployment.

The deployed J3455 profile uses a 30-second `memory_execute` budget. A real-target benchmark found exact reads at 9.32 ms p50, lexical search at 601 ms p50/1.80 s p95, graph lookup through `memory_execute` at 392 ms p50, Git-backed patch/rename operations at 2.1--3.1 seconds and scalar Needle routes at 10--13 seconds. The full report is [`docs/evidence/diskstation-memory-benchmark-2026-07-19.json`](evidence/diskstation-memory-benchmark-2026-07-19.json).

A commit may finish just after the execute deadline and still return a controlled timeout to the client. Mutation callers must reconcile an ambiguous timeout using the idempotency key, repository revision and target path before retrying. Raw punctuation in lexical queries also needs FTS5 quoting or escaping; ordinary term queries are the safer default.

No DiskStation deployment is performed by GitHub Actions. Release automation builds and tests the image, then publishes it to GHCR. Updating the NAS remains a separate operator action with an explicit version and rollback plan.
