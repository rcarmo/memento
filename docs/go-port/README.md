# Pure Go port: correctness before optimisation

Branch `go` is an end-to-end replacement project: the service, uMCP, storage/recovery, GTE, Needle and operations tooling, not just the native embedding worker. The original Python/Rust implementation is retained as the test reference until each replacement passes its compatibility gates.

The Go runtime must build with `CGO_ENABLED=0`, without Python, Rust, C inference, native SQLite extensions or external Git commands for core operation. Existing browser JavaScript and static assets can be served unchanged by Go; rewriting the browser in another language is not part of this port. Python/Rust/Git remain permissible in the independent test oracle. SIMD, assembly, quantisation, GPU integration and performance-driven algorithm changes are deferred.

## Start here

* [Parity matrix](parity.md): every subsystem, its reference and completion gate.
* [Implementation plan](plan.md): architecture decisions, staged work and unresolved dependencies.
* [Test contract](testing.md): exact comparisons, numerical gates, coverage, fuzzing and reference regeneration.
* [Go package instructions](../../go/AGENTS.md): runtime and correctness constraints.
* [Initial work item](../../workitems/20-doing/pure-go-end-to-end.md): acceptance criteria and progress.

## Pinned references

* Memento: `0b0b8f94dd8b0410a0e3c0fd547e995d2b739b41`, main after the 0.5.9 deployment report.
* Deployed uMCP dependency: `9c89a708d14ae804e32aa65de10af7c02922617d`.
* uMCP Git tip authorised as reference: `30cce7dfe08c6ee63de235f7d81754ba286dafbb`. Its delta from the dependency pin is documentation-only, including persistent Streamable HTTP sessions. `umcp.py`, `aioumcp.py` and `umcp_shared.py` are unchanged between those revisions.
* Recorded in [`baseline.json`](../../go/testdata/parity/baseline.json). Do not silently follow upstream tip; review reference updates and regenerate fixtures explicitly.

The isolated checkout is `/workspace/projects/memento-go`. `/workspace/projects/memento` contains separate Vulkan work. Bring accepted upstream fixes into `go` with merge, never rebase; update the pinned comparison baseline only through an explicit review.

## What runs today

The Go scaffold implements response-envelope constructors, scalar vector encoding/dot/cosine/AXPY, uMCP ID/response/version helpers, HTTP media/header/response rules, and a request dispatcher with isolated contexts, cooperative cancellation and progress. Sync and async Python reference fixtures verify dispatch and progress behaviour, including arbitrary-size JSON integer IDs and exact large-integer progress comparisons. The dispatcher invokes only registered handlers; it is not a complete tool/resource/prompt server. List pagination also matches principal/list-bound cursor bytes and error behaviour from both uMCP bases. A sequential stdio loop now matches both Python bases on blank/final lines, invalid UTF-8 replacement, notifications and requests longer than 64 KiB. It takes implemented handlers and is not yet wired as a Memento daemon. Origin/URL helpers also match fixture-tested Python splitting, host/port and NFKC delimiter checks using the pure-Go `golang.org/x/text` dependency. Discovery/method implementations and HTTP/session/TCP/SSE/file modes remain unported. Its synthetic fixtures were emitted by the actual pinned Python and Rust implementations, not handwritten copies of expected values. The operation registry, twenty surface configurations and argument/execute schemas are captured as references but are **not implemented service endpoints**.

`go/gte` now restores the original Go GTE1 layout, tokenizer and scalar transformer algorithms, retaining Memento's Unicode character-boundary fix and cancellation checkpoints. Synthetic Rust oracle cases pass, and a digest-pinned 384-dimensional model test compares five real inputs. The loader currently copies weights; mmap/resource parity, expanded corpora and worker integration remain unfinished. No assembly, native BLAS or fast-math path is enabled.

The Needle NDL1 parser also loads the real model: all 31 tensors, all 8192 vocabulary entries, metadata and BF16-to-float32 hashes match Rust. Pure-Go SentencePiece encoding/decoding (BPE/Unigram, normalisation and byte fallback) and Needle's special-token rules pass synthetic and real tokenizer comparisons. Scalar Needle encoder/decoder, RoPE, grouped attention, KV caches, gated norms and constrained generation are now implemented. Five real generation cases match output/checkpoint sequences, and all 360 held-out queries match complete Rust output strings locally. Native corpus CI is a required gate; parser error-text fidelity, broader malformed/configuration cases and service integration remain open. The derived SentencePiece package retains its Apache-2.0 licence separately from Memento's MIT code.

`memento-go version` reports that the port is incomplete. Other commands exit with an explicit error; there is no fake status server or placeholder inference.

```sh
make -C go check  # format, vet, zero-uncovered-statement gate, CGO-free build
make -C go race   # test-only cgo for Go's race detector
make -C go fuzz   # bounded initial fuzz runs
make -C go cross  # baseline amd64 and arm64 builds
```

Passing these initial tests does not establish complete uMCP, MCP, storage or inference parity. Every remaining row in the matrix must be implemented and verified before this branch can replace production. No release, migration or deployment is authorised by creating the branch.
