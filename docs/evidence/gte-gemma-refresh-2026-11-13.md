# GTE-small and EmbeddingGemma: refreshed host comparison

## Decision

Keep production GTE unchanged. Gemma deserves a representative multilingual trial, not an automatic model replacement. On this larger synthetic diagnostic it improves multilingual retrieval substantially, while English top-1 is similar. Serial Gemma still costs more CPU; four-worker Gemma can reduce long-chunk latency, but the original spin-wait implementation wastes CPU during serial work.

A bounded follow-up replaces projection-team polling with parked workers. It preserves the model, numerical kernels, cancellation/join contracts and serial default. The measured trade-off is lower total CPU with higher four-worker latency. This explicitly supersedes the earlier “within 2× GTE on every input” wall-time result for the new parked implementation: short queries are approximately 2.2× GTE, not within 2×. Do not retain busy polling merely to preserve that old headline.

## Reproduction and limits

- Source: `a5a01e996c02bd3a4d331a803e1e68a619e2adb6`, built on native Gemma `c5bfdb3`; parked-worker candidate changes only `internal/embeddinggemma/parallel.go`. Source/corpus committed before inference. No changes to production, NAS configuration, live indexes or GTE implementation.
- Host: Linux amd64 VM on Intel Core i7-12700, six visible separate cores, AVX2/FMA, Go 1.27.1, CGO disabled for measured binaries. CPU affinity `0`, scheduler budget 1 for serial; affinity `0-3`, budget 4 for parallel. GTE inference remains serial under either budget. These are not NAS measurements or concurrent-request throughput measurements.
- Model: original GTE-small `models/gte/gte-small.gtemodel`; Gemma `embeddinggemma-2-UD-Q4_K_XL.gguf`, pinned revision `031f0d4b35536f69ab3509d4893c923264fcf253`, SHA-256 `ea905fd08e8061db77a0031cbb5096d459d7cfd2bdd0ad2ba66e483f16719493`. Restored missing local checkpoint and verified integrity before loading. This is a durable model asset, not a retained expanded-weight cache.
- New corpus `tools/embedding-compare/corpus-expanded.json`, SHA-256 `e7c56b400ac4374bb9d72e87142786778b84f486a517228af6c074e2b09baa7f`: 60 English documents, 80 queries, 20 infrastructure/delivery topics. Forty English questions and 40 translations, eight each in Portuguese, Spanish, French, German and Japanese. Each query has a single labelled answer plus two intended same-topic hard negatives; search considers every document. Fixed shuffled order, frozen labels/texts. Original 18-document diagnostic unchanged.
- The new set is synthetic and Memento-adjacent, not an external benchmark or a random production sample. Translations are paired with English intents. The 80 observations are correlated. Five dimension settings are exploratory; 512 dimensions is a candidate discovered here, not a preselected winner. Revalidate it on unseen data before selecting it.
- Both engines use native tokenizer/input contracts, including document titles and Gemma query/document prefixes. Token counts are GTE/Gemma 8/15 for query, 69/69 for paragraph and 262/254 for chunk. Input texts are identical; tokenisation is not assumed identical.
- Warm timings: sequential AB then BA process order, 17 query, 13 paragraph and 9 chunk repetitions per process (34/26/18 pooled observations). Each process loads and warms independently. No concurrent benchmark processes. CPU is process user+system time across all OS threads, averaged over the measured block; wall time is median of individual requests. Profiling is separate and excluded from primary timings. RSS includes mapped pages; heap alone is insufficient.

## Warm timing and CPU cost

Each cell is **median wall ms / mean process CPU ms**. Pooled medians are descriptive, not formal significance estimates.

| Workload | GTE, 1 core | Gemma, 1 core | GTE, 4-core budget | Gemma, 4 workers, polling | Gemma, 4 workers, parked |
|---|---:|---:|---:|---:|---:|
| Query | 17.75 / 17.87 | 53.56 / 53.68 | 15.62 / 15.88 | 22.44 / 92.48 | 34.12 / 65.87 |
| Paragraph | 142.88 / 139.63 | 235.73 / 235.45 | 139.85 / 141.37 | 136.29 / 550.34 | 155.51 / 256.61 |
| Longer chunk | 809.27 / 815.43 | 866.00 / 866.26 | 807.19 / 829.35 | 465.28 / 1866.77 | 548.42 / 947.66 |

Serial Gemma/GTE wall ratios are 3.02×, 1.65× and 1.07×. Parked four-worker Gemma/GTE four-core-budget wall ratios are 2.18×, 1.11× and 0.68×; CPU ratios are 4.15×, 1.82× and 1.14×. Four workers are not free capacity: even parked Gemma spends more CPU than serial GTE.

Parking versus polling reduces mean CPU by 26.61/293.73/919.11 ms per query/paragraph/chunk (28.8%/53.4%/49.2%), at wall-median increases of 11.68/19.21/83.14 ms (52.0%/14.1%/17.9%). The short-query wake-up cost is material; serial default and explicit parallel opt-in remain appropriate. The parked path still reduces wall time versus serial Gemma by about 36%/34%/37%, while CPU rises by about 23%/9%/9%.

