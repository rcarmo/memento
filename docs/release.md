# Release

The release path proves the project can be built, checked and installed locally and in CI. It does not yet prove published artefacts, registry state or supply-chain evidence end to end, and that gap should stay explicit.

## Local release checklist

* `make install-dev`
* `make check`
* `make coverage`
* `make build-wheel`
* `make install-wheel`
* `make diff-check`

## CI

GitHub Actions validates Python 3.10--3.12 through the Make targets, builds a wheel, installs it, and performs a container build check.

## Pending publication evidence

The current workflow is build validation only.

* Registry push remains pending.
* SBOM publication remains pending.
* Provenance attestation remains pending.
* Immutable digest publication remains pending.
* Live verification of published artefacts remains pending.
