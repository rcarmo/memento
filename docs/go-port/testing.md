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

The race detector's build needs cgo on the test runner; distributed binaries and normal test builds use `CGO_ENABLED=0`. Check runtime dependencies in packaging tests. Fuzzing uses a fixed 10,000 generated executions per target with a separate 120-second safety timeout; this avoids a wall-clock shutdown race observed on a loaded native CI runner. Longer scheduled corpus runs are still required. Fuzzing starts with small bounded runs for CI and needs longer scheduled corpus runs as parsers/transports are ported. No untrusted fixture can trigger unbounded allocation or leak the daemon's resources.

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
* Actual local sync/async HTTP server scenarios: 41 requests compare statuses, JSON/text bodies and selected headers, normalising only random session IDs. In-process and real-socket Go tests exercise event streams, keepalive, duplicate-stream rejection, principal/version binding, queue cap, TTL/deletion, hooks and I/O failures. Raw HTTP parser details, configured connection lifetimes and the complete upstream transport regression suite remain open.
* HTTP Accept quality/wildcard and content-type parsing, singleton headers, all status reason phrases from 99 through 600, and response bounds/injection checks. Runtime-typed Go responses deliberately exclude impossible Python dynamic type combinations; transport parsers must reject them at decoding.
* Tool fixtures: 380 sync/async result-format/schema-subset cases and 40 call/coercion/error cases, with dynamic discovery and annotations. Text embedded inside results preserves dict insertion order and Python number formatting. Native Go structs must be explicitly normalised; nonfinite structured JSON, reflection inference and malformed dynamic schema corner cases remain gaps. Handler panics are contained and remote-redacted. Source coercion is intentionally not strict service validation; Memento's typed validators remain required.
* Completion/config/logging scenarios compare both bases: reference resolution, parameter enums, provider output/total/hasMore validation, prefix/dedup limits, source initialise capabilities and log redaction. Session-targeted notification delivery is still a transport gate.
* Resource/prompt fixtures cover dynamic metadata, pagination, static/template reads, binary and nested content, missing URIs, handler errors/cancellation, subscriptions, prompt defaults/coercion and discovery-versus-call docstring metadata. Notification delivery and transport-authenticated session ownership remain separate gates.
* Request-dispatch fixtures emitted independently by both sync and async uMCP bases with the same overridden test method: malformed input, validation ordering, metadata, integer IDs, notification and cancellation results. These isolate dispatch from method implementation and do not prove discovery/tool execution.
* URL/Origin fixtures compare raw urllib-style components, default paths, exact allowlists/authority, loopback/ports, IPv6/IPvFuture errors, Unicode delimiters and error text. `golang.org/x/text/unicode/norm` supplies CGO-free NFKC; the transport must still enforce wire-size/auth rules.
* Actual sync/async stdio loops driven by byte streams: blank and final lines, notifications, invalid UTF-8 replacement and large requests. Expected responses compare JSON objects, not insignificant whitespace. Caller-owned blocked readers still require caller closure to interrupt; the source loop is sequential.
* Pagination fixtures emitted by both bases: principal/list identity binding, exact opaque cursor bytes, Unicode labels, large offsets/sizes, invalid params and the reference's permissive base64/bool/default-size edge cases. Cursors remain data, not authorisation tokens.
* Progress fixtures emitted by both bases: absent token, message sanitisation/rune bounds, integer/float validation, large integer total/progress comparisons and errors. Concurrency tests cover request-ID/progress-group cancellation, duplicate-ID cleanup and isolated metadata.
* Registry: 34 Memento operations, twenty tool-surface/model-option combinations, concrete argument schemas and the execute plan schema. These are forward references; they are not claimed as implemented discovery.
* Rust vector helpers: finite/length errors, empty/dimension/zero-norm cases, scalar values and AXPY results.
* Embedding framing: Rust-produced successful request/response headers and scalar payloads, short reads, invalid arguments, EOF, write/flush failures, oversized headers and nonfinite output. Test-only `go/oracle/check_worker.py` exercises the unchanged Python client against the Go executable and real public model. Malformed serde diagnostics and duplicate known fields remain explicitly incomplete, as do service worker/deadline/resource integration.
* GTE: Rust-generated tokenizer fixtures across limits/Unicode, synthetic zero/one/two-layer model outputs and checkpoint sequences. The original Go tokenizer's bytewise UNK behaviour differs from Memento's Unicode fix; default Go-port tokenisation preserves the latter.
* `make -C go model-test GTE_MODEL_PATH=/path/to/gte-small.gtemodel` is a required native CI step, not an optional skipped release gate. It verifies the model SHA, exact token IDs and five real-model outputs against Rust. Current explicit exploratory numeric gates are max absolute error <=1e-5, cosine >=0.999999 and unit norm error <=1e-5; observed local max error is <=1.2e-7. These are not bit-identical arithmetic or a complete retrieval corpus and need review before production compatibility is signed off.

