# Memento Rust semantic search workspace

This workspace contains a faithful Rust port of the FP32 `GTE1` model loader/tokenizer/inference path from `/tmp/go-gte`, plus shared vector kernels, a framed embedding protocol, a stable C FFI, and a loadable SQLite vector extension.

## Crates

- `memento-vector`: shared `f32le` validation, dot and cosine kernels with scalar fallback and safe runtime-dispatched SIMD on `x86_64` and `aarch64` where available.
- `memento-gte`: `GTE1` parser, exact tokenizer behaviour matching the Go implementation, scalar inference, and bounded batch embedding with cancellation checkpoints.
- `memento-embed`: persistent framed protocol library and `memento-embed` binary exposing `info`, `embed`, and `embed_batch`.
- `memento-ffi`: `cdylib`/`rlib` crate exposing a stable C ABI for model loading, embedding, cancellation, error retrieval, and vector helpers.
- `memento-sqlite-vector`: loadable SQLite extension exposing `vector_cosine`, `vector_dimensions`, and `vector_is_valid`.

## Commands

```bash
cd rust
cargo fmt --all
cargo clippy --workspace --all-targets
cargo test --workspace
```

## Fixture generation

If `/tmp/go-gte/models/gte-small/model.safetensors` is present, run:

```bash
cd rust
./tests/scripts/generate_golden.sh
```

This generates:

- `tests/fixtures/gte-small.gtemodel`
- `tests/fixtures/go_parity.json`

The generator preserves MIT attribution to the upstream Go implementation.
