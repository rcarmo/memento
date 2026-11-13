# Gemma within twice GTE latency on this host

Opt-in four-worker Gemma met the requested ≤2× fresh GTE-small median latency on all three fixed workloads in two opposite-order runs and a profiled verification. It uses up to four CPU cores within one request. Serial operation remains the default; production GTE was untouched.

## Acceptance comparison

Source baseline `fc6bcf1` plus this change. Linux amd64, Intel Core i7-12700, Go 1.27.1, CGO disabled, AVX2/FMA. Both executables had affinity `0,1,2,3` and `GOMAXPROCS=4`. Each model processes one request at a time. GTE's existing engine stays serial within a request; Gemma uses four cooperating projection workers, including the caller. This is a wall-latency comparison with the same available CPU budget, **not** equal CPU consumption or a single-core improvement.

Checkpoint digests, fixed authored `tools/embedding-compare/corpus.json`, prompts, source content, token counts, warm-up and sample counts are unchanged. Both return owned normalised vectors. Model load, worker IPC and index lookup are excluded. Filesystem pages were warm; the host was not otherwise reserved.

| Workload | Tokens GTE/Gemma | GTE median | Gemma median | Gemma/GTE | Reverse-order ratio |
|---|---:|---:|---:|---:|---:|
| Query | 8/15 | 15.72 ms | 27.99 ms | 1.78× | 1.60× |
| Paragraph | 69/69 | 138.52 ms | 160.22 ms | 1.16× | 1.23× |
| Longer chunk | 262/254 | 860.73 ms | 591.59 ms | 0.69× | 0.75× |

First run: GTE then Gemma, seven/five/three samples. Reverse order: Gemma medians 27.61/175.27/660.29 ms; GTE 17.30/142.94/877.07 ms. Every workload median passed the ≤2× target in both rounds. The target applies to these warmed workload medians, not every request, percentile, input length or host state.

Final profiled full-corpus verification measured Gemma 27.33/192.11/631.61 ms versus GTE 16.56/153.47/872.01 ms, again below 2× on all three. The preceding serial-only comparison on the same current source measured 67.43/279.14/1,048.23 ms versus GTE 15.05/134.43/817.93 ms, ratios 4.48×/2.08×/1.28×. Serial Gemma does not satisfy the short-query target.

## Implementation and resource cost

`Workspace.Workers` opts into 2–8 bounded workers; zero/one keep serial inference. The CLI exposes `-workers`, default 1. Workers are scoped to the embedding request, never persistent background jobs, and are joined before returning or releasing the model mapping read lock.

Each projection packs its immutable input once, partitions independent output-row panels across workers, and waits for all results before the next operation. Each worker owns decoded/sums scratch, so concurrent output rows do not race. The caller processes one panel partition. Atomic generation/completion handshakes reduce repeated channel wakeups; idle loops yield every 128 polls to avoid oversubscription deadlock. Context errors drain active work; Close waits for joined workers.

Full row panels now write only actual token outputs, including non-multiple-of-eight token tails. Checked output extents use actual valid tokens; padded token stores are suppressed and assembly row-offset registers are restored before the next token panel. This removes unnecessary buffered result copies without changing coefficient arithmetic or reduction order.

Polling deliberately spends CPU during attention/other serial phases as well as parallel projections. In the paired full performance process, Gemma took 3.96 s wall / 14.89 s user / 0.07 s system, about 3.78 CPU-seconds per wall second. GTE took 4.56 s wall / 4.47 s user / 0.10 s system, about 1.00. Gemma therefore consumes substantially more CPU energy/capacity to meet the latency target. Multiple simultaneous four-worker requests may degrade host throughput; choose bounded request concurrency and explicit scheduler allowance. This mode is not enabled in production.

