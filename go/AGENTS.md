# Pure Go port

This subtree is an in-progress end-to-end port, not a production replacement.

* Runtime and distributable commands must build with `CGO_ENABLED=0`. No Python/Rust subprocess inference, CGo, C BLAS, or native SQLite extension as runtime dependencies. The final runtime must not require shelling out to Git for core service operations; the source implementation may use Git as a test oracle. A pure-Go Git implementation must prove equivalent CAS, lock, object/ref and crash-recovery semantics.
* Port correctness first. Scalar float32 reference code only: no assembly, SIMD, quantisation, GPU work, fast-math or speculative parallelism in this stage.
* Preserve observable behaviour and safety boundaries. Do not fix or silently reinterpret source behaviour while porting; document incompatibilities and security issues explicitly.
* Reference revisions live in `testdata/parity/baseline.json`. Fixtures must identify and verify their source revision. Regeneration must be explicit and reviewable.
* Never use production memory, credentials, proposal content or assets in fixtures. Synthetic/public model fixtures only.
* Do not claim 100% correctness from passing unit tests or fixture counts. Maintain the parity matrix; missing modules and untested cases remain gaps. No catch-all successful handlers, TODO-based skips, fabricated model outputs, or unimplemented endpoints advertised as working.
* Keep the Python/Rust implementation available as the oracle. The oracle may require those tools for test generation, but Go builds/tests consume checked-in fixtures without them.
* A durable Go server needs tests for HTTP/stdio/TCP/SSE sessions, identity, cancellation, persistence/recovery/concurrency, inference and browser behaviour before replacement.
* Run `make -C go check`, `make -C go race`, `make -C go fuzz` and `make -C go cross` before committing code. Every implemented Go package must have exactly zero uncovered statements in the coverage profile, with no exclusions. Full statement coverage is necessary but insufficient: require explicit error/concurrency branches, differential fixtures and a complete contract matrix. Use only additive root build/CI changes while the original implementation remains supported.
* Work only in `/workspace/projects/memento-go` on branch `go`; the other worktree has independent Vulkan work.
