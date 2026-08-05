# Release

The release path validates Python, Rust, the wheel and the container, then publishes tagged multi-architecture images and a GitHub release.

## Local release checklist

* `make install-dev`
* `make check`
* `make coverage`
* `make build-wheel`
* `make install-wheel`
* `make diff-check`
* build and smoke the release container with a fresh non-root state directory

## Packaging notes

The base images are pinned Debian Bookworm manifests: Rust 1.88 for the builder and Python 3.12 for the runtime. amd64 Rust code targets baseline x86-64; AVX2/FMA and NEON kernels are selected at runtime. The release pipeline runs the amd64 image under a no-AVX Westmere CPU model before publishing the manifest. See [ADR 0008](decisions/0008-build-for-baseline-cpus.md).

* The Python wheel contains the service and the client-side skill import command. Platform-specific Rust libraries are built separately.
* The container packages the Rust GTE and Needle runtimes, release-prepared models and Git. Accepted versioned asset ZIPs are ordinary Git blobs.
* `MEMENTO_ADMIN_MASTER_KEY` is mandatory when managed access is enabled. Bootstrap/recovery bearer variables are required for initial import; dynamically issued principal credentials live only as control-database verifiers. Provider API keys and model path overrides remain optional.
* Asset submissions that fit the configured 72 MiB MCP request limit use `zip_base64`; reverse proxies must permit the same bounded request size. Larger packs use the principal-bound raw-upload staging path.

## CI and publication

Ordinary CI runs for pushes to `main` and pull requests, cancelling superseded runs for the same branch or pull request. It does not run for release tags or special asset refs. Tag releases and manual release dispatches use a separate, non-cancelling workflow with their own Python 3.12--3.14 quality matrix, Rust checks, wheel install and clean-diff gate before image publication. Cache and artifact transfers use commit-pinned Node 24 action releases. Stable `v*` tags then publish native `linux/amd64` and `linux/arm64` images to GHCR, create a multi-architecture OCI index, publish a GitHub release and retain five releases. Fresh untagged architecture manifests are protected for seven days so cleanup cannot break a tagged index.

Published tags include the full version, major/minor, major and stable-only `latest`.

## Runtime model asset policy

Workflow checkouts require only ordinary Git. A single model-preparation job derives a cache key from `models/runtime-models.json`, restores the three runtime image artefacts (GTE model, Needle NDL and tokenizer) from the Actions cache, or downloads the pinned `model-assets-v1` GitHub Release bundle on a cache miss. The archive and every extracted file are SHA-256 checked against the committed manifest before being uploaded once as a one-day uncompressed Actions artifact. Native image builders consume that artifact. Training JSONL, vocabulary and Python checkpoint files are never downloaded by CI or release jobs.

Real GTE and Needle model coverage runs against the built container. Pointer-aware library tests skip unavailable models in matrix jobs rather than accidentally parsing pointer text. Updating a runtime model requires publishing the matching pointer-keyed release bundle before merging the pointer change.

## Progressive embedding release checks

Release validation covers pointer-only/rebuild reuse, restart-derived pending work, model-revision invalidation, manual priority, startup and interactive idle gates, `/proc/stat` CPU sampling with I/O wait treated as idle, pacing, `nice` command construction, and one-thread native environments. The operator-run DiskStation deployment preserves `/var/lib/memento`, then checks status and graph revision fields including `pause_reason`, `current_path` and `completed`.

## Remaining provenance limits

Base-image manifests and GitHub Actions are pinned. SBOM attachment remains a future release improvement; BuildKit provenance attestations are included in the OCI index.

## Access-management release checks

Release validation must cover the v7 control migration, bootstrap rename to `sandbox`, admin-only tool discovery, `/admin` authentication, one-time credential behavior and the explicit offline master-key rotation command. Runtime deployment requires `MEMENTO_ADMIN_MASTER_KEY`; per-principal environment tokens are bootstrap/recovery inputs.