Four workers reserve four 64 KiB decoded panels and four 2 KiB sums arrays (~264 KiB), plus request-scoped coordination/goroutine state. The existing main workspace packing buffers remain. Threads exit after each request; scratch retains the configured high-water count. Request-scoped workers add small allocations: measured query ~23,769 bytes/~224 objects, paragraph ~141,550/~1,472, longer chunk 475,512/5,229, compared with serial 21,464/217, 140,312/1,466 and 474,584/5,224. Peak RSS was about 164 MiB for Gemma and 263 MiB for GTE in the paired performance runs. Maximum-context/concurrent/NAS resource use is not qualified.

CPU profiles ranked matrix arithmetic and polling/scheduler yield work. The final Gemma capture had 27.55 s CPU samples over 6.93 s wall; `procyieldAsm` and direct-scatter matrix arithmetic each accounted for about 29% flat. CPU, alloc_space and alloc_objects were analysed. Sampled total allocated space was GTE 258.90 MiB / Gemma 85.35 MiB, dominated by model loading/tokenizer parsing; Gemma sampled 794,538 allocation objects. These startup profiles do not replace per-request allocation measurements.

## Rejected experiments

Channel-only row partitioning improved longer inputs but left the query above 2×. Larger per-token layer scheduling, grouped projections, always-yield atomic polling, caller-only channel scheduling, panel realignment, matrix unrolling and a wider short-input kernel did not provide stable accepted query performance. They were reverted. The retained combination is bounded polling workers including the caller, original matrix arithmetic, and checked actual-token tail stores. An early tail-store probe clobbered row-offset registers across token panels; tests caught it and the final assembly restores them per panel. No failed probe is counted as passed verification.

## Numerical, lifecycle and retrieval checks

All 11 independent reference vectors pass unchanged limits: RMS `3e-7`, max absolute `1e-6`, cosine at least `1 - 1e-9`. Parallel and serial outputs also agree. No full-weight FP32 expansion, activation quantisation, prompt change or regenerated reference fixture was introduced. Generated-output hashes have no acceptance role.

Passing verification:

- Parallel reference suite, including multilingual/special text, Matryoshka prefixes and 1,028-token attention window.
- Race tests for parallel requests, worker buffers, cancellation and model Close. A deterministic blocked context proves Close waits while published work is in flight; oversubscription with eight workers and `GOMAXPROCS=1` completes; an error generation followed by a successful generation reuses worker state correctly.
- Partial-row/partial-token projection tests and guard pages for suppressed padded stores. Input/output extents and worker counts are checked. Independent kernel oracles remain intact.
- Explicit Go 1.26.6 real-checkpoint package/CLI/comparison tests; AVX/FMA-disabled fallback/reference suite; Linux ARM64/RISC-V cross-compilation; vet; `CGO_ENABLED=0 go test ./... -count=1`. Non-amd64 parallelism falls back to serial and those CPUs were not executed.
- Read-only review found no synchronisation or mapping-lifetime blocker; its in-flight cancellation, oversubscription and reuse coverage suggestions were added and passed.

The fixed retrieval sanity set remains GTE 14/16 top results, Gemma 16/16 at 768/512/256 dimensions and 15/16 at 128. Embedding all 18 documents and 16 queries took 2.83 s GTE / 2.99 s Gemma in the final profiled run. The small authored set does not establish production-corpus relevance quality.

One long combined verification command timed out after targeted/fallback checks; it was not counted as a successful full repository run. The separately rerun repository suite passed. Raw profiles, binaries, discarded source probes and disposable logs/JSON were removed after analysis, including the earlier serial comparison captures. Source, pinned models, fixed independent fixture and this report remain.

## Use

```sh
. tools/project-env.sh
GOMAXPROCS=4 go run ./cmd/memento-gemma-embed \
  -model models/embeddinggemma/embeddinggemma-2-UD-Q4_K_XL.gguf \
  -workers 4 -role query -text 'Where is the project documentation?'
# Reproduce the available-budget comparison on this host:
# tools/embedding-compare: -threads 4 -gemma-workers 4, affinity 0,1,2,3
```

For a constrained/oversubscribed host, keep `-workers 1` unless a measured CPU budget supports the parallel mode. Production deployment and NAS evaluation require a separate decision.
