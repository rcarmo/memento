---
id: pure-go-end-to-end
title: End-to-end pure Go port with scalar correctness and full coverage
status: doing
priority: high
created: 2026-09-17
updated: 2026-09-18
estimate: XL
risk: high
tags: [work-item, port, go, parity]
owner: pi
---

# End-to-end pure Go port

## Summary

Port Memento's entire service and inference runtime to Go, including uMCP wire/transport behaviour, storage and recovery, GTE and Needle. Preserve one-to-one behaviour before introducing SIMD or other performance changes. The user explicitly requires full test coverage and permits uMCP Git tip as a reference.

## Acceptance Criteria

* Isolated `go` branch/worktree; no changes to the active Vulkan checkout or production.
* Runtime builds with `CGO_ENABLED=0` and needs no Python/Rust/C inference, native SQLite extension or external Git executable for core operations.
* All rows of [the parity matrix](../../docs/go-port/parity.md) complete with actual reference/Go comparison tests; no placeholder success handlers.
* Zero uncovered statements in every implemented Go package, no exclusions, plus explicit edge/error/race/fuzz/crash tests and all upstream conformance cases.
* Preserve model/file/wire formats, trusted identity and ACLs, atomic mutation/idempotency, proposal/rebase/history/assets, bounded memory and shutdown semantics.
* Scalar numerical and tokenizer/output parity first; SIMD/GPU/quantisation deferred. Any floating tolerance or ABI exception is explicit and reviewed.

## Implementation Paths

A (chosen): side-by-side pure Go subtree with pinned oracles, slice-by-slice tests and full replacement gates. Keeps the existing runtime available for differential tests and rollback.

B (rejected for this request): replace only the Rust workers and retain the Python service. Lower risk, but does not satisfy the user's end-to-end requirement. CGo/native-inference wrappers also do not satisfy the pure-Go runtime target.

## Test Plan

* `make -C go check`: format/vet, strict statement coverage, CGO-free tests/build.
* `make -C go race`, `make -C go fuzz`, `make -C go cross`; native amd64 and ARM64 CI.
* Independent pinned Python/uMCP/Rust oracle generation, checked-in synthetic expected outputs.
* Full uMCP sessions/transports/auth conformance; service/security/storage/concurrency/crash suites; GTE/Needle model/tokenizer/routing goldens and browser tests as each subsystem lands.
* No production fixture mutations. Migration and rollback on disposable state copies before any separately approved deployment.

## Definition of Done

* [x] Dedicated branch and reference revisions recorded.
* [x] Scope/index, staged plan and test policy written.
* [x] Initial dependency-free Go code with oracle-backed tests and strict coverage gate.
* [x] Native amd64/ARM64 CI passed for the initial slice.
* [ ] All uMCP and service surfaces ported and tested.
* [ ] Pure-Go persistence/Git/SentencePiece decisions proven by experiments.
* [ ] Scalar GTE and Needle parity complete.
* [ ] Full operational/resource/crash/migration/rollback gates passed.
* [ ] External C ABI consumers resolved without silently weakening scope.
* [ ] User-authorised deployment and complete validation before production replacement.

## Updates

### 2026-09-18

