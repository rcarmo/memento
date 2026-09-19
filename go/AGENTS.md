# Pure-Go port

This subtree is the verified end-to-end pure-Go `v1.0.0` replacement. Production rollout remains separately authorised; the repository-level `AGENTS.md` still applies.

## Runtime and parity boundaries

* Runtime commands and distributable binaries must build with `CGO_ENABLED=0`. Do not add Python/Rust subprocess inference, CGo, C BLAS or native SQLite extensions as runtime dependencies.
* Core service operations must not shell out to Git. Use pure-Go Git code and prove equivalent compare-and-swap, locking, object/ref and crash-recovery behaviour.
* Preserve observable Python/uMCP behaviour, ordering, diagnostics and security boundaries. Do not silently reinterpret reference behaviour while porting.
* Keep `go/umcp` generic. Memento identity, policy, persistence, execution and managed access belong in service packages and use public uMCP APIs.
* Scalar float32 inference remains the correctness oracle. SIMD changes require scalar differential tests, real-model gates and architecture-specific measurements; quantisation, GPU work and fast-math remain separate projects.
* Historical Python and Rust results survive only as checked-in parity fixtures/evidence. Go tests and builds consume those fixtures without either runtime.

## Go design rules

* Keep packages cohesive and dependencies acyclic. Commands belong under `cmd/<name>`; do not place `.go` files at the module root.
* Prefer small concrete types and narrow consumer-owned interfaces. Add abstraction only for a current caller or deterministic test seam.
* Accept `context.Context` as the first parameter for blocking I/O. Propagate cancellation and errors; do not store contexts in structs.
* Wrap errors only when the added operation/object context helps callers. Preserve protocol-compatible error text and typed errors where tests pin them.
* Do not panic for untrusted input or ordinary I/O failure. A panic is acceptable only for a proved internal invariant and must be recovered at a protocol boundary.
* Copy caller-owned maps, slices and byte buffers when retaining them. Never invoke callbacks or handlers while holding a mutex.
* Use `defer` for acquired resource cleanup once acquisition succeeds. Keep database transactions and lock scopes explicit and short.
* Bound request bodies, rows, recursion, queues, goroutines, retries, model work and output. Do not start unowned goroutines.
* Keep imports in standard-library, third-party and local groups; run `gofmt` rather than hand-formatting.
* Add package and exported-symbol comments that explain contracts. Compatibility comments should explain why a non-idiomatic behaviour is retained.

## Tests and analysis

* Prefer table-driven public-behaviour tests, deterministic clocks/randomness and small fakes. Test malformed input, cancellation, authorisation, stale state, replay, partial failure and recovery.
* Every implemented statement must be covered. Zero uncovered statements is necessary, not sufficient: include explicit error and concurrency assertions.
* Add or extend fuzz targets for parsers, framing, coercion, path handling and persisted untrusted data. `make fuzz` discovers every `func Fuzz*` target automatically; new targets must not require Makefile edits.
* Keep tests offline and repeatable. Model-dependent tests use digest-pinned public fixtures through `make model-test` or `make corpus-test`.
* Do not change compatibility fixtures by hand. Regeneration belongs on the historical reference branch; review any imported fixture update separately.
* Runtime code must cross-build for Linux amd64 and arm64. The race detector may use CGo in its test toolchain; shipped binaries may not.

## Make workflow

Use Make as the stable local and CI interface:

```text
make -C go help        # list supported targets
make -C go format      # apply gofmt
make -C go quality     # format/layout, vet, staticcheck, tests, coverage, build
make -C go audit-fast  # quality plus govulncheck
make -C go audit       # audit-fast plus race, fuzz, allocation budgets and cross-builds
```

Pinned analysis tools install under `build/go/tools` through `make -C go tools`; do not commit tool binaries. `staticcheck.conf` excludes only ST1005 because uMCP-compatible error strings retain Python capitalisation.

`govulncheck` is strict. Dependency findings must be upgraded or explicitly investigated. Standard-library findings require the first patched Go release; the current minimum secure toolchain is Go 1.26.6 because earlier Go 1.26 releases have reachable standard-library vulnerabilities.

Before committing Go changes, run at least:

```text
make -C go quality
make -C go race
make -C go fuzz
make -C go cross
git diff --check
```

For a repository-wide audit or release candidate, run `make -C go audit` and the root container contract. Work only in `/workspace/projects/memento-go` on branch `go`. NAS remains CPU-only.
