# Load testing

Memento now includes a repository-owned load harness at [`tools/load_test.py`](../tools/load_test.py). It exercises the Python service directly in temporary repositories and can also drive a running authenticated Streamable HTTP endpoint with stdlib JSON-RPC requests.

The harness is intentionally local-first:

* it creates temporary repositories and control databases;
* it never mutates canonical test fixtures in place;
* it emits a JSON report for review and CI artifacts;
* its thresholds are **local development checks**, not universal SLOs.

## Scenarios

The harness currently covers:

* direct functional read/search load with configurable concept count, worker count and request count;
* semantic query load using `search_mode="semantic"`, degrading gracefully when vendored GTE/FFI support is unavailable;
* concurrent write contention from the same base revision, expecting exactly one success and the rest conflicts;
* idempotent replay storms, expecting one recorded operation/commit result and replay for the rest;
* concurrent proposal create/list/read activity;
* timed backup/restore drills in temporary state roots;
* optional HTTP load against a running authenticated Streamable HTTP endpoint using JSON-RPC `initialize` and `tools/call`.

Graceful shutdown and longer soak runs are intentionally left to shell wrappers or external orchestration so the in-repo harness stays small and deterministic.

## Report format

Each run writes a JSON report containing:

* environment details and current Git revision;
* per-scenario counts and throughput;
* p50/p95/p99/max latency;
* error summaries;
* invariant failures;
* threshold checks and overall pass/fail.

The tests in [`tests/test_load_harness.py`](../tests/test_load_harness.py) validate report structure, percentile calculations and scenario invariants. They do **not** assert performance numbers. Reviewed local reports live under [`docs/evidence/`](evidence/README.md).

## Usage

Install development dependencies first:

```bash
make install-dev
```

Run bounded local checks:

```bash
make load-check
```

Run the functional-only profile:

```bash
make load-functional
```

Run the broader operational profile:

```bash
make load-operational
```

All three targets use bounded defaults intended to be CI-friendly. For heavier ad hoc local runs, invoke the harness directly:

```bash
PYTHONPATH=src .venv/bin/python tools/load_test.py \
  --profile operational \
  --concepts 100 \
  --workers 16 \
  --requests 500 \
  --output build/load-heavy.json
```

## Optional HTTP scenario

To exercise a running authenticated Streamable HTTP deployment, provide the endpoint and bearer token explicitly:

```bash
PYTHONPATH=src .venv/bin/python tools/load_test.py \
  --profile check \
  --include-http \
  --http-url http://127.0.0.1:8000/mcp \
  --http-token "$MEMENTO_TOKEN_SMITH" \
  --http-concurrency 8 \
  --duration-seconds 10 \
  --output build/load-http.json
```

The HTTP mix is configurable with:

* `--http-status-ratio`
* `--http-search-ratio`
* `--http-read-ratio`

Those ratios are interpreted as a local traffic mix, not as a production traffic model.
