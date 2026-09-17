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

The Go scaffold implements response-envelope constructors, a subset of uMCP shared validation/protocol helpers and scalar vector encoding/dot/cosine/AXPY. Its synthetic fixtures were emitted by the actual pinned Python and Rust implementations, not handwritten copies of expected values. The operation registry, twenty surface configurations and argument/execute schemas are captured as references but are **not implemented service endpoints**.

`memento-go version` reports that the port is incomplete. Other commands exit with an explicit error; there is no fake status server or placeholder inference.

```sh
make -C go check  # format, vet, zero-uncovered-statement gate, CGO-free build
make -C go race   # test-only cgo for Go's race detector
make -C go fuzz   # bounded initial fuzz runs
make -C go cross  # baseline amd64 and arm64 builds
```

Passing these initial tests does not establish complete uMCP, MCP, storage or inference parity. Every remaining row in the matrix must be implemented and verified before this branch can replace production. No release, migration or deployment is authorised by creating the branch.
