# Pure-Go Memento module

This directory is a standalone Go module for the pure-Go Memento port. It builds with `CGO_ENABLED=0`; Python and Rust are test-oracle tools, not runtime dependencies.

The port is not yet the default Memento daemon. [`../docs/go-port/parity.md`](../docs/go-port/parity.md) tracks implemented behaviour and remaining gaps.

## Commands

* `cmd/memento-go` is the service command. Its operational daemon wiring is still in progress.
* `cmd/memento-embed-go` exposes scalar GTE inference through Memento's framed embedding-worker protocol.

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

`testdata/parity` contains checked-in synthetic/public differential fixtures shared by several packages. Large files live there instead of package source directories so they are never linked into runtime binaries unless a package explicitly embeds a compact generated table.

`oracle` contains test-only Python and Rust fixture generators. Production commands do not import or execute them. Generated runtime tables sit beside their consuming package and use `//go:embed`; provenance and regeneration checks live in `testdata/parity/baseline.json` and `oracle/export.py`.

Build artefacts go to `../build/go`, outside this module tree.

## Checks

```bash
make check
make race
make fuzz
make cross
```

`make layout-check` additionally verifies module tidiness, package discovery, command placement and that no Go source leaks into the module root. The repository-level gates remain mandatory before a port slice is committed.
