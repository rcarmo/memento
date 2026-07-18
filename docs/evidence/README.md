# Local validation reports

These reports were generated on 2026-07-17 from a Linux x86_64 development host. Thresholds are bounded regression checks for this environment, not production service-level objectives.

* `load-operational-local.json` -- 250 concepts, 16 workers and 1,000 direct requests, followed by same-base write contention, an idempotent replay storm, proposal concurrency and a backup/restore drill. All scenarios passed.
* `load-http-local.json` -- the bounded direct/operational checks plus 3,010 authenticated Streamable HTTP tool calls over 10 seconds at eight concurrent workers. The HTTP mix was 40% status and 60% search. No HTTP operation failed.
* `load-semantic-local.json` -- 100 concepts and 200 direct requests plus 200 searches through the vendored GTE-small model, Rust FFI and SQLite vector index. Semantic readiness was true and no degradation warning was emitted.

Each JSON document records the Git revision, host and Python information, operation counts, throughput, latency percentiles, errors, invariants and threshold results. `passed=true` means every included scenario and invariant passed.

The Python 3.14 local container rebuild also produced image ID `sha256:2b508ad4e469d272bf9d43559fcbc2e1825f5b31f60c83f1ed2940b457e1726d`, and the image-contained model digest matched `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`. That image ID is a local observation only, not a published immutable digest claim: the Dockerfile currently builds from floating `rust:1-slim` and `python:3.14-slim` bases, so later local rebuilds can legitimately produce a different image ID.

## Reproduction commands

### `load-operational-local.json`

```bash
PYTHONPATH=src .venv/bin/python tools/load_test.py \
  --profile operational \
  --concepts 250 \
  --workers 16 \
  --requests 1000 \
  --output docs/evidence/load-operational-local.json
```

### `load-http-local.json`

Start a local daemon first:

```bash
export MEMENTO_TOKEN_SMITH='replace-me'
export MEMENTO_TOKEN_FLINT='replace-me-too'
memento-serve --config /path/to/config.json serve --host 127.0.0.1 --port 18768
```

Then run:

```bash
PYTHONPATH=src .venv/bin/python tools/load_test.py \
  --profile check \
  --concepts 50 \
  --workers 8 \
  --requests 200 \
  --include-http \
  --http-url http://127.0.0.1:18768/mcp \
  --http-token "$MEMENTO_TOKEN_SMITH" \
  --http-concurrency 8 \
  --duration-seconds 10 \
  --http-status-ratio 40 \
  --http-search-ratio 60 \
  --http-read-ratio 0 \
  --output docs/evidence/load-http-local.json
```

### `load-semantic-local.json`

```bash
PYTHONPATH=src .venv/bin/python tools/load_test.py \
  --profile functional \
  --concepts 100 \
  --workers 8 \
  --requests 200 \
  --semantic-enabled \
  --include-semantic \
  --output docs/evidence/load-semantic-local.json
```

Heavier or deployed runs should write new reports rather than replacing these local baselines. See [`../load-testing.md`](../load-testing.md).
