# Go/Python baseline benchmark

Run from the repository root after building the helper:

```sh
(cd go && CGO_ENABLED=0 go build -o ../build/go/memento-benchmark-go ./cmd/memento-benchmark-go)
VENV=/path/to/memento/.venv "$VENV/bin/python" tools/run_comparative_benchmark.py 1000
```

The runner creates 1,000 deterministic concepts with cross-links and common lexical terms, then gives both implementations the same Markdown tree. It records five fresh derived-index rebuilds and 100 warm-index lexical searches after ten warmups. Fixture generation and helper process startup are outside measured samples. Both implementations must return 20 results; the corpus SHA-256 identifies accepted input bytes.

Evidence for the current host is in [`../evidence/go-python-models-off-benchmark-2026-09-19.json`](../evidence/go-python-models-off-benchmark-2026-09-19.json).

On this Linux x86-64 host, Python rebuild p50 was **1487.893 ms** and Go rebuild p50 was **1529.048 ms** (Go/Python **1.028×**). Python lexical-search p50 was **2.736 ms** and Go lexical-search p50 was **3.822 ms** (Go/Python **1.397×**). These are local scalar models-off measurements, not deployment SLOs; repeat on target hardware before capacity decisions.
