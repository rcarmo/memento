# Memento pure-Go replacement

## Product boundary

This branch ships Memento as a pure-Go service and container. Runtime artifacts must not depend on Python, Rust, CGo, native SQLite extensions, an external Git executable, or a shell.

Preserve the accepted client and persisted-state contracts:

- MCP Streamable HTTP, graph/admin and staging routes;
- bearer authentication, authorization and response/error schemas;
- `/etc/memento/config.json`, `/var/lib/memento`, `/models`, port 8000 and `/mcp`;
- Git repository, control SQLite and derived SQLite formats, including rollback readability by the previous image.

The NAS target is CPU-only. Do not package or enable Vulkan for NAS without an explicit new decision.

## Layout

- `go/` — all product runtime and tests.
- `go/umcp/` — reusable standalone uMCP module; no Memento imports.
- `deploy/` — container deployment examples.
- `models/runtime-models.json` and `tools/prepare_runtime_models.py` — CI-only model bundle retrieval and digest verification. Python is allowed here only as build tooling and is never shipped.
- `docs/evidence/` and `go/testdata/parity/` — retained immutable acceptance evidence and generated fixtures.

## Required gates

Use Make targets rather than ad-hoc commands:

```sh
make -C go quality
make -C go audit
make performance
make model-test
make go-container-contract MEMENTO_VERSION=1.0.0
```

Every production statement must remain covered. Run formatting, vet, pinned Staticcheck, govulncheck, race, fuzz, cross-build and release checks before pushing.

Performance work must be profile-led. Preserve pprof evidence under ignored `build/go/profiles/` during investigation, minimize allocations in hot paths, and update `go/tools/performance-budgets.json` only with measured evidence. CI enforces allocation/byte ceilings; do not add flaky wall-clock gates across heterogeneous runners.

## Engineering rules

- Follow YAGNI; prefer small typed boundaries and standard-library code.
- Read relevant files before editing and test after every behavioral change.
- Keep native commands under `go/cmd/<name>` and reusable logic in packages.
- Preserve deterministic JSON, ordering, timestamps and error boundaries required by parity fixtures.
- Do not weaken security, cancellation, resource, corruption or recovery behavior for speed.
- Never use `git rebase`; merge/pull with `--no-rebase`.
- Commit as `Rui Carmo <rui.carmo@gmail.com>` after configuring local and global identity.
