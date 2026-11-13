# GTE-small and EmbeddingGemma on the NAS

## Result

Keep GTE on this NAS. Gemma retains the multilingual quality advantage seen on the development host, but takes about 13.6×/5.0×/3.4× as long for the query/paragraph/chunk workloads. Process CPU ratios are similar: the gap is not explained by background scheduling alone. The i7 host's AVX2 results do not transfer to the NAS's Celeron J3455.

No production deployment, configuration, state, model or index was changed. The isolated benchmark container and its owned scratch volume were removed after completion. Production remains healthy on the same image, with zero restarts and no OOM.

## Setup and safety

- Target: Portainer endpoint 18, DiskStation, Intel Celeron J3455 at 1.50 GHz, four logical processors, about 7.62 GiB RAM. Docker 24.0.2, kernel 4.4.302+. CPU flags lack AVX, AVX2 and FMA. Both engines selected **SSE2**.
- Source `31c6819` on `feat/embeddinggemma-native`, following `70708b8`. This adds a harness-only correction to report the actual selected Gemma backend rather than the old hard-coded `auto/avx2` label. Numerical/model code is unchanged by that reporting fix. Static Linux amd64 baseline (`GOAMD64=v1`), Go 1.27.1, CGO disabled.
- Sequential inference, CPU affinity **3**, `GOMAXPROCS=1`, Gemma workers=1, `nice=19`, Docker CPU shares 128, memory and memory+swap both 768 MiB. The NAS was already busy: observed run-start load averages ranged roughly 6.6–12.3 on four CPUs. These are deliberately low-priority, loaded-system measurements, not idle-host latency or a production service-level objective.
- Networkless helper based on already-installed Alpine 3.22, read-only root, all capabilities dropped, no-new-privileges. Final helper had **no production state/config/secret/model mounts**. Its only writable volume mounted at the approved portable project root `/tmp/memento-go`; all payloads/results lived under `runs/nas-compare`. No NAS SSH access or production exec was needed.
- An initial empty helper using tmpfs could not receive Docker archive uploads: Docker's archive path did not expose its tmpfs directory. It was stopped/deleted, then replaced with the dedicated disposable volume. No benchmark results came from that probe. The initially inspected external production model volume was empty; it was not modified or mounted into the final helper.
- Both pinned checkpoints were copied to owned benchmark scratch and checked on the NAS: GTE SHA-256 `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`; Gemma SHA-256 `ea905fd08e8061db77a0031cbb5096d459d7cfd2bdd0ad2ba66e483f16719493`. Hashes establish asset provenance, not output acceptance.
- Frozen expanded corpus: `tools/embedding-compare/corpus-expanded.json`, SHA-256 `e7c56b400ac4374bb9d72e87142786778b84f486a517228af6c074e2b09baa7f`. Same text, labels and prefixes as the [host comparison](gte-gemma-refresh-2026-11-13.md). The corpus contains 60 English documents, 80 queries across 20 topics, including 40 translations in five languages. It remains a synthetic diagnostic, not representative production-quality evidence.

The full sequence ran from 23:01:11 to 23:35:44 UTC on 13 November 2026: GTE → Gemma → Gemma → GTE warm timing, separate GTE/Gemma profiles, then GTE/Gemma retrieval. All completed without timeout or memory-limit failure. Cgroup peak charge was 551,104,512 bytes (525.6 MiB), including cached checkpoint pages and activity from multiple sequential processes; this is not a single process's peak RSS. The 768 MiB limit is not a recommended service budget.

## Warm latency and CPU

Two independently loaded processes per engine, with 17/13/9 timed query/paragraph/chunk repetitions per process (34/26/18 pooled samples). First inference and warm-up are excluded. Wall median is pooled; process CPU is user+system time over the repeat block divided by requests. Token counts are GTE/Gemma 8/15, 69/69 and 262/254; input text is identical, tokenisation is not.

