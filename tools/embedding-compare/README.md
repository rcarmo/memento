# Offline embedding comparison

This host-only tool compares native GTE-small and quantised EmbeddingGemma. It does not import into the service or change production configuration.

## Corpora

- `corpus.json`: original 18 documents and 16 queries. Keep this historical diagnostic unchanged.
- `corpus-expanded.json`: authored on 13 November 2026 before running either model. It contains 60 English documents in 20 topic groups and 80 queries: 40 English questions and 40 translations, eight each in Portuguese, Spanish, French, German and Japanese. Each query has one relevant document and two named, same-topic distractors. Retrieval searches all 60 documents, not just the topic group. Documents and queries use a fixed shuffled order to avoid grouping/tie-order advantages.

The expanded set covers recovery, DNS, access controls, credentials, certificates, storage, containers, transactions, indexes, queues, time, observability, memory, search, billing, contracts, deployment, networking, files and cancellation. The questions require distinguishing similar procedures rather than matching unrelated subjects. Labels and wording are frozen before rankings are observed. Do not repair a failure by changing its relevance label.

Both sets are synthetic diagnostics, not external benchmarks or random samples of production queries. Translations share intent with their English counterparts; 80 questions are not 80 independent observations. Named distractors are intended hard negatives, not a claim that either model actually finds them difficult. Relevance is binary, with exactly one answer per question; Recall@k is therefore the hit rate and nDCG@10 is reciprocal log-discounted rank for that single answer. Also report MRR, language slices and individual failures. Any confidence interval must account for shared topic/translation structure and does not establish production generalisation.

## Running

Source `tools/project-env.sh` before build or test commands. Put binaries, result JSON, profiles and disposable logs under `$PROJECT_TMP_ROOT/runs/<purpose>/`. Model files remain intentional assets under `models/`; the Gemma manifest pins the checkpoint revision and integrity hash.

```sh
. tools/project-env.sh
run="$PROJECT_TMP_ROOT/runs/gte-gemma-refresh"
mkdir -p "$run"
CGO_ENABLED=0 go build -o "$run/compare" ./tools/embedding-compare
# One core, serial inference for both engines.
taskset -c 0 "$run/compare" -engine gte -model models/gte/gte-small.gtemodel \
  -corpus tools/embedding-compare/corpus-expanded.json -mode performance \
  -threads 1 -revision "$(git rev-parse HEAD)" > "$run/gte-1.json"
# Four-core scheduler budget, opt-in Gemma projection parallelism.
taskset -c 0-3 "$run/compare" -engine gemma \
  -model models/embeddinggemma/embeddinggemma-2-UD-Q4_K_XL.gguf \
  -corpus tools/embedding-compare/corpus-expanded.json -mode performance \
  -threads 4 -gemma-workers 4 -revision "$(git rev-parse HEAD)" > "$run/gemma-4.json"
```

Run the corresponding GTE and Gemma commands in both orders, sequentially rather than concurrently. Report one-core results separately from four-core-budget results. GTE's inference path remains serial: equal scheduler ceilings do not mean equal CPU consumption or equal batch throughput. The report includes Linux process user/system CPU time across all threads, actual allowed CPUs, per-call allocations, warm distributions, and heap/RSS measurements. Unsupported CPU accounting is explicit. CPU time for the measured repeat block is averaged per request; wall time retains each sample. Load/first inference are separate from warm timing. Peak RSS includes model loading and cannot be called steady state.

Use `-mode retrieval` for the frozen retrieval set. Gemma is evaluated at 768, 512, 384, 256 and 128 dimensions using cosine on each prefix (equivalent to renormalising it). GTE remains at 384. Each engine uses its own tokenizer and native input contract; GTE prepends titles and Gemma uses query/document role prefixes and titles. Record token counts rather than assuming equal tokenisation.

`bun tools/embedding-compare/summarise.mjs "$run"` analyses the named `gte-1-a/b`, `gemma-1-a/b`, `gte-4-a/b`, `gemma-4-a/b` and `gte-quality`/`gemma-quality` JSON files. It pools timing samples, shows language slices and paired outcomes, and computes a deterministic 10,000-resample topic-cluster percentile interval for the top-1 difference. Its machine-readable output is disposable analysis data, not a second archive. The interval does not correct synthetic selection bias or multiple dimension exploration.

Optional `-profiles "$run/profiles"` captures CPU and heap/allocation profiles for a diagnostic run. Do not mix profiled latency into the primary unprofiled comparison. Inspect CPU, `alloc_space` and `alloc_objects`; remove profiles, matching binaries, raw result JSON and logs immediately after analysis. Keep only the corpus, harness and concise measured report. Hashes identify model/corpus provenance, never embedding-output acceptance.
