# Implementation sequence and decisions

> Status: completed on branch `go`. This document preserves the staged plan and the constraints used during implementation; current behaviour and remaining deliberate differences are recorded in [the parity matrix](parity.md).

## Phase 0: pinned references and failing gaps

Create the isolated `go` branch/worktree, pure-Go module, coverage gates, reference snapshots and this matrix. Capture public uMCP helper/schema behaviour and Rust numerical fixtures from known revisions. The initial command must identify itself as incomplete. Keep production and the separate Vulkan worktree untouched.

## Phase 1: uMCP and service contracts

Port uMCP shared validation and sync/async observable behaviours into a Go package with explicit `context.Context`, sessions and principal objects. Do not substitute a generic MCP SDK and assume compatibility: port or reuse only code that passes the pinned uMCP conformance cases. Add every upstream fixture and negative transport/security test, including raw HTTP cases that a normal HTTP client would sanitise.

Expose initialise/ping/discovery only when they are genuinely supported. Build golden catalog/argument-schema tests for all Memento surfaces and optional model modes; implement execute validation/reference/projection semantics against those schemas. Stub service methods must fail and must not appear as completed parity rows.

## Phase 2: persistence, auth and mutation correctness

Select a CGO-free SQLite implementation, with `modernc.org/sqlite` as a candidate rather than an approved dependency. Prove compatibility with the existing SQLite files, FTS5 syntax/tokenisers, connection pragmas, vector function registration, busy/lock behaviour, online backup and schema migrations before adopting it. SQLite's Go implementation must not require the Rust loadable vector extension.

Use a pure-Go Git implementation only after it proves the reference object's/tree/ref, expected-revision CAS, locking, crash-recovery and rollback semantics. `go-git` is a candidate, not a guarantee of atomic publication parity. Runtime shelling-out to Git is not the final target. Preserve current concept and control/index formats; use disposable copied fixtures, never the live data directory.

Port namespaces/credentials/ACLs, concept operations, assets, proposals/rebase, audit/history, idempotency and concurrent publication. Implement fault injection and operation reconciliation at every commit boundary before moving to model integration. Preserve original-key ownership and separation between submitted proposals and accepted Git content.

## Phase 3: scalar model correctness

Implement scalar matrix kernels and frame protocol, GTE1 parsing and tokenizer, then GTE inference against exact existing model bytes. Reuse and audit `rcarmo/go-gte` algorithm code where appropriate, but do not pull its assembly/AVX paths or change float32 algorithms for speed. Keep bounds/error/cancellation behaviour from Memento, not merely the original library's happy path.

Port NDL1/BF16 parsing and a pure-Go SentencePiece implementation for Needle. Tokenisation must match all model-relevant normalisation, byte/Unicode, control-token and segmentation behaviour; parsing the protobuf is insufficient. Add encoder/decoder reference intermediates and the complete routing corpus before trusting generated calls. Keep GTE1/NDL1 assets unchanged initially.

Retain the existing framed worker model if it is needed for bounded RAM/deadlines, but the child must be Go. No Python, Rust, C inference or CGo-based tokenizer escape hatch. Preserve process-group cleanup on Unix and equivalent lifecycle management per target platform.

## Phase 4: complete server and operational replacement

Wire read/model/write paths, graph/admin endpoints and static assets, model backends, backup/import/export, HTTP resource limits and graceful draining. Run upstream uMCP tests, Memento service scenarios and browser tests against the Go daemon. Replace runtime packaging only after the full matrix passes; keep the old implementation as the differential test oracle until deployment parity and rollback have been demonstrated.

Native C ABI users need an explicit decision: a `c-shared` Go library requires cgo, while a CGO-free standalone service does not. The source server's internal ctypes calls can disappear, but any external ABI consumers must either receive a separately approved compatibility artefact or an explicit migration. This is a closure gate, not a reason to quietly weaken pure-Go runtime requirements.

## Phase 5: optimisation, separately authorised

Scalar service/model parity and CPU SIMD optimisation are complete. GPU work remains separate and needs its own backend identity, parity, memory and performance gates.

Retain the verified NAS Vulkan container recipe from [`vulkan-nas-bookworm-2026-09-19.md`](../evidence/vulkan-nas-bookworm-2026-09-19.md): Debian Bookworm Mesa 22.3.6, Vulkan loader 1.3.239 and libdrm2 2.4.114 work through `/dev/dri/renderD128` on the Intel HD 500 when UID 65532 receives host render group 937. Mesa 26 failed. The four Rust/wgpu GTE cases reached maximum error `1.79e-7` under a standalone 512 MiB cap, but cold GPU execution was slower than CPU. A Go backend must requalify those results, warm behaviour and full-service memory before use; CPU remains the default.

## Preparation decisions

The user wants the full service and inference stack, not a hybrid “Go wrappers around native inference” runtime. Existing browser assets are kept. Runtime core operations should be CGO-free and self-contained; packaging references/tests may use the original languages. No feature deletion or new wire protocol is assumed.

Supported deployment architectures must at least include baseline linux/amd64 (including the J3455) and linux/arm64. Other platforms supported by the source need inventory and test runners before the final support matrix is signed off. CPU/scalar correctness is the priority; lower initial throughput is acceptable for development, but memory bounds and cancellation remain functional requirements.

The initial dependency-free module proves only the testing approach. SQLite, Git and SentencePiece choices remain open until compatibility experiments pass. No production migration or release is part of branch preparation. “100% correctness” is the objective of the contract/test matrix; it is not a claim that a coverage percentage proves absence of bugs.