| Workload | GTE wall median | Gemma wall median | Gemma/GTE wall | GTE CPU/request | Gemma CPU/request |
|---|---:|---:|---:|---:|---:|
| Query | 0.156 s | 2.115 s | 13.56× | 0.153 s | 2.074 s |
| Paragraph | 1.534 s | 7.640 s | 4.98× | 1.470 s | 7.465 s |
| Longer chunk | 8.219 s | 27.903 s | 3.40× | 7.672 s | 26.994 s |

CPU ratios are 13.57×, 5.08× and 3.52×. These quantify a genuine compute penalty independently of most scheduler delay, though CPU frequency/cache contention can still affect CPU time. No claim is made that pinning a different core or running at normal priority would reproduce the same absolute wall timings.

| Workload | GTE process medians A / B | Gemma process medians A / B | GTE pooled p95 | Gemma pooled p95 |
|---|---:|---:|---:|---:|
| Query | 0.166 / 0.147 s | 2.129 / 2.086 s | 0.248 s | 2.286 s |
| Paragraph | 1.610 / 1.501 s | 7.658 / 7.591 s | 1.889 s | 8.083 s |
| Longer chunk | 8.051 / 8.472 s | 27.620 / 29.668 s | 9.395 s | 34.597 s |

Both run orders agree on the conclusion despite load variability. P95 for 18 long-chunk samples is the maximum sample, not a well-estimated population tail.

Load times were GTE 0.380/0.477 s and Gemma 1.163/1.071 s. First query inference was GTE 0.152/0.142 s and Gemma 2.165/2.148 s. File integrity hashing occurs before load timing. Repeated launches benefit from filesystem cache; these are not cold-boot disk timings.

There is no meaningful four-worker test of the current Gemma projection implementation on this CPU. `packedProjectionSupported` requires AVX2; without it the request stays on serial projection even if more workers are requested. Raising the scheduler ceiling cannot enable missing instructions. No four-core run was used to imply acceleration or monopolise the production NAS.

## Memory and allocations

| Measurement | GTE | Gemma |
|---|---:|---:|
| Peak process RSS, including loading | 263.5–263.7 MiB | 164.4–164.5 MiB |
| Long-chunk process RSS | 147.3–147.8 MiB | 153.2–158.4 MiB |
| Post-load, post-GC Go heap | ~129.5 MiB | ~52.9 MiB |

Mapped Gemma weights are outside the Go heap. Gemma has a lower loading peak, not a lower steady RSS in these runs. GTE's checkpoint buffer plus decoded weights explain its startup peak; this measurement did not alter the loader.

Per-call allocation bytes/objects match the serial host results:

| Workload | GTE | Gemma |
|---|---:|---:|
| Query | ~1,566 bytes / 3 objects | 21,464 / 217 |
| Paragraph | 1,592 / 7 | 140,312 / 1,466 |
| Longer chunk | 1,640 / 13 | 474,584 / 5,224 |

No OOM, cgroup memory-limit failure or production restart occurred. The helper's limit bounds this experiment, not the production service. Kernel PID-limit support was unavailable on the first helper probe; no claim is made that a PID cgroup limit was enforced. The final helper had a fixed small process tree and no exposed network.

## Retrieval on the NAS

The complete 140-text embedding pass (60 documents plus 80 queries) took:

| Engine | Wall time | Process CPU | CPU/GTE |
|---|---:|---:|---:|
| GTE serial | 120.45 s | 111.32 s | 1.00× |
| Gemma serial | 611.82 s | 569.37 s | 5.11× |

Wall ratio is 5.08×. This is sequential corpus embedding, not end-to-end search latency or concurrent throughput.

Aggregate quality matches the host diagnostic:

| Engine/dimensions | Top-1 | Recall@3 | Recall@10 | MRR |
|---|---:|---:|---:|---:|
| GTE / 384 | 51/80 | 73.75% | 83.75% | .70539 |
| Gemma / 768 | 71/80 | 100% | 100% | .94167 |
| Gemma / 512 | 73/80 | 100% | 100% | .95625 |
| Gemma / 384 | 70/80 | 100% | 100% | .93542 |
| Gemma / 256 | 68/80 | 100% | 100% | .92083 |
| Gemma / 128 | 63/80 | 95% | 100% | .87408 |

