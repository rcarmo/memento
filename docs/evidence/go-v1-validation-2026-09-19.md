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

Every package containing production Go code must expose at least one useful fuzz target. The root module has 40 targets and uMCP has 14, including its example command. All 54 passed 10,000 deterministic executions per target. The race detector passes both modules, and govulncheck reports zero reachable vulnerabilities.

## Models and allocation budgets

The pinned GTE and Needle assets pass their real-model loaders, tokenizers and generated-output checks. Needle validates all 31 tensors, all 8,192 tokenizer pieces and token decodes, five generation cases and the 360-case held-out corpus on native amd64 and ARM64.

The enforced allocation ceilings are:

| Benchmark | Allocations/op | Bytes/op |
| --- | ---: | ---: |
| semantic cosine over 384 values | 0 | 0 |
| Needle vocabulary lookup | 0 | 0 |
| selected SIMD dot product | 0 | 0 |
| real GTE embedding | 2 | 2,400 |
| lexical search over 100 concepts | 875 | 43,500 |
| real Needle generation | 900 | 1,900,000 |

Local real-Needle runs after constraint-template caching and request-local workspace reuse measured 758--759 allocations/op, about 1.78 MB/op and 78--81 ms/op. These wall-clock figures describe the local Intel i7-12700 host and are not cross-runner release thresholds.

## Packaging and rollback

The release check builds reproducible, stripped, static linux/amd64-v1 and linux/arm64 archives containing `memento-go`, `memento-embed-go` and `memento-skill-import-go`. Archive layout, SHA-256 manifests, ELF architecture, absent dynamic dependencies and native version output pass.

The replacement image is distroless, runs as UID/GID 65532 and contains no shell, Python, Rust library, CGo dependency, native SQLite extension or Git executable. Repeated container-contract runs passed under the read-only 512 MiB profile with approximately 171.4--171.6 MiB idle RSS.

A disposable volume was created with the pinned `0.5.9` image, opened by `v1.0.0`, then reopened by `0.5.9` with repository and index revisions intact. The final `service_version: 0.5.9` line in that test belongs to the deliberate rollback leg, not the Go image.

## Deployment boundary

This report validates release inputs and rollback compatibility. The latest recorded live DiskStation deployment remains `0.5.9`; replacing it with `v1.0.0` requires an operator-controlled backup, image pull, target smoke test and rollback decision.
