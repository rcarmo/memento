# Memento Rust semantic search workspace

This workspace holds the Rust runtime used by Memento's semantic search path. It ports the FP32 `GTE1` loader, tokenizer and inference flow from `/tmp/go-gte`, keeps the vector maths in one place, and exposes the same runtime through a framed process protocol, a stable C ABI and a loadable SQLite extension.

## Workspace crates

* `memento-vector` is the shared low-level layer. It validates packed `f32le` vectors and implements dot-product and cosine kernels with scalar code plus runtime-dispatched SIMD on `x86_64` and `aarch64` where the host supports it.
* `memento-gte` is the model runtime. It parses `GTE1`, preserves tokenizer behaviour against the Go reference, runs scalar inference and supports bounded batch embedding with cancellation checkpoints.
* `memento-embed` is the framed embedding protocol library and binary. The `memento-embed` executable exposes `info`, `embed` and `embed_batch` for the Python side when a process boundary is preferable to direct FFI.
* `memento-ffi` builds as `cdylib` and `rlib`. It exposes the stable C ABI used for model loading, embedding, cancellation, error retrieval and vector helpers, and is the surface consumed from Python via `ctypes`.
* `memento-sqlite-vector` also builds as `cdylib` and `rlib`. It is a loadable SQLite extension that exposes `vector_cosine`, `vector_dimensions` and `vector_is_valid` over packed float32 blobs.

## Builds and tests

Use the workspace root for formatting, linting and tests.

```bash
cd rust
cargo fmt --all
cargo clippy --workspace --all-targets
cargo test --workspace
```

These commands cover every crate in the workspace. They do not fetch or generate model fixtures by themselves -- the golden artefacts are managed separately.

## Model fixtures

If `/tmp/go-gte/models/gte-small/model.safetensors` is present, run:

```bash
cd rust
./tests/scripts/generate_golden.sh
```

This generates:

* `tests/fixtures/gte-small.gtemodel`
* `tests/fixtures/go_parity.json`

`tests/fixtures/gte-small.gtemodel` is the converted `GTE1` model used by Rust-side runtime tests. `tests/fixtures/go_parity.json` records token and embedding parity data generated through the upstream Go implementation. The generator preserves MIT attribution to the upstream Go implementation.

## FFI and SQLite extension outputs

* `memento-ffi` produces the shared-library artefact used by the Python runtime when it loads the Rust embedding engine through `ctypes`.
* `memento-sqlite-vector` produces the loadable SQLite extension used for `vector_cosine` evaluation inside SQLite over packed float32 fields.
* Both crates also build as `rlib`, which keeps them usable from Rust tests and other internal Rust callers without introducing a second implementation path.

## Golden generation

The parity inputs, outputs and script behaviour are documented in [docs/golden-generation.md](docs/golden-generation.md). Use that document when fixtures need to be regenerated or audited.