Primary pooled p95 wall ms, query/paragraph/chunk:

- GTE serial: 35.50/242.33/1168.69; GTE four-core budget: 18.33/158.54/969.07.
- Gemma serial: 56.37/236.82/876.90; polling four-worker: 30.42/153.71/518.51.
- Parked four-worker: 38.22/166.05/564.49.

GTE has more timing variability on this host. Serial query process medians span 16.12–19.61 ms and paragraph medians 132.08–182.06 ms. Treat small paragraph differences as noise-sensitive, not a decisive speed win. Gemma serial process medians are 53.43–53.62/235.62–235.73/865.48–867.47 ms. Parked process medians are 32.39–35.70/152.91–157.85/539.00–549.67 ms.

An additional ABBA confirmation (polling → parked → parked → polling) reproduced the trade-off. Pooling all four processes per variant gives 68/52/36 observations: polling wall medians 22.39/136.32/467.15 ms and CPU 91.74/549.97/1863.99 ms; parked wall medians 34.28/159.29/553.72 ms and CPU 66.30/261.76/965.66 ms. CPU falls 27.7%/52.4%/48.2% while wall medians rise 53.1%/16.9%/18.5%. Pooled confirmation p95s are polling 27.54/148.94/488.77 ms versus parked 42.40/176.86/597.64 ms. This confirms a CPU-efficiency increment with a latency cost, not a pure speed-up. Serial post-change confirmation is 53.93/239.14/867.25 ms with exactly the same per-call allocation counts/bytes as before (within ordinary timing noise).

## Memory and allocations

Historical and fresh measurements must not be conflated. Fresh peak RSS is about 263.6 MiB for GTE and 164.3–164.6 MiB for Gemma; parked Gemma peaks at 163.8 MiB. This includes load transients. Steady long-chunk RSS is about 147 MiB for GTE and 162–163 MiB for Gemma: Gemma's smaller peak is not a smaller steady resident footprint. GTE initially holds a checkpoint buffer plus decoded weights, explaining the loading peak. No claim is made that this baseline startup cost was newly optimised.

Post-load, post-GC Go heap is about 129 MiB for GTE and 53 MiB for Gemma. Gemma maps weights, so that comparison omits resident mapped pages. Query RSS can still reflect GTE's recent loading peak before scavenging; it is not a steady-state memory comparison.

Per-request allocation bytes/objects for Gemma serial are 21,464/217, 140,312/1,466 and 474,584/5,224. GTE uses roughly 1.6 KiB and 3/7/13 objects per query/paragraph/chunk call. Parking adds approximately 0.3–0.5 KiB and 3–4 objects versus polling for request-owned channel/team setup; both parallel paths already allocate more request setup than serial, and bounded worker buffers remain reusable. This small, explicit allocation increase is accepted for the measured CPU reduction, not described as an allocation optimisation.

## Retrieval

Single-relevant-document labels make Recall@k equivalent to hit rate. nDCG@10 uses binary relevance. Gemma prefix cosine recomputes norms; shorter vectors are not compared as unnormalised dot products.

| Engine/dimensions | Top-1 | Recall@3 | Recall@10 | MRR | nDCG@10 |
|---|---:|---:|---:|---:|---:|
| GTE / 384 | 51/80 (63.75%) | 73.75% | 83.75% | .70539 | .73273 |
| Gemma / 768 | 71/80 (88.75%) | 100% | 100% | .94167 | .95684 |
| Gemma / 512 | 73/80 (91.25%) | 100% | 100% | .95625 | .96771 |
| Gemma / 384 | 70/80 (87.50%) | 100% | 100% | .93542 | .95223 |
| Gemma / 256 | 68/80 (85.00%) | 100% | 100% | .92083 | .94137 |
| Gemma / 128 | 63/80 (78.75%) | 95% | 100% | .87408 | .90559 |

| Language | GTE / 384 top-1 | Gemma / 512 top-1 |
|---|---:|---:|
| English | 37/40 | 37/40 |
| Portuguese | 2/8 | 6/8 |
| Spanish | 3/8 | 7/8 |
| French | 3/8 | 7/8 |
| German | 5/8 | 8/8 |
| Japanese | 1/8 | 8/8 |
| All non-English | 14/40 | 36/40 |

Full 768-dimensional Gemma scores 36/40 English and 35/40 non-English. The headline quality gain is cross-language retrieval, not demonstrated English superiority. At 512 dimensions, Gemma fixes 24 GTE top-1 failures but loses two GTE successes; 49 are correct for both and five wrong for both. The paired top-1 gain is 27.5 percentage points. A deterministic 10,000-resample topic-cluster percentile bootstrap gives a descriptive 95% interval of +18.75 to +36.25 points. This accounts for within-topic translations but not synthetic-set selection bias or multiple dimension exploration; it is not a production confidence claim.

