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
* The container packages the Rust GTE and Needle runtimes, vendored models, Git and Git LFS. Git LFS stores accepted versioned asset ZIPs.
* `MEMENTO_ADMIN_MASTER_KEY` is mandatory when managed access is enabled. Bootstrap/recovery bearer variables are required for initial import; dynamically issued principal credentials live only as control-database verifiers. Provider API keys and model path overrides remain optional.
* Skill submission can require up to the configured 72 MiB MCP request limit; reverse proxies must permit the same bounded request size.

## CI and publication

Both push CI and tag releases validate Python 3.12--3.14 through the Make targets, run Rust formatting/Clippy/workspace tests, build and install a wheel, and check for a clean diff. A tag cannot start image publication until that release-owned quality matrix passes. Stable `v*` tags then publish native `linux/amd64` and `linux/arm64` images to GHCR, create a multi-architecture OCI index, publish a GitHub release and retain five releases. Fresh untagged architecture manifests are protected for seven days so cleanup cannot break a tagged index.

Published tags include the full version, major/minor, major and stable-only `latest`.

## Remaining provenance limits

Base-image manifests and GitHub Actions are pinned. SBOM attachment remains a future release improvement; BuildKit provenance attestations are included in the OCI index.

## Access-management release checks

Release validation must cover the v7 control migration, bootstrap rename to `sandbox`, admin-only tool discovery, `/admin` authentication, one-time credential behavior and the explicit offline master-key rotation command. Runtime deployment requires `MEMENTO_ADMIN_MASTER_KEY`; per-principal environment tokens are bootstrap/recovery inputs.
