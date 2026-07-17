# Local acceptance evidence

These reports were generated on 2026-07-17 from a Linux x86_64 development host. Thresholds are bounded regression checks for this environment, not production service-level objectives.

* `load-operational-local.json` -- 250 concepts, 16 workers and 1,000 direct requests, followed by same-base write contention, an idempotent replay storm, proposal concurrency and a backup/restore drill. All scenarios passed.
* `load-http-local.json` -- the bounded direct/operational checks plus 3,010 authenticated Streamable HTTP tool calls over 10 seconds at eight concurrent workers. The HTTP mix was 40% status and 60% search. No HTTP operation failed.
* `load-semantic-local.json` -- 100 concepts and 200 direct requests plus 200 searches through the vendored GTE-small model, Rust FFI and SQLite vector index. Semantic readiness was true and no degradation warning was emitted.

Each JSON document records the Git revision available when the harness started, host and Python information, operation counts, throughput, p50/p95/p99/max latency, errors, invariants and threshold decisions. The final local container build produced image ID `sha256:7826f59c4f2fbd9f5f6f5f3e352f058a749acdd1136006a0b829e09f9a3a0b36`; the image model digest matched `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`. A report only passes when every included scenario and invariant passes.

Reproduce the bounded gate with:

```bash
make load-check
```

Heavier or deployed runs should write new reports rather than replacing these local baselines. See [`../load-testing.md`](../load-testing.md).
