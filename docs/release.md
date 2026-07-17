# Release

Status: CI/build steps implemented; **not live verified** for published artifacts.

## Local release checklist

1. `make install-dev`
2. `make check`
3. `make coverage`
4. `make build-wheel`
5. `make install-wheel`
6. `make diff-check`

## CI

GitHub Actions runs Python 3.10-3.12 validation through Make targets, builds a wheel, installs it, and performs a container build check.

## Artifact notes

The workflow performs build validation only. Registry push, SBOM publication, provenance attestation and immutable digest publication remain documented follow-up work and are not claimed as live verified.
