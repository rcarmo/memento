# DiskStation deployment notes

The deployment candidate targets a Synology DiskStation with an Intel Celeron J3455 (Apollo Lake). That CPU supports SSE4.2 but not AVX, AVX2 or FMA.

Memento's vector kernels select AVX2/FMA only after runtime feature detection. On the J3455 they use the scalar implementation automatically. The amd64 release build also sets Rust's target CPU to baseline `x86-64`, which prevents the GitHub runner's newer CPU features from leaking into ordinary generated code.

Before any NAS deployment, the release workflow runs the amd64 image under QEMU's Westmere CPU model and checks:

* SSE4.2 is visible;
* AVX2 and FMA are not visible;
* GTE-small loads and produces a 384-value embedding;
* the fine-tuned Needle router loads and produces one valid shallow action.

Native image tests measured Needle at 185 MiB peak RSS. GTE-small reached about 297 MiB because its FP32 model is expanded during inference. The initial DiskStation profile therefore enables Needle only and leaves semantic search off. If semantic search is enabled later, raise the container limit to at least 384 MiB; 512 MiB leaves more room for concurrent requests.

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

The template pins `MEMENTO_VERSION`, binds MCP to loopback for a reverse proxy, runs as UID/GID 65532, drops Linux capabilities, uses a read-only root filesystem and sets a 320 MiB memory limit for Needle with semantic search disabled. Check RSS and latency on the J3455 before changing that limit.

No DiskStation deployment is performed by GitHub Actions. Release automation builds and tests the image, then publishes it to GHCR. Updating the NAS remains a separate operator action with an explicit version and rollback plan.
