# Test gates for the Go port

## Coverage is a gate, not a completion claim

Every implemented Go package must have zero uncovered statements in `go test -coverpkg=./...` output. The gate checks raw counters rather than the rounded `100.0%` display, with no excluded production files or helper allowlists. CLI entry wiring is tested too. CI runs the same gate on amd64 and ARM64; cross-compilation alone does not prove execution correctness.

Go reports statement coverage, not full branch/condition coverage. Explicitly enumerate alternate conditions, invalid types, missing fields, empty values, nulls, malformed/truncated frames, overflow, cancellation, shutdown and concurrency. Add each observed regression before changing code. Packages do not count as ported solely because their current small implementation has full coverage.

Required per-slice commands:

```sh
make -C go check
make -C go race
make -C go fuzz
make -C go cross
```

The race detector's build needs cgo on the test runner; distributed binaries and normal test builds use `CGO_ENABLED=0`. Check runtime dependencies in packaging tests. Fuzzing starts with small bounded runs for CI and needs longer scheduled corpus runs as parsers/transports are ported. No untrusted fixture can trigger unbounded allocation or leak the daemon's resources.

## Reference generation

The runtime and ordinary Go tests do not need Python or Rust. The separate test oracle does. It imports the actual pinned Python/uMCP implementation and builds the existing Rust vector crate to emit synthetic results.

```sh
make -C go oracle \
  ORACLE_PYTHON=/path/to/reference/venv/bin/python \
  UMCP_REFERENCE=/path/to/umcp-at-pinned-commit
```

`oracle/export.py` refuses mismatched uMCP HEAD or modified reference source. Memento source is compared against the recorded baseline, including uncommitted changes. Rust reference builds must pass this preflight as part of the combined target. The oracle files are never part of the shipped Go runtime.

Check in and review regenerated fixtures; ordinary CI must never regenerate expected values from the candidate implementation. Do not use production content, credentials or private model prompts. Public model binaries may be fetched through an explicit opt-in test-asset step with pinned SHA-256; they must not become opaque CI downloads from a moving tag.

Current captures:

* Python envelopes: defaults, null data, warnings, stale revisions, error fields.
* uMCP JSON-RPC ID/response validation and protocol version/error helpers, including bool-versus-int and integers larger than float64 can preserve.
* HTTP Accept quality/wildcard and content-type parsing, singleton headers, all status reason phrases from 99 through 600, and response bounds/injection checks. Runtime-typed Go responses deliberately exclude impossible Python dynamic type combinations; transport parsers must reject them at decoding.
* Request-dispatch fixtures emitted independently by both sync and async uMCP bases with the same overridden test method: malformed input, validation ordering, metadata, integer IDs, notification and cancellation results. These isolate dispatch from method implementation and do not prove discovery/tool execution.
* Actual sync/async stdio loops driven by byte streams: blank and final lines, notifications, invalid UTF-8 replacement and large requests. Expected responses compare JSON objects, not insignificant whitespace. Caller-owned blocked readers still require caller closure to interrupt; the source loop is sequential.
* Pagination fixtures emitted by both bases: principal/list identity binding, exact opaque cursor bytes, Unicode labels, large offsets/sizes, invalid params and the reference's permissive base64/bool/default-size edge cases. Cursors remain data, not authorisation tokens.
* Progress fixtures emitted by both bases: absent token, message sanitisation/rune bounds, integer/float validation, large integer total/progress comparisons and errors. Concurrency tests cover request-ID/progress-group cancellation, duplicate-ID cleanup and isolated metadata.
* Registry: 34 Memento operations, twenty tool-surface/model-option combinations, concrete argument schemas and the execute plan schema. These are forward references; they are not claimed as implemented discovery.
* Rust vector helpers: finite/length errors, empty/dimension/zero-norm cases, scalar values and AXPY results.

## End-to-end gates still to build

Run an identical scenario against reference and Go daemons and compare HTTP/JSON/SSE output, accepted Git content, control/index tables, model outputs and final operation state. Inject identical clock/UUID inputs where identity matters. Keep nondeterminism normalisation narrow and reviewed: never remove ACLs, status, revisions, warnings, operation IDs or proposal decisions simply to make a diff pass.

Extend coverage to persistent HTTP sessions, auth-bound retries, notifications/cancellation/progress, malformed requests, byte limits and backpressure. Port all existing proposal/asset/rebase/recovery tests and source regression cases, not only successful routes. Crash testing must kill the process before/after publication and prove original-key reconciliation from disk on restart.

GTE/Needle gates require exact token IDs, model-file validation, layer/intermediate golden tests, finite/shape checks, model outputs and full routing/retrieval corpora. Floating-point tolerances must be explicit and agreed; no hidden re-quantisation or embedding-space mixing. SIMD remains disabled in these baseline tests.

The final gate is the entire parity matrix plus conformance, fuzz/race/fault tests, resource limits, browser behaviour and state-preserving migration/rollback in disposable copies. Only a separately authorised deployment can replace production.
