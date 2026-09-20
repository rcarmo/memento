# Pure-Go Memento module

This directory is the standalone Go module for Memento `v1.0.0`. It builds with `CGO_ENABLED=0`; the runtime and ordinary verification do not require Python or Rust.

The module is the verified `v1.0.0` replacement on branch `go`; production deployment remains an operator release action. [`../docs/go-port/parity.md`](../docs/go-port/parity.md) records matched behaviour, explicit typed-boundary differences and deferred non-baseline work.

## Commands

* `cmd/memento-go` runs the pure-Go service daemon and operational status, rebuild, audit, backup/restore, key-rotation and Dream commands. Model-backed modes require their configured provider slots and fail closed otherwise.
* `cmd/memento-embed-go` exposes pure-Go GTE inference through Memento's framed embedding-worker protocol, using automatic AVX2/SSE2/NEON/scalar dispatch unless overridden.
* `cmd/memento-skill-import-go` validates and atomically imports recalled skill packs into a workspace.

Command packages contain process concerns only. Reusable code belongs in library packages.

## Packages

* `umcp` implements protocol dispatch and transports.
* `service` composes identity, policy, persistence and tool behaviour.
* `repository`, `control`, `derived`, `access` and `assets` own storage boundaries.
* `execute` validates and runs bounded multi-operation plans.
* `gte`, `needle`, `sentencepiece` and `embedding` implement pure-Go inference and worker framing.
* `internal/envelope`, `internal/pydatetime`, `internal/pyjson` and `internal/vector` are implementation details shared inside this module.

Dependencies point towards lower-level storage/protocol packages; commands depend on libraries, and libraries never import `cmd`.

## Test and generated data

`testdata/parity` contains the checked-in synthetic/public differential fixtures used by package tests. The removed Python/Rust reference harness generated those fixtures before the pure-Go cutover; they are immutable compatibility records on this branch and production commands do not import or execute either language.

Generated runtime tables sit beside their consuming package and use `//go:embed`. Fixture provenance and reference revisions live in `testdata/parity/baseline.json`. Recreate or update fixtures on a dedicated reference branch, then review the imported data separately.

Build artefacts go to `../build/go`, outside this module tree.

## Checks

```bash
make quality
make performance GTE_MODEL_PATH=../models/gte/gte-small.gtemodel NEEDLE_MODEL_PATH=../models/needle/memento-router.ndl NEEDLE_TOKENIZER_PATH=../models/needle/needle.model
make model-test GTE_MODEL_PATH=../models/gte/gte-small.gtemodel NEEDLE_MODEL_PATH=../models/needle/memento-router.ndl NEEDLE_TOKENIZER_PATH=../models/needle/needle.model
make audit
make release-check VERSION=1.0.0 SOURCE_DATE_EPOCH=0
make install DESTDIR=/tmp/package-root PREFIX=/usr/local
```

Release archives contain static stripped `linux/amd64` (`GOAMD64=v1`) and `linux/arm64` binaries for all three commands, a version marker and a sorted SHA-256 manifest. `release-check` validates archive layout, architecture, absence of ELF dynamic dependencies, checksums and the native `version` smoke test. Fixed source epochs, sorted tar entries, numeric ownership, `-trimpath` and disabled VCS metadata make identical source/toolchain inputs byte-reproducible.

`make quality` also verifies that every package containing production Go code exposes at least one fuzz target. The root and nested uMCP modules currently provide 54 targets and the audit runs each for 10,000 deterministic executions. `make layout-check` verifies module tidiness, package discovery, command placement and that no Go source leaks into the module root. Repository-level model, performance, container and release gates remain mandatory for release work.