Regenerate the real-model reference only deliberately with the digest-pinned file:

```sh
cd go
CARGO_TARGET_DIR=../build/go/oracle-target cargo run --locked --release \
  --manifest-path oracle/rust-gte/Cargo.toml -- real /path/to/gte-small.gtemodel \
  > testdata/parity/gte-real.json
```

The public model is fetched/verified by existing test tooling, never at Go runtime. Synthetic parser tests include every truncated offset, invalid dimensions/UTF-8 and oversize header guards. A complete malformed-error-text matrix and mmap/resource tests remain required.

## End-to-end gates still to build

Run an identical scenario against reference and Go daemons and compare HTTP/JSON/SSE output, accepted Git content, control/index tables, model outputs and final operation state. Inject identical clock/UUID inputs where identity matters. Keep nondeterminism normalisation narrow and reviewed: never remove ACLs, status, revisions, warnings, operation IDs or proposal decisions simply to make a diff pass.

Extend coverage to persistent HTTP sessions, auth-bound retries, notifications/cancellation/progress, malformed requests, byte limits and backpressure. Port all existing proposal/asset/rebase/recovery tests and source regression cases, not only successful routes. Crash testing must kill the process before/after publication and prove original-key reconciliation from disk on restart.

`make -C go model-test` now requires both `GTE_MODEL_PATH` and `NEEDLE_MODEL_PATH`; the Needle fixture validates the 31 tensor shapes/raw bytes/expanded FP32 hashes and all 8192 vocabulary entries from the real NDL1 model. Regenerate with `cargo run --locked --release --manifest-path go/oracle/rust-needle/Cargo.toml -- /path/to/memento-router.ndl`. Add `NEEDLE_TOKENIZER_PATH` for the now-required tokenizer gate: 15 encode/decode inputs and all 8192 single-token decodes are compared with Rust. Always-on tests compare 288 synthetic BPE/Unigram normalisation/byte-fallback cases against sentencepiece-rust 0.1.1. The model test also compares five real scalar generation outputs and checkpoint sequences. `make -C go corpus-test` requires the pinned model/tokenizer and compares 360 complete output strings with freshly generated Rust oracle results from corpus SHA `9ffeb303574fa6bd24718adc42c7a3d8c4632e3cf78685d14886c7b24b2ddca9`. The first local scalar run matched 360/360 in 1408.79 seconds; native amd64/ARM64 CI run 35291886039 subsequently passed the full gate on both architectures. Native amd64/ARM64 CI runs this gate without enabling SIMD. A match is behavioural parity, not a claim that the model's output is correct or authorised for execution; Python service validation still needs porting. Source and port reject malformed files, but their JSON/UTF-8 diagnostic wording has not yet been made fully identical.

GTE/Needle gates require exact token IDs, model-file validation, layer/intermediate golden tests, finite/shape checks, model outputs and full routing/retrieval corpora. Floating-point tolerances must be explicit and agreed; no hidden re-quantisation or embedding-space mixing. SIMD remains disabled in these baseline tests.

The final gate is the entire parity matrix plus conformance, fuzz/race/fault tests, resource limits, browser behaviour and state-preserving migration/rollback in disposable copies. Only a separately authorised deployment can replace production.
