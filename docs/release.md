# Release

The release path proves the project can be built, checked and installed locally and in CI. It does not yet prove published artefacts, registry state or supply-chain evidence end to end, and that gap should stay explicit.

## Local release checklist

* `make install-dev`
* `make check`
* `make coverage`
* `make build-wheel`
* `make install-wheel`
* `make diff-check`

## Packaging notes

* The wheel path validates the Python package, but platform-specific Rust semantic libraries are still deployment artefacts built separately.
* The container build currently copies the vendored `rust/tests/fixtures/gte-small.gtemodel` into `/usr/local/share/memento/models/gte-small.gtemodel` and sets default semantic-library environment variables. Semantic search still depends on config enablement.
* Principal bearer tokens remain mandatory runtime configuration. Provider API keys and semantic path overrides are optional environment variables, used only when the config calls for them.

## CI

GitHub Actions validates Python 3.12--3.14 through the Make targets, builds a wheel, installs it, and performs a container build check.

## Image provenance limits

The current Dockerfile uses floating base images (`rust:1-slim` and `python:3.14-slim`). That means a locally observed image ID is useful as a point-in-time build note, but it is not a reproducible published digest claim. Until release automation pins base-image digests and publishes immutable outputs, do not present local image IDs as if they were stable release artefacts.

## Pending publication evidence

The current workflow is build validation only.

* Registry push remains pending.
* SBOM publication remains pending.
* Provenance attestation remains pending.
* Immutable digest publication remains pending.
* Live verification of published artefacts remains pending.
