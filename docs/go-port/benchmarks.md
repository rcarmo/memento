# Go/Python baseline benchmark

The removed reference harness created 1,000 deterministic concepts with cross-links and common lexical terms, then gives both implementations the same Markdown tree. It records five fresh derived-index rebuilds and 100 warm-index lexical searches after ten warmups. Fixture generation and helper process startup are outside measured samples. Both implementations must return 20 results; the corpus SHA-256 identifies accepted input bytes.

Evidence for that historical run is retained in [`../evidence/go-python-models-off-benchmark-2026-09-19.json`](../evidence/go-python-models-off-benchmark-2026-09-19.json). The pure-Go branch no longer carries the Python comparison harness. `make audit` runs model-independent allocation checks; `make performance` adds the pinned real GTE and Needle benchmarks after model preparation.

On this Linux x86-64 host, Python rebuild p50 was **1487.893 ms** and Go rebuild p50 was **1529.048 ms** (Go/Python **1.028×**). Python lexical-search p50 was **2.736 ms** and Go lexical-search p50 was **3.822 ms** (Go/Python **1.397×**). These are local scalar models-off measurements, not deployment SLOs; repeat on target hardware before capacity decisions.
