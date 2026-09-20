# Pure-Go v1.0.0 validation

This report records the final pre-release validation of the pure-Go Memento replacement on branch `go`. It covers commit `0e1f528eac1403883c56864cfa40f9f8c8de7b08`; no production deployment was performed.

## CI result

GitHub Actions run [35476773451](https://github.com/rcarmo/memento/actions/runs/35476773451) completed successfully:

* the pure-Go quality and allocation job passed;
* the pinned runtime-model bundle was restored and verified;
* native amd64 model and replacement-container checks passed;
* native ARM64 model, allocation and 360-case Needle corpus checks passed.

The previous ARM64 GTE byte-accounting failure was resolved by retaining the strict two-allocation ceiling while setting the cross-architecture byte ceiling to 2,400 bytes per operation. No ARM-specific inference path was added.

## Quality and fuzzing

`make quality` reports zero uncovered production statements in the root module and 100% statement coverage in the standalone uMCP module. Vet, pinned Staticcheck and static builds pass.

Every package containing production Go code must expose at least one useful fuzz target. The root module has 45 targets and uMCP has 14, including its example command. All 59 passed 10,000 deterministic executions per target. The race detector passes both modules, and govulncheck reports zero reachable vulnerabilities.

## Models and allocation budgets

The pinned GTE and Needle assets pass their real-model loaders, tokenizers and generated-output checks. Needle validates all 31 tensors, all 8,192 tokenizer pieces and token decodes, five generation cases and the 360-case held-out corpus on native amd64 and ARM64.

The enforced allocation ceilings are:

| Benchmark | Allocations/op | Bytes/op |
| --- | ---: | ---: |
| semantic cosine over 384 values | 0 | 0 |
| Needle vocabulary lookup | 0 | 0 |
| selected SIMD dot product | 0 | 0 |
| selected SIMD row-dot projection | 0 | 0 |
| selected SIMD row-AXPY projection | 0 | 0 |
| real GTE embedding | 2 | 2,400 |
| lexical search over 100 concepts | 875 | 43,500 |
| real Needle generation | 400 | 1,800,000 |

A later profile-led pass added fused row kernels, a register-tiled AVX2 projection path, request-local rune buffers for constrained decoding and a typed SentencePiece BPE heap. Real Needle generation fell from 758--759 to 307--388 allocations/op across repeated benchmark runs; native ARM64 measured 388 allocations/op, so the portable gate is 400. Allocation profiling on amd64 measured 183 allocations/op. Local latency was typically 59--74 ms/op. GTE retained 2 allocations/op, and its 384-row fused dot microkernel measured roughly 5--7 microseconds on the Intel i7-12700. The exact 360-case Needle corpus completed in 387.4 seconds on that host, down from the retained 1,408.8-second scalar baseline. These wall-clock figures describe that host and are not cross-runner release thresholds.

## Packaging and rollback

The release check builds reproducible, stripped, static linux/amd64-v1 and linux/arm64 archives containing `memento-go`, `memento-embed-go`, `memento-needle-go`, `memento-needle-model-go` and `memento-skill-import-go`. Archive layout, SHA-256 manifests, ELF architecture, absent dynamic dependencies and native version output pass. The OCI build generates the architecture-independent FP32 sidecar once from the pinned NDL model rather than duplicating it in each binary archive.

The replacement image is distroless, runs as UID/GID 65532 and contains no shell, Python, Rust library, CGo dependency, native SQLite extension or Git executable. The release build deterministically expands the 51 MB NDL1 Needle model into a 105,265,860-byte, architecture-independent, little-endian FP32 sidecar. Each route runs in a short-lived static Go worker that maps the sidecar read-only and exits after its framed response. On the Intel i7-12700, mapped startup took about 5--6 ms and complete one-shot routes took 79--93 ms, compared with 131--134 ms just to read and expand NDL1 and 190--204 ms for the former one-shot path.

The updated container contract separately measures Docker's cgroup working set and the daemon process RSS after real Needle and GTE subprocess requests. Repeated rebuilt-image runs measured 38.4--39.1 MiB cgroup usage and 52.3--52.9 MiB daemon RSS, with neither worker left resident. Earlier 171.4--171.7 MiB `docker stats` observations were cgroup working-set figures after model activity, not daemon RSS, and included reclaimable file cache.

A disposable volume was created with the pinned `0.5.9` image, opened by `v1.0.0`, then reopened by `0.5.9` with repository and index revisions intact. The final `service_version: 0.5.9` line in that test belongs to the deliberate rollback leg, not the Go image.

## Deployment boundary

This report validates release inputs and rollback compatibility. The latest recorded live DiskStation deployment remains `0.5.9`; replacing it with `v1.0.0` requires an operator-controlled backup, image pull, target smoke test and rollback decision.