GTE versus Gemma/512 English top-1 is 37/40 versus 37/40; non-English is 14/40 versus 36/40. The same seven Gemma/512 queries as on the host rank their answer second (`oauth-b-fr`, `oauth-a-en`, `replication-b-es`, `http-auth-b-pt`, `network-b-pt`, `network-b-en`, `search-b-en`). Labels/texts were not adjusted after results. The exploratory 512-dimensional advantage over full-size Gemma still needs unseen-data validation.

All benchmark embeddings pass finite, dimensional and unit-norm checks. Separately, all 11 independent Gemma reference cases passed locally with AVX/AVX2/FMA disabled, including the 1,028-token local-attention case. Observed reference RMS errors were about 7.9e-9–7.8e-8, max absolute error at most 2.75e-7, cosine printed 1.0000000000; original numerical tolerances were unchanged. This fallback correctness run is explicitly a local reference check, not an independent NAS-reference execution. The NAS's retrieval ranks and norm checks provide additional target evidence, not byte-exact signal acceptance.

## Profile analysis

Separate diagnostic profiles use the same three input texts with three timed repetitions each for both engines. CPU capture starts after load/first inference; allocation history includes model/tokenizer construction. Profiled elapsed times are excluded from primary timing tables. Inspected CPU flat/cumulative, `alloc_space` and `alloc_objects`; captures were nonempty.

- **GTE:** 45.22 s profiled elapsed, 37.73 s CPU samples. `dotRowsSSE2` is 61.09% flat; `forwardInto` is 24.89% flat and 99.84% cumulative. Approximately 259.62 MB sampled allocation space, dominated by decoded weights (128.60 MB) and file contents (128.04 MB). Model vocabulary construction accounts for 32,768 of 32,861 sampled allocated objects. These are mostly startup costs; warm allocations remain small.
- **Gemma:** 161.16 s elapsed, 148.26 s CPU samples. `dotRowsSSE2` is 73.46% flat; `decodeRow` is 14.85% flat. Projection is about 87.9% cumulative. There is no polling-team scheduler outlier because AVX2 packed projections/teams are unavailable. Around 75.32 MB sampled allocation space; tokenizer construction 25.01 MB flat and parser values 21.86 MB flat, with parser strings/values dominating ~613k sampled allocated objects. Sampled allocations are not exact totals and should not be compared as byte-for-byte regression gates.

The major target cost is repeated SSE2 projection/decode, not an orchestration defect repaired by the parked-worker change. This task measures the existing implementation; no performance fix or pre-release optimisation completion is claimed. A separate bounded SSE2 blocked/batched-kernel project could investigate avoiding repeated decode/row-dot work while preserving reference tolerances, ownership and cancellation. Its benefit is unmeasured and must not be assumed. The actionable deployment conclusion does not depend on speculative future optimisation: the current implementation is too costly to replace GTE here.

## Production checks and disposal

Before and after: same production container `71a9a9af…`, image `sha256:a42e4e4d62c0f385198aaf918f1488067873fa419af1914d521baa464d8044e3`, started 10:04:28 UTC, running/healthy, restart count 0, OOM false. Authenticated read-only MCP status during the run reports service 1.0.12, semantic ready, unchanged repository/index/embedding revision `c9c3bb7eca06868b23aeae1b356c0c9f53349bd1`, index not stale and proposal backlog 0. The scoped shared-reader principal sees 111 concepts; visibility is principal-specific, not evidence of deletion. Missing-auth MCP ping remains rejected with 401.

After jobs completed, removed container `00762ece…` and owned Docker volume `memento-go-benchmark-20261113`. Raw profiles were deleted locally and remotely immediately after analysis. Transferred checkpoint copies, payload archives, result JSON, temporary scripts/logs and matching binaries were removed after the final summary. Durable local model assets and source remain. No image was built or pulled on the NAS, and no production file/mount was written.

**Recommendation:** retain NAS GTE. If multilingual retrieval warrants Gemma, evaluate an AVX2-capable host or a separately measured NAS SSE2 optimisation before any deployment decision. Do not mix vectors from different models or silently change the live index.
