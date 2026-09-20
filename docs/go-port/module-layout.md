# Go module layout

Memento's primary Go module lives at the repository root and builds with `CGO_ENABLED=0`; ordinary runtime and verification do not require Python or Rust. Application packages are deliberately internal because the repository exposes binaries and protocols, not a public Go SDK. The independently reusable uMCP implementation remains a nested module under `umcp/`.

## Commands

* `cmd/memento-go` runs the service daemon and operational status, rebuild, audit, backup/restore, key-rotation and Dream commands.
* `cmd/memento-embed-go` exposes pure-Go GTE inference through the framed embedding-worker protocol.
* `cmd/memento-needle-model-go` converts validated NDL1 weights into the deterministic mapped FP32 release sidecar.
* `cmd/memento-needle-go` maps that sidecar read-only and serves bounded framed routing requests.
* `cmd/memento-skill-import-go` validates and atomically imports recalled skill packs.

Command packages contain process concerns only. Reusable application code belongs under `internal/`.

## Packages

* `internal/service` composes identity, policy, persistence and tool behaviour.
* `internal/repository`, `internal/control`, `internal/derived`, `internal/access` and `internal/assets` own storage boundaries.
* `internal/execute` validates and runs bounded multi-operation plans.
* `internal/gte`, `internal/needle`, `internal/sentencepiece`, `internal/embedding` and `internal/needleworker` implement pure-Go inference and worker framing.
* `internal/envelope`, `internal/pydatetime`, `internal/pyjson`, `internal/simd` and `internal/vector` are shared implementation details.
* `umcp` is a separate module with no Memento imports.

Dependencies point toward lower-level storage/protocol packages. Commands depend on internal packages, internal packages never import `cmd`, and uMCP remains independently buildable.

## Test and generated data

`testdata/parity` contains checked-in synthetic/public differential fixtures. The removed Python/Rust reference harness generated those fixtures before the pure-Go cutover; they are immutable compatibility records and production commands do not execute either language.

Generated runtime tables sit beside their consuming package and use `//go:embed`. Fixture provenance and reference revisions live in `testdata/parity/baseline.json`. Recreate fixtures only on a dedicated reference branch, then review imported data separately.

Build artefacts go under ignored `build/`.

## Checks

```bash
make quality
make audit
make performance
make model-test
make release-check MEMENTO_VERSION=1.0.0 SOURCE_DATE_EPOCH=0
make install DESTDIR=/tmp/package-root PREFIX=/usr/local
```

Release archives contain stripped static `linux/amd64` (`GOAMD64=v1`) and `linux/arm64` binaries for all five commands. `release-check` validates archive layout, architecture, absent ELF dynamic dependencies, checksums and native version output. The OCI build uses the converter to generate and verify the architecture-independent FP32 Needle sidecar once from the pinned NDL asset, avoiding duplicate 105 MB copies in both binary archives.

`make quality` also verifies that every production package has a meaningful fuzz target. `make audit` adds race, fuzz, model-independent allocation and cross-build checks; `make performance` adds pinned real-model benchmarks. `make layout-check` verifies module tidiness, package discovery, command placement, the absence of root-level Go source, and removal of the legacy `go/` subtree.
