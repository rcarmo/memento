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

* The Python wheel contains the service and safe client-side skill import helper. Platform-specific Rust libraries remain deployment artifacts built separately.
* The container packages the Rust GTE and Needle runtimes, vendored models, Git and Git LFS. Git LFS is required for accepted versioned skill ZIPs.
* Principal bearer tokens remain mandatory runtime configuration. Provider API keys and model path overrides are optional and used only when enabled.
* Skill submission can require up to the configured 72 MiB MCP request limit; reverse proxies must permit the same bounded request size.

## CI and publication

GitHub Actions validates Python 3.12--3.14 through the Make targets, runs Rust formatting/Clippy/workspace tests, builds and installs a wheel, and builds the container. Stable `v*` tags publish native `linux/amd64` and `linux/arm64` images to GHCR, create a multi-architecture OCI index, publish a GitHub release and retain five releases. Fresh untagged architecture manifests are protected for seven days so cleanup cannot break a tagged index.

Published tags include the full version, major/minor, major and stable-only `latest`.

## Remaining provenance limits

The Dockerfile still uses floating base-image tags (`rust:1-slim` and `python:3.14-slim`). Published image digests are immutable observations of a completed build, but rebuilding the same source later may produce a different digest until base images are pinned. SBOM attachment remains a future release improvement; BuildKit provenance attestations are included in the OCI index.
