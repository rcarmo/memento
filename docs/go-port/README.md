# Pure Go port

The `go` branch contains the verified pure-Go Memento replacement candidate: the service daemon, standalone uMCP module, Git/SQLite persistence and recovery, GTE, Needle, operational commands and release tooling. Runtime binaries build with `CGO_ENABLED=0` and do not require Python, Rust, C inference, native SQLite extensions or external Git commands for core operation.

Python, Rust and the pinned upstream uMCP repository remain test oracles. The separate Vulkan worktree remains outside this baseline.

## Start here

* [Parity matrix](parity.md) records matched behaviour, explicit typed-boundary differences and deferred non-baseline work.
* [Implementation plan](plan.md) preserves the sequence and decisions used to build the port.
* [Test contract](testing.md) defines differential fixtures, numerical gates, coverage, fuzzing and reference regeneration.
* [SIMD results](simd.md) cover scalar, SSE2, AVX2 and NEON validation on x86-64 and ARM64.
* [Benchmark results](benchmarks.md) compare Go and Python on the same 1,000-concept repository.
* [Go package instructions](../../go/AGENTS.md) define runtime and quality constraints.
* [Completed work item](../../workitems/30-done/pure-go-end-to-end.md) retains the acceptance criteria and historical progress log.

## Pinned references

* Memento baseline: `0b0b8f94dd8b0410a0e3c0fd547e995d2b739b41` (0.5.9).
* Deployed Python uMCP dependency: `9c89a708d14ae804e32aa65de10af7c02922617d`.
* Authorised uMCP reference tip: `30cce7dfe08c6ee63de235f7d81754ba286dafbb`.
* Machine-readable baseline: [`baseline.json`](../../go/testdata/parity/baseline.json).

Reference updates are explicit: review the upstream delta, update the pin and regenerate fixtures through the oracle. Never hand-edit expected compatibility data.

## What runs

`go/cmd/memento-go` is the CGO-free daemon and maintenance CLI. It owns authenticated uMCP transports, managed access, repository/control/derived state, proposals and direct mutations, execute plans, assets, backup/restore, audit/status/rebuild, Dream, semantic search, cited answers, model-assisted proposals and Needle routing.

`go/cmd/memento-embed-go` implements the framed embedding-worker protocol with the pure-Go GTE runtime. `go/cmd/memento-skill-import-go` validates and atomically installs recalled skill packs.

The nested `go/umcp` module has no Memento dependency. It covers dynamic tools/resources/prompts, completion/logging/notifications, cancellation/progress, stdio, file, TCP, legacy SSE and Streamable HTTP. All 254 pinned upstream Python tests and 22 differential fixture families pass.

Storage uses pure-Go Git object/ref/worktree code and modernc SQLite. Concepts and accepted assets remain canonical Git data; `control.sqlite` owns operations, proposals and managed access; `derived.sqlite` owns rebuildable FTS5, graph and embedding state. Startup recovery and legacy asset/skill migrations use the same journalled transaction path.

Optional local inference uses the original scalar Go GTE algorithm and the pure-Go Needle/SentencePiece port. Automatic SIMD dispatch is AVX2 -> SSE2 -> NEON -> scalar, with `MEMENTO_SIMD=scalar` retaining the correctness oracle. Real assets pass GTE and Needle model gates on x86-64 and ARM64; the held-out Needle corpus is 360/360 exact under NEON. The separate Vulkan branch is deferred.

## Verification

The baseline is accepted only when all of these pass:

```sh
make -C go audit
make -C go release-check
make check
```

The Go audit includes formatting/layout, vet, pinned Staticcheck, tests, a zero-uncovered-statement gate, govulncheck, race detection, every fuzz target and Linux amd64/arm64 cross-builds. Release checks build reproducible static archives for the daemon, embedding worker and skill importer, then validate layout, checksums, ELF metadata and native smoke tests.

The reference-tree check runs Ruff, formatting, strict mypy, 442 Python tests, graph/browser checks and Rust formatting, Clippy, tests and doc-tests. Model-dependent gates use SHA-256-pinned assets and are documented in [testing.md](testing.md) and [simd.md](simd.md).

## Boundaries

The baseline preserves observable wire, storage, security and model behaviour within idiomatic typed Go APIs. The parity matrix calls out narrower Python-only coercions, engine-specific diagnostics, hostile out-of-band filesystem changes, power-loss testing and external C ABI compatibility separately. Those do not add hidden runtime dependencies or placeholder handlers.

No deployment is implied by the branch. Production replacement remains an operator decision with backup, migration, smoke and rollback checks from the ordinary release process.