All seven Gemma/512 failures rank the labelled answer second. They concern ambiguous neighbouring concepts rather than unbounded ranking failure:

- `oauth-b-fr`: compromised refresh-token revocation confused with normal renewal.
- `oauth-a-en`: normal access-token renewal confused with compromised-token handling.
- `replication-b-es`: standby promotion confused with read-replica routing.
- `http-auth-b-pt`: authenticated-but-forbidden confused with missing/expired authentication.
- `network-b-pt`, `network-b-en`: TCP timeout confused with immediate connection refusal.
- `search-b-en`: query/document prefix handling confused with embedding-model migration.

Labels and texts were not edited after observing these failures. Some document pairs have intentionally shared language; human-labelled real examples are the next check, not post-hoc diagnostic repair.

Embedding all 140 retrieval texts costs GTE serial 10.06 s wall / 10.06 s CPU; Gemma polling four-worker 8.22 s / 32.70 s; Gemma parked four-worker 10.91 s / 21.27 s. Parking saves 11.44 CPU-seconds (35.0%) with 2.69 s extra wall time (32.8%). This is a sequential corpus pass, not concurrent throughput or end-to-end search latency. Prefix dimension reduction saves vector storage, not transformer inference: 512 float32 dimensions use 2 KiB/vector, 33% more than GTE/384; Gemma/256 uses 1 KiB, 33% less than GTE/384.

## Profiles and bounded tuning

Inspected CPU flat/cumulative and heap `alloc_space`/`alloc_objects` for GTE serial/four-core-budget, Gemma serial/four-worker and parked four-worker. Captures contain real samples, not missing/empty evidence. Profiles include load allocations but CPU capture starts after first inference; no claim that live heap alone measures allocation history.

- GTE CPU is dominated by existing `forwardInto` (~39% flat), AVX2 row dots (~35–36%) and exponentiation (~11%). Allocation bytes are predominantly checkpoint read/decode (~97–98% of ~261 MB), with token/vocabulary construction prominent in object counts. Warm request allocation remains small. Startup streaming and GTE kernel changes are outside this comparison; no new startup or GTE performance regression was introduced.
- Gemma serial CPU concentrates in `wideScatterAVX2` (~58% flat), with activation, quant decode and attention following. This is useful numerical work, not repeated avoidable orchestration. No further numerical micro-optimisation was attempted here.
- Original four-worker CPU has `runtime.goschedImpl` ~45.9% cumulative and `runtime.procyieldAsm` ~29.8% flat: workers repeatedly yield/spin while the caller runs packing/attention/other serial work. Equivalent-workload profile: 7.32 s elapsed / 28.89 s CPU samples.
- Parked workers use request-owned buffered wake/completion channels. Publication and completion retain happens-before ordering; every published job drains before scratch reuse, cancellation return or close. Equivalent profile: 8.67 s elapsed / 14.86 s CPU samples. AVX2 projection becomes ~50% flat; attention and activation follow. Scheduler spinning is no longer a top outlier. Sampling noise and profiled elapsed overhead are excluded from primary latency tables.
- Gemma allocation profiles are dominated by GGUF metadata parsing and tokenizer construction (~84–90 MB sampled total; tokenizer ~44–47 MB cumulative). The channel path adds bounded request setup allocations, not per-panel allocation churn. Existing substantial tokenizer/load cost is disclosed rather than claimed fixed. It is not a new long-lived inference leak.

## Validation and next step

Frozen-corpus and malformed-input tests, ranking/nDCG cutoff and monolingual reporting tests pass. Linux all-thread CPU accounting is explicit; unsupported platforms report unavailable rather than a false zero. Every performance/retrieval output is checked for dimensions, finite norm and unit normalisation.

The full repository `CGO_ENABLED=0 go test ./... -count=1` passes, including service/repository packages. The full native Gemma and harness suites pass with the real checkpoint. Independent reference cases, parallel versus serial, guard/tail cases, in-flight cancellation, Close joining and oversubscribed workers pass. Race-instrumented Gemma/harness suites pass. Vet, Linux ARM64 and Darwin ARM64 harness builds, and pure-Go Gemma CLI build pass. Retrieval rerun with parked workers preserves all ranks; maximum and RMS observed top-score differences are zero. This is a measured numerical comparison, not a hash acceptance gate; the existing independent tolerances remain authoritative and unchanged.

Next validation should use a separately frozen, access-approved real corpus with human labels and query-language distribution matching actual use, plus NAS serial resource measurements and (if relevant) concurrent-request throughput. Do not choose a checkpoint/dimension or deploy from this synthetic result alone. No production inference refresh, image/config change, release or NAS rollout was performed.

Raw profiles and matching binaries were removed immediately after analysis. Disposable result JSON, scripts and logs were removed once final summaries and tests were complete. Retain the harness/corpora, this concise evidence and intentional model asset only.