* Added dynamic resource/template and prompt methods with sync/async differential fixtures, metadata isolation, pagination, binary/nested content, subscription state and concurrency/error tests. All current Go statements covered. Completion, logging, server composition and network/session transports remain gaps.
* Added dynamic uMCP tool registration/list/call, signature coercion, schema-subset validation and ordered Python-compatible result text. Oracle covers 380 formatting/schema and 40 call cases in both sync/async references. Focused review found missing panic containment; added remote-redacted recovery. Strict coercion suggestions deliberately not applied because they change source behaviour; service validators remain an open requirement.
* Added the Go embedding-worker executable and framed protocol, with Rust response fixtures, short/partial I/O, bounds and nonfinite-output tests. Unchanged Python client accepts five real GTE vectors and model identity; local coverage/race/fuzz/cross gates pass. Malformed JSON/duplicate-key fidelity and host worker integration remain incomplete.
* Inference commit `e730023` CI stopped during timed vector fuzz shutdown (`context deadline exceeded`, no failing corpus input); ARM job cancelled before corpus completion. Replaced the five-second fuzz budget with 10,000 executions plus a separate safety timeout. Local full 360-case Needle match remains valid; native full-corpus verification must be rerun.
* Scalar Needle encoder/decoder, RoPE, grouped attention, KV caches, norms/gates and constrained decoding implemented. Local full corpus comparison matches 360/360 complete Rust output strings (1408.79 seconds), with synthetic branch/error/cancellation tests reaching 100% coverage. Native corpus CI added as a required gate; service routing expansion remains unported.
* SentencePiece commit `727fe9f` passed native CI [35288453919](https://github.com/rcarmo/memento/actions/runs/35288453919). No SIMD/native runtime or production change.

### 2026-09-17

* Added Apache-2.0-derived pure-Go SentencePiece protobuf/model parsing, BPE/Unigram, charsmap/whitespace normalisation and byte decode, plus Needle special-token handling. All implemented statements covered; local race/fuzz/cross pass. 288 synthetic oracle cases and 15 real encode/decode cases plus 8192 token decodes match Rust. Encoder/decoder and constrained generation are next.
* Needle NDL1 parser implemented with section/metadata checksums, tensor bounds and BF16 expansion; the real-model oracle matches 31 tensors and 8192 pieces. Local parser tests reach 100% statement coverage. Encoding/inference and exact malformed-diagnostic parity remain open. GTE commit `86bd39e` passed native CI [35286859612](https://github.com/rcarmo/memento/actions/runs/35286859612), including real-model parity on both architectures.
* Restored scalar Go GTE1 loading/tokenisation/transformer inference, based on original Go GTE algorithms with Memento Unicode and cancellation compatibility. All implemented Go statements covered; check/race/fuzz/cross pass. Public GTE1 SHA verified; five real model inputs match exact token IDs with local max_abs<=1.2e-7 against Rust. Native model-parity CI added; mmap/resource and worker integration still pending. URL helper commit `b0be94e` passed CI [35285682108](https://github.com/rcarmo/memento/actions/runs/35285682108).
* Added URL/Origin helpers with Python-generated allowlist, loopback, IPv6/port, raw path and Unicode NFKC delimiter fixtures. `golang.org/x/text v0.23.0` is a pure-Go dependency; no native runtime introduced. Local gates remain at 100% coverage. Stdio commit `b38896d` passed native CI [35285053975](https://github.com/rcarmo/memento/actions/runs/35285053975).
* Added sequential stdio framing with actual sync/async transport-loop fixtures, invalid UTF-8 handling, notification suppression and large-line tests. Local coverage/race/fuzz/cross gates pass at 100%; pagination commit `fc7a78e` also passed native CI [35284597442](https://github.com/rcarmo/memento/actions/runs/35284597442).
* Discovery pagination now matches 51 pinned sync/async cases, including principal/list binding and exact opaque bytes. Local check/race/fuzz/cross gates pass at 100% statement coverage. Dispatch/progress slice `877738c` passed native CI [35283952029](https://github.com/rcarmo/memento/actions/runs/35283952029).
* Continued uMCP port with HTTP helper rules, dispatcher validation/context, cooperative cancellation and progress; oracle runs both Python sync/async bases. Fixed exact large-integer progress comparisons and Python/Go status-phrase differences exposed by fixtures. Origin/URL rules, discovery and real transports remain incomplete.
* Initial scaffold committed/pushed as `b654c892c75f1aa436a988ff905ec9b60e5bea51`; native amd64 and ARM64 [Go port CI 35282374718](https://github.com/rcarmo/memento/actions/runs/35282374718) passed coverage, race, fuzz and static cross-build gates. This completes branch preparation, not the end-to-end port. Next: remaining uMCP shared rules and transport/session conformance.
* Created branch `go` at Memento `0b0b8f94dd8b0410a0e3c0fd547e995d2b739b41` in `/workspace/projects/memento-go`.
* Pinned uMCP Git tip `30cce7dfe08c6ee63de235f7d81754ba286dafbb`; difference from deployed `9c89a70` is documentation-only.
* Added scalar vector primitives, response envelopes, uMCP ID/response/version helpers, development-only CLI and oracle fixtures. Each current package reached 100% statement coverage locally; service/model subsystems remain unported.
* User explicitly requested full coverage; added raw zero-uncovered-block enforcement rather than trusting the rounded display percentage.
* Validation: all 17 current production Go functions at 100% statement coverage; race tests pass; uMCP fuzz ran about 134k executions and vector fuzz about 636k; baseline amd64/ARM64 binaries are statically linked. Oracle fixtures regenerate byte-identically. Original Python/Rust `make check` and `make typecheck` pass from this worktree.
* Oracle Python script passes Ruff/format/mypy independently. New documentation file links resolve. Delegated scope audit timed out; no independent approval claimed.
* Quality: ★★★★☆ 8/10 (problem 2, scope 2, test 2, dependencies 0, risk 2). Git/SQLite/SentencePiece/ABI decisions are explicit open gates, not blockers to the independent foundations.

## Notes

Refinement: full end-to-end service, scalar first, CGO-free runtime, current data/model/wire contracts, at least baseline Linux amd64 and ARM64. Keep frontend assets; do not mistake browser JavaScript for a backend dependency. Old implementations are test-only references until retirement. Full correctness is a compatibility objective, not a claim of a mathematical proof from test coverage. No release/deployment is requested in this preparation step.

## Links

* [Go port index](../../docs/go-port/README.md)
* [Architecture and phases](../../docs/go-port/plan.md)
* [Test gates](../../docs/go-port/testing.md)
